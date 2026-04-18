package runtime

import (
	"fmt"
	"strings"
	"syscall"
)

// unixUnmount is a helper that performs a lazy unmount.
func unixUnmount(target string) error {
	return syscall.Unmount(target, syscall.MNT_DETACH)
}

func mountOverlayFS(lowerDirs []string, upperDir, workDir, mergedDir string) error {
	if len(lowerDirs) == 0 {
		return fmt.Errorf("lowerdirs cannot be empty")
	}
	data := fmt.Sprintf(
		"lowerdir=%s,upperdir=%s,workdir=%s",
		strings.Join(lowerDirs, ":"),
		upperDir,
		workDir,
	)
	return syscall.Mount("overlay", mergedDir, "overlay", 0, data)
}
