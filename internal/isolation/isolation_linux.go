//go:build linux

package isolation

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Exec runs cmd inside a chroot at rootfs.
//
// This is the isolation boundary: the process will see rootfs as "/".
func Exec(rootfs string, workdir string, cmd []string, env map[string]string) (int, error) {
	if len(cmd) == 0 {
		return 1, errors.New("isolation: empty command")
	}

	c := exec.Command(cmd[0], cmd[1:]...)
	c.Dir = workdir
	c.Env = encodeEnv(env)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	c.SysProcAttr = &syscall.SysProcAttr{
		Chroot: rootfs,
		// Run as the invoking user inside the chroot.
		Credential: &syscall.Credential{
			Uid: uint32(os.Getuid()),
			Gid: uint32(os.Getgid()),
		},
	}

	err := c.Run()
	if err == nil {
		return 0, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if st, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			return st.ExitStatus(), nil
		}
		return 1, nil
	}

	// Most common case: not running as root / lacking permission for chroot.
	if errors.Is(err, syscall.EPERM) {
		return 1, fmt.Errorf("runtime isolation failed (chroot): %w (run as root or enable appropriate Linux permissions)", err)
	}
	return 1, fmt.Errorf("runtime exec failed: %w", err)
}
