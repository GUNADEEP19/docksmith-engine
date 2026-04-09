package provision

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ProvisionBaseBinaries copies essential host binaries and their shared
// library dependencies into rootfs. This provides a minimal userland so that
// commands like "sh -c ..." work inside a chroot.
//
// This is used by both:
// - runtime: docksmith run
// - build: RUN during layer creation
func ProvisionBaseBinaries(rootfs string) error {
	binaries := []string{
		"sh", "bash", "cat", "echo", "ls", "env", "mkdir", "rm",
		"cp", "mv", "chmod", "chown", "grep", "sed", "awk",
		"head", "tail", "wc", "touch", "date", "uname",
		"dirname", "basename", "tr", "tee", "sort", "uniq",
		"find", "xargs", "test", "true", "false", "sleep",
		"printf", "pwd", "id", "whoami",
	}

	copied := make(map[string]bool)

	for _, name := range binaries {
		hostPath, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		hostPath, err = filepath.Abs(hostPath)
		if err != nil {
			continue
		}
		if err := copyFileToRootfs(rootfs, hostPath, copied); err != nil {
			continue
		}
		ensureMirrorLink(rootfs, hostPath)
	}

	resolveSharedLibs(rootfs, copied)
	return nil
}

func copyFileToRootfs(rootfs, hostPath string, copied map[string]bool) error {
	if copied[hostPath] {
		return nil
	}
	st, err := os.Stat(hostPath)
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return nil
	}
	dest := filepath.Join(rootfs, hostPath)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	in, err := os.Open(hostPath)
	if err != nil {
		return err
	}
	defer in.Close()
	mode := st.Mode() & os.ModePerm
	if mode == 0 {
		mode = 0o755
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	copied[hostPath] = true
	return nil
}

func ensureMirrorLink(rootfs, hostPath string) {
	base := filepath.Base(hostPath)
	var mirror string
	if strings.HasPrefix(hostPath, "/bin/") {
		mirror = filepath.Join(rootfs, "/usr/bin", base)
	} else if strings.HasPrefix(hostPath, "/usr/bin/") {
		mirror = filepath.Join(rootfs, "/bin", base)
	} else {
		return
	}
	if err := os.MkdirAll(filepath.Dir(mirror), 0o755); err != nil {
		return
	}
	_ = os.Remove(mirror)
	// Absolute link target works inside chroot.
	_ = os.Symlink(hostPath, mirror)
}

func resolveSharedLibs(rootfs string, copied map[string]bool) {
	// Best-effort: ldd is host-specific; ignore failures.
	for hostPath := range copied {
		// Only attempt for binaries likely in standard locations.
		if !strings.HasPrefix(hostPath, "/bin/") && !strings.HasPrefix(hostPath, "/usr/bin/") {
			continue
		}
		cmd := exec.Command("ldd", hostPath)
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			// Typical formats:
			//   libz.so.1 => /lib/x86_64-linux-gnu/libz.so.1 (0x...)
			//   /lib64/ld-linux-x86-64.so.2 (0x...)
			var libPath string
			if strings.Contains(line, "=>") {
				parts := strings.Split(line, "=>")
				if len(parts) < 2 {
					continue
				}
				rhs := strings.TrimSpace(parts[1])
				fields := strings.Fields(rhs)
				if len(fields) > 0 {
					libPath = fields[0]
				}
			} else {
				fields := strings.Fields(line)
				if len(fields) > 0 && strings.HasPrefix(fields[0], "/") {
					libPath = fields[0]
				}
			}
			if strings.HasPrefix(libPath, "/") {
				_ = copyFileToRootfs(rootfs, libPath, copied)
			}
		}
	}
}

func DebugDump(rootfs string) string {
	// Helper for local debugging; keep it simple and safe.
	return fmt.Sprintf("rootfs=%s", rootfs)
}
