package runtime

import "syscall"

// unixUnmount is a helper that performs a lazy unmount.
func unixUnmount(target string) error {
	return syscall.Unmount(target, syscall.MNT_DETACH)
}
