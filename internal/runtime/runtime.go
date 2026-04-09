package runtime

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"docksmith-engine/internal"
	"docksmith-engine/internal/isolation"
	"docksmith-engine/internal/provision"
)

// Runtime runs a built image by extracting its layers into a temporary rootfs
// and executing the configured command inside an isolated filesystem.
//
// On non-Linux platforms, isolation will return an error.
// On Linux, this uses chroot-based isolation.
type Runtime struct {
	docksmithRoot string
}

// Option configures Runtime.
type Option func(*Runtime)

// WithDataRoot sets Docksmith data directory (default: ~/.docksmith).
func WithDataRoot(root string) Option {
	return func(r *Runtime) {
		if strings.TrimSpace(root) != "" {
			r.docksmithRoot = root
		}
	}
}

func defaultDocksmithRoot() string {
	h, err := os.UserHomeDir()
	if err != nil {
		h = "."
	}
	return filepath.Join(h, ".docksmith")
}

// New creates a Runtime.
func New(opts ...Option) *Runtime {
	r := &Runtime{docksmithRoot: defaultDocksmithRoot()}
	for _, o := range opts {
		o(r)
	}
	if strings.TrimSpace(r.docksmithRoot) == "" {
		r.docksmithRoot = defaultDocksmithRoot()
	}
	return r
}

// Run implements internal.Runtime.
func (r *Runtime) Run(image internal.Image, cmd []string, env map[string]string) (int, error) {
	if len(cmd) == 0 {
		return 1, errors.New("runtime: no command specified")
	}
	rootfs, err := os.MkdirTemp("", "docksmith-rootfs-*")
	if err != nil {
		return 1, fmt.Errorf("runtime: temp dir: %w", err)
	}
	defer os.RemoveAll(rootfs)

	// Extract layers in order (bottom -> top).
	for _, l := range image.Layers {
		if err := r.extractLayerTar(rootfs, l.Digest); err != nil {
			return 1, err
		}
	}

	// Provision the rootfs with essential host binaries so that commands
	// like "sh -c ..." work inside the chroot.
	if err := provision.ProvisionBaseBinaries(rootfs); err != nil {
		return 1, fmt.Errorf("runtime: provision binaries: %w", err)
	}

	workdir := strings.TrimSpace(image.Config.WorkingDir)
	if workdir == "" {
		workdir = "/"
	}
	// Ensure working directory exists inside rootfs.
	wdAbs := filepath.Join(rootfs, filepath.FromSlash(strings.TrimPrefix(filepath.Clean("/"+workdir), "/")))
	if err := os.MkdirAll(wdAbs, 0o755); err != nil {
		return 1, fmt.Errorf("runtime: create workdir: %w", err)
	}

	exitCode, err := isolation.Exec(rootfs, workdir, cmd, env)
	if err != nil {
		return exitCode, err
	}
	return exitCode, nil
}

func (r *Runtime) extractLayerTar(rootfs string, digest string) error {
	hex := strings.TrimPrefix(strings.TrimSpace(digest), "sha256:")
	if len(hex) != 64 {
		return fmt.Errorf("runtime: invalid layer digest %q", digest)
	}
	p := filepath.Join(r.docksmithRoot, "layers", fmt.Sprintf("sha256_%s.tar", hex))
	f, err := os.Open(p)
	if err != nil {
		return fmt.Errorf("runtime: open layer tar: %w", err)
	}
	defer f.Close()

	tr := tar.NewReader(f)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("runtime: read layer tar: %w", err)
		}
		if err := untarEntry(rootfs, tr, h); err != nil {
			return err
		}
	}
	return nil
}

func untarEntry(rootfs string, tr *tar.Reader, h *tar.Header) error {
	name := strings.TrimSpace(h.Name)
	if name == "" {
		return nil
	}
	// Layer tars should always use relative paths.
	name = filepath.Clean(name)
	name = strings.TrimPrefix(name, string(filepath.Separator))
	if name == "." || name == "" {
		return nil
	}
	// Prevent path traversal.
	if strings.HasPrefix(name, ".."+string(filepath.Separator)) || name == ".." {
		return fmt.Errorf("runtime: invalid tar path %q", h.Name)
	}

	dst := filepath.Join(rootfs, name)
	switch h.Typeflag {
	case tar.TypeDir:
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return fmt.Errorf("runtime: mkdir: %w", err)
		}
		return nil
	case tar.TypeReg, tar.TypeRegA:
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("runtime: mkdir parent: %w", err)
		}
		mode := os.FileMode(h.Mode) & os.ModePerm
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			return fmt.Errorf("runtime: create file: %w", err)
		}
		if _, err := io.Copy(out, tr); err != nil {
			_ = out.Close()
			return fmt.Errorf("runtime: write file: %w", err)
		}
		if err := out.Close(); err != nil {
			return fmt.Errorf("runtime: close file: %w", err)
		}
		return nil
	case tar.TypeSymlink:
		// Best-effort symlink support. Reject absolute link targets.
		if filepath.IsAbs(h.Linkname) {
			return fmt.Errorf("runtime: absolute symlink target %q", h.Linkname)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("runtime: mkdir parent: %w", err)
		}
		_ = os.Remove(dst)
		if err := os.Symlink(h.Linkname, dst); err != nil {
			return fmt.Errorf("runtime: symlink: %w", err)
		}
		return nil
	default:
		// Ignore other types for now.
		return nil
	}
}
