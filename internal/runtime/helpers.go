package runtime

import "os"

// CleanupRootFS unmounts virtual filesystems and removes the rootfs directory.
// This is exported for use by the build engine after RUN steps.
func CleanupRootFS(rootFS string) {
	if rootFS == "" {
		return
	}
	unmountAll(rootFS)
	// Note: the caller is responsible for os.RemoveAll if needed
}

// AssembleRootFS creates a temporary directory suitable for use as a container root.
// Returns the path to the temp directory.
func AssembleRootFS() (string, error) {
	return os.MkdirTemp("", "docksmith-rootfs-")
}
