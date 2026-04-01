package builder

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"docksmith-engine/internal"
	"docksmith-engine/internal/cache"
	"docksmith-engine/internal/image"
)

// Builder implements internal.Builder using layer deltas and image manifest digests.
type Builder struct {
	layer         internal.Layer
	cache         internal.Cache
	docksmithRoot string
}

// Option configures Builder construction.
type Option func(*Builder)

// WithDataRoot sets the Docksmith data directory (default: ~/.docksmith). Used for cache storage and layer tar size on cache hits.
func WithDataRoot(root string) Option {
	return func(b *Builder) {
		if strings.TrimSpace(root) != "" {
			b.docksmithRoot = root
		}
	}
}

// WithCache sets the layer cache implementation (defaults to ~/.docksmith/cache).
func WithCache(c internal.Cache) Option {
	return func(b *Builder) {
		b.cache = c
	}
}

// New returns a Builder that uses the given layer implementation.
func New(l internal.Layer, opts ...Option) *Builder {
	b := &Builder{layer: l}
	for _, o := range opts {
		o(b)
	}
	if b.docksmithRoot == "" {
		b.docksmithRoot = defaultDocksmithRoot()
	}
	if b.cache == nil {
		b.cache = cache.New(b.docksmithRoot)
	}
	return b
}

func defaultDocksmithRoot() string {
	h, err := os.UserHomeDir()
	if err != nil {
		h = "."
	}
	return filepath.Join(h, ".docksmith")
}

func layerTarSize(docksmithRoot, digest string) (int64, error) {
	hex := strings.TrimPrefix(strings.TrimSpace(digest), "sha256:")
	if len(hex) != 64 {
		return 0, fmt.Errorf("invalid digest")
	}
	p := filepath.Join(docksmithRoot, "layers", fmt.Sprintf("sha256_%s.tar", hex))
	st, err := os.Stat(p)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}

// Build executes instructions; only COPY and RUN create layers. FROM resets the layer stack.
func (b *Builder) Build(instructions []internal.Instruction, tag string, context string, noCache bool) (internal.Image, error) {
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
	var cascade bool

	for _, inst := range instructions {
		op := strings.ToUpper(strings.TrimSpace(inst.Op))
		switch op {
		case "FROM":
			curDigest = ""
			wd = "/"
			cascade = false
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
			key, err := cache.KeyHash(curDigest, inst.Raw, wd, env, op, inst.Args, ctxAbs)
			if err != nil {
				return internal.Image{}, fmt.Errorf("build: cache key: %w", err)
			}

			var digest string
			var sz int64

			if noCache {
				digest, sz, err = b.layer.CreateLayer(curDigest, runInst, ctxAbs)
				if err != nil {
					return internal.Image{}, err
				}
			} else {
				hit := false
				if !cascade {
					if d, ok := b.cache.Check(key); ok {
						if stSize, err := layerTarSize(b.docksmithRoot, d); err == nil {
							digest = d
							sz = stSize
							hit = true
						}
					}
				}
				if !hit {
					cascade = true
					digest, sz, err = b.layer.CreateLayer(curDigest, runInst, ctxAbs)
					if err != nil {
						return internal.Image{}, err
					}
					if err := b.cache.Store(key, digest); err != nil {
						return internal.Image{}, fmt.Errorf("build: cache store: %w", err)
					}
				}
			}

			layers = append(layers, internal.ImageLayer{
				Digest:    digest,
				Size:      sz,
				CreatedBy: inst.String(),
			})
			curDigest = digest
		default:
			return internal.Image{}, fmt.Errorf("build: unsupported instruction %q", op)
		}
	}

	img := internal.Image{
		Name:      name,
		Tag:       imgTag,
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
