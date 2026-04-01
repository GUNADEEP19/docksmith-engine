//go:build !linux

package isolation

import "fmt"

// Exec is not supported outside Linux. This project requires Linux primitives
// (chroot/namespaces) for proper filesystem isolation.
func Exec(rootfs string, workdir string, cmd []string, env map[string]string) (int, error) {
	_ = rootfs
	_ = workdir
	_ = cmd
	_ = env
	return 1, fmt.Errorf("runtime is only supported on Linux (requires chroot/unshare isolation)")
}
