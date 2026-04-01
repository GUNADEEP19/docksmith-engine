package runtime

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"docksmith-engine/internal"
	"docksmith-engine/internal/isolation"
)

// Service implements internal.Runtime.
type Service struct {
	root string
}

// Option configures runtime service construction.
type Option func(*Service)

// WithDataRoot sets the Docksmith data root (default: ~/.docksmith).
func WithDataRoot(root string) Option {
	return func(s *Service) {
		if strings.TrimSpace(root) != "" {
			s.root = root
		}
	}
}

// New creates a runtime service.
func New(opts ...Option) *Service {
	h, err := os.UserHomeDir()
	if err != nil {
		h = "."
	}
	s := &Service{root: filepath.Join(h, ".docksmith")}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Run extracts image layers into a temporary rootfs and executes the command in isolation.
func (s *Service) Run(image internal.Image, cmd []string, env map[string]string) (int, error) {
	if len(cmd) == 0 {
		return 1, fmt.Errorf("runtime: no command provided")
	}

	rootfs, err := os.MkdirTemp("", "docksmith-rootfs-*")
	if err != nil {
		return 1, fmt.Errorf("runtime: create temp rootfs: %w", err)
	}
	defer os.RemoveAll(rootfs)

	if err := s.extractLayers(rootfs, image.Layers); err != nil {
		return 1, err
	}

	workdir := strings.TrimSpace(image.Config.WorkingDir)
	if workdir == "" {
		workdir = "/"
	}
	if !path.IsAbs(workdir) {
		workdir = "/" + strings.TrimLeft(workdir, "/")
	}
	if err := os.MkdirAll(filepath.Join(rootfs, filepath.FromSlash(strings.TrimPrefix(path.Clean(workdir), "/"))), 0o755); err != nil {
		return 1, fmt.Errorf("runtime: create workdir %s: %w", workdir, err)
	}

	envMap := map[string]string{}
	for k, v := range image.Config.Env {
		envMap[k] = v
	}
	for k, v := range env {
		envMap[k] = v
	}
	if _, ok := envMap["PATH"]; !ok {
		envMap["PATH"] = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	}

	return isolation.RunIsolated(rootfs, workdir, cmd, envMap)
}

func (s *Service) extractLayers(rootfs string, layers []internal.ImageLayer) error {
	for _, l := range layers {
		digest := strings.TrimSpace(l.Digest)
		if err := validateDigest(digest); err != nil {
			return err
		}
		tarPath := filepath.Join(s.root, "layers", fmt.Sprintf("sha256_%s.tar", strings.TrimPrefix(digest, "sha256:")))
		if err := extractTarInto(rootfs, tarPath); err != nil {
			return fmt.Errorf("runtime: extract layer %s: %w", digest, err)
		}
	}
	return nil
}

func validateDigest(d string) error {
	if !strings.HasPrefix(d, "sha256:") {
		return fmt.Errorf("runtime: invalid layer digest %q", d)
	}
	hex := strings.TrimPrefix(d, "sha256:")
	if len(hex) != 64 {
		return fmt.Errorf("runtime: invalid layer digest length")
	}
	for _, c := range hex {
		if c >= '0' && c <= '9' || c >= 'a' && c <= 'f' {
			continue
		}
		return fmt.Errorf("runtime: invalid digest hex")
	}
	return nil
}

func extractTarInto(root string, tarPath string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err := safeExtractTarEntry(root, tr, hdr); err != nil {
			return err
		}
	}
	return nil
}

func safeExtractTarEntry(root string, tr io.Reader, hdr *tar.Header) error {
	clean := path.Clean(hdr.Name)
	if clean == "." || strings.HasPrefix(clean, "..") {
		return fmt.Errorf("runtime: invalid tar path %q", hdr.Name)
	}
	target := filepath.Join(root, filepath.FromSlash(clean))

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(targetAbs+string(os.PathSeparator), rootAbs+string(os.PathSeparator)) && targetAbs != rootAbs {
		return fmt.Errorf("runtime: tar entry escapes root: %q", hdr.Name)
	}

	switch hdr.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, os.FileMode(hdr.Mode))
	case tar.TypeReg, tar.TypeRegA:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
		if err != nil {
			return err
		}
		_, cErr := io.Copy(f, tr)
		closeErr := f.Close()
		if cErr != nil {
			return cErr
		}
		if closeErr != nil {
			return closeErr
		}
		return nil
	default:
		return nil
	}
}
