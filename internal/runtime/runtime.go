package runtime

import (
	"archive/tar"
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"docksmith-engine/internal"
	"docksmith-engine/internal/isolation"
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
	if err := provisionBaseBinaries(rootfs); err != nil {
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

// provisionBaseBinaries copies essential host binaries and their shared
// library dependencies into the chroot rootfs. This gives the container a
// minimal userland so that shell commands work.
func provisionBaseBinaries(rootfs string) error {
	// Essential binaries to look for on the host.
	binaries := []string{
		"sh", "bash", "cat", "echo", "ls", "env", "mkdir", "rm",
		"cp", "mv", "chmod", "chown", "grep", "sed", "awk",
		"head", "tail", "wc", "touch", "date", "uname",
		"dirname", "basename", "tr", "tee", "sort", "uniq",
		"find", "xargs", "test", "true", "false", "sleep",
		"printf", "pwd", "id", "whoami",
	}

	copied := make(map[string]bool) // track files we already copied

	for _, name := range binaries {
		hostPath, err := exec.LookPath(name)
		if err != nil {
			continue // not available on host, skip
		}
		hostPath, err = filepath.Abs(hostPath)
		if err != nil {
			continue
		}
		if err := copyFileToRootfs(rootfs, hostPath, copied); err != nil {
			continue // best-effort
		}
		// Also create a symlink from /usr/bin/<name> if the binary lives
		// in /bin (or vice versa) so both paths work inside the chroot.
		ensureMirrorLink(rootfs, hostPath)
	}

	// Copy shared library dependencies of all binaries we placed.
	resolveSharedLibs(rootfs, copied)

	return nil
}

// copyFileToRootfs copies a single host file into the same absolute path
// inside rootfs, preserving the executable bit.
func copyFileToRootfs(rootfs, hostPath string, copied map[string]bool) error {
	if copied[hostPath] {
		return nil
	}

	dst := filepath.Join(rootfs, hostPath)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	in, err := os.Open(hostPath)
	if err != nil {
		return err
	}
	defer in.Close()

	st, err := in.Stat()
	if err != nil {
		return err
	}

	mode := st.Mode() | 0o555 // ensure executable
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	copied[hostPath] = true
	return nil
}

// ensureMirrorLink creates symlinks between /bin and /usr/bin inside the
// rootfs so that binaries can be found at either path (many systems use
// both interchangeably).
func ensureMirrorLink(rootfs, hostPath string) {
	dir := filepath.Dir(hostPath)
	base := filepath.Base(hostPath)

	var mirror string
	switch dir {
	case "/bin":
		mirror = filepath.Join(rootfs, "/usr/bin", base)
	case "/usr/bin":
		mirror = filepath.Join(rootfs, "/bin", base)
	default:
		return
	}

	// Don't overwrite a real file.
	if _, err := os.Lstat(mirror); err == nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(mirror), 0o755)
	_ = os.Symlink(hostPath, mirror)
}

// resolveSharedLibs uses ldd to discover shared library dependencies for
// every binary already copied, then copies those libraries into the rootfs.
func resolveSharedLibs(rootfs string, copied map[string]bool) {
	// Collect all binaries we need to inspect.
	var bins []string
	for p := range copied {
		bins = append(bins, p)
	}

	libs := make(map[string]bool)
	for _, bin := range bins {
		out, err := exec.Command("ldd", bin).Output()
		if err != nil {
			continue // statically linked or ldd not available
		}
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			// Typical ldd output line:  libfoo.so => /lib/x86_64-linux-gnu/libfoo.so (0xaddr)
			// or:  /lib64/ld-linux-x86-64.so.2 (0xaddr)
			if strings.Contains(line, "=>") {
				parts := strings.SplitN(line, "=>", 2)
				if len(parts) == 2 {
					rhs := strings.TrimSpace(parts[1])
					// Strip the (0x...) address suffix.
					if idx := strings.Index(rhs, " ("); idx >= 0 {
						rhs = rhs[:idx]
					}
					rhs = strings.TrimSpace(rhs)
					if rhs != "" && filepath.IsAbs(rhs) {
						libs[rhs] = true
					}
				}
			} else if filepath.IsAbs(line) {
				// Direct path like /lib64/ld-linux-x86-64.so.2 (0x...)
				p := line
				if idx := strings.Index(p, " ("); idx >= 0 {
					p = p[:idx]
				}
				p = strings.TrimSpace(p)
				if p != "" {
					libs[p] = true
				}
			}
		}
	}

	// Copy each library into rootfs.
	for lib := range libs {
		if copied[lib] {
			continue
		}
		// Resolve symlinks to get the real file.
		real, err := filepath.EvalSymlinks(lib)
		if err != nil {
			continue
		}
		// Copy the real file.
		_ = copyFileToRootfs(rootfs, real, copied)
		// If lib is a symlink, recreate the symlink in rootfs.
		if real != lib {
			dst := filepath.Join(rootfs, lib)
			_ = os.MkdirAll(filepath.Dir(dst), 0o755)
			_ = os.Remove(dst)
			// Create a symlink pointing to the real path (absolute, works inside chroot).
			_ = os.Symlink(real, dst)
		}
	}
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
