//go:build !linux

package isolation

import "fmt"

// RunIsolated is only supported on Linux where chroot/namespaces are available.
func RunIsolated(rootfs string, workdir string, cmd []string, env map[string]string) (int, error) {
	_ = rootfs
	_ = workdir
	_ = cmd
	_ = env
	return 1, fmt.Errorf("runtime isolation requires Linux (chroot/namespaces)")
}
