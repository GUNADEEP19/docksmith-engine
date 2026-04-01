package builder

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"docksmith-engine/internal"
	"docksmith-engine/internal/image"
)

// Builder implements internal.Builder using layer deltas and image manifest digests.
type Builder struct {
	layer internal.Layer
}

// New returns a Builder that uses the given layer implementation.
func New(l internal.Layer) *Builder {
	return &Builder{layer: l}
}

// Build executes instructions; only COPY and RUN create layers. FROM resets the layer stack.
func (b *Builder) Build(instructions []internal.Instruction, tag string, context string, noCache bool) (internal.Image, error) {
	_ = noCache
	if len(instructions) == 0 {
		return internal.Image{}, errors.New("no instructions provided")
	}
	parts := strings.SplitN(tag, ":", 2)
	if len(parts) != 2 {
		return internal.Image{}, fmt.Errorf("invalid tag %q", tag)
	}
	name := strings.TrimSpace(parts[0])
	imgTag := strings.TrimSpace(parts[1])
	if name == "" || imgTag == "" {
		return internal.Image{}, fmt.Errorf("invalid tag %q", tag)
	}

	ctxAbs, err := filepath.Abs(context)
	if err != nil {
		return internal.Image{}, fmt.Errorf("build: context: %w", err)
	}

	var curDigest string
	var layers []internal.ImageLayer
	wd := "/"
	env := map[string]string{}
	var cmd []string

	for _, inst := range instructions {
		op := strings.ToUpper(strings.TrimSpace(inst.Op))
		switch op {
		case "FROM":
			curDigest = ""
			wd = "/"
		case "WORKDIR":
			if len(inst.Args) != 1 {
				return internal.Image{}, fmt.Errorf("WORKDIR requires one argument")
			}
			wd = resolveContainerPath(wd, inst.Args[0])
		case "ENV":
			if len(inst.Args) != 1 {
				return internal.Image{}, fmt.Errorf("ENV requires KEY=value")
			}
			k, v, ok := strings.Cut(inst.Args[0], "=")
			if !ok || strings.TrimSpace(k) == "" {
				return internal.Image{}, fmt.Errorf("ENV requires KEY=value")
			}
			env[strings.TrimSpace(k)] = v
		case "CMD":
			if len(inst.Args) == 0 {
				return internal.Image{}, fmt.Errorf("CMD requires a command")
			}
			cmd = append([]string(nil), inst.Args...)
		case "COPY", "RUN":
			runInst := inst
			if op == "COPY" {
				runInst = rewriteCopyInstruction(wd, inst)
			}
			d, sz, err := b.layer.CreateLayer(curDigest, runInst, ctxAbs)
			if err != nil {
				return internal.Image{}, err
			}
			layers = append(layers, internal.ImageLayer{
				Digest:    d,
				Size:      sz,
				CreatedBy: inst.String(),
			})
			curDigest = d
		default:
			return internal.Image{}, fmt.Errorf("build: unsupported instruction %q", op)
		}
	}

	img := internal.Image{
		Name:      name,
		Tag:       imgTag,
		// Deterministic builds: do not inject wall-clock time into the manifest digest.
		CreatedAt: time.Time{},
		Config: internal.ImageConfig{
			Cmd:        cmd,
			Env:        env,
			WorkingDir: wd,
		},
		Layers: layers,
	}
	digest, err := image.ImageManifestDigest(img)
	if err != nil {
		return internal.Image{}, err
	}
	img.Digest = digest
	return img, nil
}

func resolveContainerPath(workdir, p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return workdir
	}
	if path.IsAbs(p) {
		return path.Clean(p)
	}
	return path.Clean(path.Join(workdir, p))
}

func rewriteCopyInstruction(workdir string, inst internal.Instruction) internal.Instruction {
	if len(inst.Args) < 2 {
		return inst
	}
	out := internal.Instruction{
		Raw:  inst.Raw,
		Op:   inst.Op,
		Args: append([]string(nil), inst.Args...),
	}
	last := len(out.Args) - 1
	out.Args[last] = resolveContainerPath(workdir, out.Args[last])
	return out
}
