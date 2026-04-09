package runtime

import (
	"fmt"
	"os"
	"strings"

	"github.com/priyanshu/docksmith/internal/image"
	"github.com/priyanshu/docksmith/internal/layer"
)

// Container manages the lifecycle of a container process.
type Container struct {
	manifest   *image.Manifest
	extractor  *layer.Extractor
	rootFS     string
	envOverrides map[string]string
	cmdOverride  []string
}

// NewContainer creates a new container from a manifest.
func NewContainer(manifest *image.Manifest, extractor *layer.Extractor) *Container {
	return &Container{
		manifest:     manifest,
		extractor:    extractor,
		envOverrides: make(map[string]string),
	}
}

// SetEnvOverride sets an environment variable override.
func (c *Container) SetEnvOverride(key, value string) {
	c.envOverrides[key] = value
}

// SetCmdOverride sets the command override (replaces image CMD).
func (c *Container) SetCmdOverride(cmd []string) {
	c.cmdOverride = cmd
}

// Run assembles the filesystem, starts the isolated process, waits for exit,
// and cleans up. Returns the exit code.
func (c *Container) Run() (int, error) {
	// Determine command
	cmd := c.manifest.Config.Cmd
	if len(c.cmdOverride) > 0 {
		cmd = c.cmdOverride
	}
	if len(cmd) == 0 {
		return 1, fmt.Errorf("no CMD defined in image %s and no command provided", c.manifest.Reference())
	}

	// Assemble rootfs
	rootFS, err := c.assembleRootFS()
	if err != nil {
		return 1, fmt.Errorf("assemble rootfs: %w", err)
	}
	c.rootFS = rootFS
	defer c.cleanup()

	// Build environment: image ENV + overrides
	env := make(map[string]string)
	for k, v := range c.manifest.Config.Env {
		env[k] = v
	}
	for k, v := range c.envOverrides {
		env[k] = v
	}

	// Determine working directory
	workDir := c.manifest.Config.WorkingDir
	if workDir == "" {
		workDir = "/"
	}

	cfg := RunConfig{
		RootFS:  rootFS,
		Command: cmd,
		Env:     env,
		WorkDir: workDir,
	}

	return RunIsolated(cfg)
}

// assembleRootFS creates a temporary directory and extracts all layers.
func (c *Container) assembleRootFS() (string, error) {
	tmpDir, err := os.MkdirTemp("", "docksmith-rootfs-")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	// Extract layers in order
	digests := make([]string, len(c.manifest.Layers))
	for i, l := range c.manifest.Layers {
		digests[i] = strings.TrimPrefix(l.Digest, "sha256:")
	}

	if err := c.extractor.ExtractLayers(digests, tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("extract layers: %w", err)
	}

	return tmpDir, nil
}

// cleanup removes the temporary rootfs directory.
func (c *Container) cleanup() {
	if c.rootFS != "" {
		// Unmount any lingering mounts before removal
		unmountAll(c.rootFS)
		os.RemoveAll(c.rootFS)
		c.rootFS = ""
	}
}

// unmountAll attempts to unmount known mount points.
func unmountAll(rootFS string) {
	mountPoints := []string{"proc", "dev", "sys", "tmp"}
	for _, mp := range mountPoints {
		target := rootFS + "/" + mp
		// Best-effort unmount
		_ = unixUnmount(target)
	}
}
