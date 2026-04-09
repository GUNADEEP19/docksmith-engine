package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"docksmith-engine/internal/builder"
	"docksmith-engine/internal/cache"
	"docksmith-engine/internal/image"
	"docksmith-engine/internal/layer"
	"docksmith-engine/internal/parser"
	"docksmith-engine/internal/runtime"
)

type envFlag map[string]string

func (e envFlag) String() string {
	if len(e) == 0 {
		return ""
	}
	parts := make([]string, 0, len(e))
	for k, v := range e {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ",")
}

func (e envFlag) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("-e expects KEY=VALUE")
	}
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
		return errors.New("-e expects KEY=VALUE")
	}
	e[strings.TrimSpace(parts[0])] = parts[1]
	return nil
}

func main() {
	lyr := layer.New()
	imgStore := image.NewStore()
	setModules(modules{
		Parser:  parser.New(),
		Builder: builder.New(lyr, builder.WithImageStore(imgStore)),
		Cache:   cache.New(""),
		Layer:   lyr,
		Image:   imgStore,
		Runtime: runtime.New(),
	})
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usageError("no command provided")
	}

	switch args[0] {
	case "build":
		fs := flag.NewFlagSet("build", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		tag := fs.String("t", "", "image tag (name:tag)")
		noCache := fs.Bool("no-cache", false, "disable cache")
		if err := fs.Parse(args[1:]); err != nil {
			return usageError(fmt.Sprintf("invalid build arguments: %v", err))
		}

		context := "."
		if rest := fs.Args(); len(rest) > 0 {
			context = rest[0]
		}
		if len(fs.Args()) > 1 {
			return usageError("build accepts at most one context path")
		}
		return buildCommand(*tag, context, *noCache)

	case "run":
		fs := flag.NewFlagSet("run", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		envs := envFlag{}
		fs.Var(envs, "e", "environment variable override (KEY=VALUE)")
		if err := fs.Parse(args[1:]); err != nil {
			return usageError(fmt.Sprintf("invalid run arguments: %v", err))
		}

		rest := fs.Args()
		if len(rest) == 0 {
			return usageError("run requires image name:tag")
		}
		image := rest[0]
		overrideCmd := []string{}
		if len(rest) > 1 {
			overrideCmd = rest[1:]
		}
		return runCommand(image, overrideCmd, envs)

	case "images":
		if len(args) > 1 {
			return usageError("images does not accept extra arguments")
		}
		return imagesCommand()

	case "rmi":
		if len(args) != 2 {
			return usageError("rmi requires image name:tag")
		}
		return rmiCommand(args[1])

	case "base":
		if len(args) != 2 {
			return usageError("base requires image name:tag")
		}
		return baseCommand(args[1])

	default:
		return usageError(fmt.Sprintf("unknown command %q", args[0]))
	}
}

func usageError(msg string) error {
	usage := "usage:\n" +
		"  docksmith build -t name:tag [--no-cache] [context]\n" +
		"  docksmith run [-e KEY=VALUE]... name:tag [cmd ...]\n" +
		"  docksmith images\n" +
		"  docksmith rmi name:tag\n" +
		"  docksmith base name:tag"
	return fmt.Errorf("%s\n%s", msg, usage)
}
