//go:build linux

package isolation

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"syscall"
)

// RunIsolated executes cmd inside rootfs using chroot on Linux.
func RunIsolated(rootfs string, workdir string, cmd []string, env map[string]string) (int, error) {
	if len(cmd) == 0 {
		return 1, fmt.Errorf("isolation: no command provided")
	}

	ec := exec.Command(cmd[0], cmd[1:]...)
	ec.Dir = workdir
	ec.Env = envMapToSlice(env)
	ec.Stdout = os.Stdout
	ec.Stderr = os.Stderr
	ec.Stdin = os.Stdin
	ec.SysProcAttr = &syscall.SysProcAttr{
		Chroot: rootfs,
	}

	err := ec.Run()
	if err == nil {
		return 0, nil
	}
	if ee, ok := err.(*exec.ExitError); ok {
		if status, ok := ee.Sys().(syscall.WaitStatus); ok {
			if status.Exited() {
				return status.ExitStatus(), nil
			}
		}
	}
	return 1, fmt.Errorf("isolation: exec failed: %w", err)
}

func envMapToSlice(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+env[k])
	}
	return out
}
