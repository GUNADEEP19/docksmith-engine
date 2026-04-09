package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"docksmith-engine/internal"
	"docksmith-engine/internal/builder"
)

type modules struct {
	Parser  internal.Parser
	Builder internal.Builder
	Cache   internal.Cache
	Layer   internal.Layer
	Image   internal.ImageStore
	Runtime internal.Runtime
}

var deps modules

func setModules(m modules) {
	deps = m
}

func buildCommand(tag string, context string, noCache bool) error {
	if strings.TrimSpace(tag) == "" {
		return errors.New("build failed: image tag is required (-t name:tag)")
	}
	if err := validateNameTag(tag); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	if strings.TrimSpace(context) == "" {
		return errors.New("build failed: build context path is required")
	}
	info, err := os.Stat(context)
	if err != nil {
		return fmt.Errorf("build failed: context path %q: %w", context, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("build failed: context path %q is not a directory", context)
	}
	if deps.Parser == nil || deps.Builder == nil {
		return errors.New("build failed: parser/builder modules are not configured")
	}

	start := time.Now()
	docksmithfilePath := filepath.Join(context, "Docksmithfile")
	instructions, err := deps.Parser.Parse(docksmithfilePath)
	if err != nil {
		// Surface parser errors verbatim so line numbers/messages match expectations.
		return err
	}

	// Prefer real per-step cache HIT/MISS reporting if the builder supports it.
	var img internal.Image
	if b, ok := deps.Builder.(*builder.Builder); ok {
		img, err = b.BuildWithProgress(instructions, tag, context, noCache, func(step internal.BuildStep) {
			instText := step.Instruction
			if strings.TrimSpace(instText) == "" {
				instText = "<empty instruction>"
			}
			logStep(step.Index, step.Total, instText, step.CacheStatus, step.DurationSeconds)
		})
	} else {
		// Fallback: log steps without real cache info.
		for i, inst := range instructions {
			instText := inst.String()
			if strings.TrimSpace(instText) == "" {
				instText = "<empty instruction>"
			}
			logStep(i+1, len(instructions), instText, "", 0)
		}
		img, err = deps.Builder.Build(instructions, tag, context, noCache)
	}
	if err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	if deps.Image != nil {
		if err := deps.Image.Save(img); err != nil {
			return fmt.Errorf("build failed: save image metadata: %w", err)
		}
	}

	digest := img.Digest
	if strings.TrimSpace(digest) == "" {
		digest = "sha256:<unknown>"
	}
	logSuccess(digest, tag, time.Since(start).Seconds())
	return nil
}

func runCommand(image string, overrideCmd []string, envVars map[string]string) error {
	if strings.TrimSpace(image) == "" {
		return errors.New("run failed: image name:tag is required")
	}
	if err := validateNameTag(image); err != nil {
		return fmt.Errorf("run failed: %w", err)
	}
	if deps.Image == nil || deps.Runtime == nil {
		return errors.New("run failed: image/runtime modules are not configured")
	}

	img, err := deps.Image.Load(image)
	if err != nil {
		return fmt.Errorf("run failed: load image %q: %w", image, err)
	}

	effectiveCmd := overrideCmd
	if len(effectiveCmd) == 0 {
		effectiveCmd = img.Config.Cmd
	}
	if len(effectiveCmd) == 0 {
		return errors.New("run failed: no CMD specified")
	}

	mergedEnv := map[string]string{}
	for k, v := range img.Config.Env {
		mergedEnv[k] = v
	}
	for k, v := range envVars {
		mergedEnv[k] = v
	}

	fmt.Printf("[run] image=%s cmd=%s\n", image, strings.Join(effectiveCmd, " "))
	exitCode, err := deps.Runtime.Run(img, effectiveCmd, mergedEnv)
	if err != nil {
		return fmt.Errorf("run failed: %w", err)
	}

	fmt.Printf("Exit code: %d\n", exitCode)
	if exitCode != 0 {
		return fmt.Errorf("run failed: process exited with code %d", exitCode)
	}
	return nil
}

func imagesCommand() error {
	if deps.Image == nil {
		return errors.New("images failed: image module is not configured")
	}

	images, err := deps.Image.List()
	if err != nil {
		return fmt.Errorf("images failed: %w", err)
	}

	if len(images) == 0 {
		fmt.Fprintln(os.Stdout, "No local images found")
		return nil
	}

	fmt.Fprintln(os.Stdout, "NAME\tTAG\tID")
	for _, img := range images {
		tag := strings.TrimSpace(img.Tag)
		if tag == "" {
			tag = "latest"
		}
		name := strings.TrimSpace(img.Name)
		if name == "" {
			name = "<unnamed>"
		}
		digest := strings.TrimSpace(img.Digest)
		if digest == "" {
			digest = "sha256:<unknown>"
		}
		fmt.Fprintf(os.Stdout, "%s\t%s\t%s\n", name, tag, digest)
	}
	return nil
}

func rmiCommand(image string) error {
	if strings.TrimSpace(image) == "" {
		return errors.New("rmi failed: image name:tag is required")
	}
	if err := validateNameTag(image); err != nil {
		return fmt.Errorf("rmi failed: %w", err)
	}
	if deps.Image == nil {
		return errors.New("rmi failed: image module is not configured")
	}

	if err := deps.Image.Remove(image); err != nil {
		return fmt.Errorf("rmi failed: remove %q: %w", image, err)
	}

	fmt.Printf("Removed image %s\n", image)
	return nil
}

func baseCommand(image string) error {
	if strings.TrimSpace(image) == "" {
		return errors.New("base failed: image name:tag is required")
	}
	if err := validateNameTag(image); err != nil {
		return fmt.Errorf("base failed: %w", err)
	}
	if deps.Image == nil {
		return errors.New("base failed: image store module is not configured")
	}
	parts := strings.SplitN(image, ":", 2)
	img := internal.Image{
		Name: strings.TrimSpace(parts[0]),
		Tag:  strings.TrimSpace(parts[1]),
		Config: internal.ImageConfig{
			Cmd:        []string{},
			Env:        map[string]string{},
			WorkingDir: "",
		},
		Layers: []internal.ImageLayer{},
	}
	if err := deps.Image.Save(img); err != nil {
		return fmt.Errorf("base failed: save base image: %w", err)
	}
	fmt.Printf("Imported base image %s\n", image)
	return nil
}

func validateNameTag(nameTag string) error {
	parts := strings.Split(nameTag, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid image reference %q: expected name:tag", nameTag)
	}
	name := strings.TrimSpace(parts[0])
	tag := strings.TrimSpace(parts[1])
	if name == "" || tag == "" {
		return fmt.Errorf("invalid image reference %q: expected non-empty name and tag", nameTag)
	}
	if strings.ContainsAny(name, " \t\n") || strings.ContainsAny(tag, " \t\n") {
		return fmt.Errorf("invalid image reference %q: name/tag must not contain spaces", nameTag)
	}
	return nil
}
