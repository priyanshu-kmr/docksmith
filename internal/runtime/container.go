package runtime

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/priyanshu/docksmith/internal/image"
	"github.com/priyanshu/docksmith/internal/layer"
)

type containerConfig struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Image      string     `json:"image"`
	Command    []string   `json:"command"`
	Created    time.Time  `json:"created"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	PID        int        `json:"pid,omitempty"`
	ExitCode   *int       `json:"exitCode,omitempty"`
	Layers     []string   `json:"layers"`
}

// Container manages the lifecycle of a persistent container process.
type Container struct {
	manifest      *image.Manifest
	extractor     *layer.Extractor
	containersDir string
	layerFSDir    string

	envOverrides map[string]string
	cmdOverride  []string
	nameOverride string
}

// NewContainer creates a new persistent container from a manifest.
func NewContainer(manifest *image.Manifest, extractor *layer.Extractor, containersDir, layerFSDir string) *Container {
	return &Container{
		manifest:      manifest,
		extractor:     extractor,
		containersDir: containersDir,
		layerFSDir:    layerFSDir,
		envOverrides:  make(map[string]string),
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

// SetNameOverride sets a custom persisted container name.
func (c *Container) SetNameOverride(name string) {
	c.nameOverride = name
}

// Run mounts OverlayFS, executes the isolated process in merged rootfs, and unmounts.
func (c *Container) Run() (int, error) {
	cmd := c.manifest.Config.Cmd
	if len(c.cmdOverride) > 0 {
		cmd = c.cmdOverride
	}
	if len(cmd) == 0 {
		return 1, fmt.Errorf("no CMD defined in image %s and no command provided", c.manifest.Reference())
	}

	if err := os.MkdirAll(c.containersDir, 0755); err != nil {
		return 1, fmt.Errorf("create containers dir: %w", err)
	}
	if err := os.MkdirAll(c.layerFSDir, 0755); err != nil {
		return 1, fmt.Errorf("create layerfs dir: %w", err)
	}

	idKey := c.manifest.Reference()
	if c.nameOverride != "" {
		idKey = c.nameOverride + "@" + c.manifest.Reference()
	}
	containerID := deterministicContainerID(idKey)
	containerPath := filepath.Join(c.containersDir, containerID)

	cfg, err := c.ensureContainerConfig(containerPath, containerID, cmd)
	if err != nil {
		return 1, err
	}
	if c.nameOverride != "" {
		cfg.Name = c.nameOverride
	}
	cfg.Command = append([]string{}, cmd...)

	lowerDirs, err := c.ensureLayerDirs(cfg.Layers)
	if err != nil {
		return 1, err
	}
	if len(lowerDirs) == 0 {
		return 1, errors.New("image has no layers for runtime rootfs")
	}

	if err := ensureOverlayDirs(containerPath); err != nil {
		return 1, err
	}
	mergedPath := filepath.Join(containerPath, "merged")
	upperPath := filepath.Join(containerPath, "upper")
	workPath := filepath.Join(containerPath, "work")

	if err := mountOverlayFS(lowerDirs, upperPath, workPath, mergedPath); err != nil {
		return 1, fmt.Errorf("mount overlayfs: %w", err)
	}
	defer func() {
		_ = unixUnmount(mergedPath)
	}()

	env := make(map[string]string, len(c.manifest.Config.Env)+len(c.envOverrides))
	for k, v := range c.manifest.Config.Env {
		env[k] = v
	}
	for k, v := range c.envOverrides {
		env[k] = v
	}

	workDir := c.manifest.Config.WorkingDir
	if workDir == "" {
		workDir = "/"
	}
	startedAt := time.Now().UTC()
	cfg.StartedAt = &startedAt
	cfg.FinishedAt = nil
	cfg.ExitCode = nil
	cfg.PID = 0
	if err := c.writeContainerConfig(containerPath, cfg); err != nil {
		return 1, err
	}

	exitCode, runErr := runIsolated(RunConfig{
		RootFS:  mergedPath,
		Command: cmd,
		Env:     env,
		WorkDir: workDir,
	}, func(pid int) error {
		cfg.PID = pid
		return c.writeContainerConfig(containerPath, cfg)
	})
	finishedAt := time.Now().UTC()
	cfg.FinishedAt = &finishedAt
	cfg.PID = 0
	if runErr == nil {
		code := exitCode
		cfg.ExitCode = &code
	} else {
		code := 1
		cfg.ExitCode = &code
	}
	_ = c.writeContainerConfig(containerPath, cfg)
	return exitCode, runErr
}

func deterministicContainerID(imageRef string) string {
	sum := sha256.Sum256([]byte(imageRef))
	return hex.EncodeToString(sum[:])[:8]
}

func layerDigestPathComponent(layerDigest string) string {
	return strings.TrimPrefix(layerDigest, "sha256:")
}

func layerDigestForConfig(layerDigest string) string {
	d := layerDigestPathComponent(layerDigest)
	return "sha256:" + d
}

func sortedLayerDigestsForOverlay(layers []string) []string {
	dirs := make([]string, 0, len(layers))
	for i := len(layers) - 1; i >= 0; i-- {
		dirs = append(dirs, layers[i])
	}
	return dirs
}

func (c *Container) ensureContainerConfig(containerPath, containerID string, command []string) (*containerConfig, error) {
	if err := os.MkdirAll(containerPath, 0755); err != nil {
		return nil, fmt.Errorf("create container dir: %w", err)
	}
	configPath := filepath.Join(containerPath, "config.json")

	if _, err := os.Stat(configPath); err == nil {
		data, readErr := os.ReadFile(configPath)
		if readErr != nil {
			return nil, fmt.Errorf("read container config: %w", readErr)
		}
		var cfg containerConfig
		if unmarshalErr := json.Unmarshal(data, &cfg); unmarshalErr != nil {
			return nil, fmt.Errorf("decode container config: %w", unmarshalErr)
		}
		return &cfg, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat container config: %w", err)
	}

	cfg := &containerConfig{
		ID:      containerID,
		Name:    "ctr-" + containerID,
		Image:   c.manifest.Reference(),
		Command: append([]string{}, command...),
		Created: time.Now().UTC(),
		Layers:  make([]string, len(c.manifest.Layers)),
	}
	for i, l := range c.manifest.Layers {
		cfg.Layers[i] = layerDigestForConfig(l.Digest)
	}

	if err := c.writeContainerConfig(containerPath, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Container) writeContainerConfig(containerPath string, cfg *containerConfig) error {
	configPath := filepath.Join(containerPath, "config.json")
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode container config: %w", err)
	}
	if err := os.WriteFile(configPath, raw, 0644); err != nil {
		return fmt.Errorf("write container config: %w", err)
	}
	return nil
}

func (c *Container) ensureLayerDirs(layerDigests []string) ([]string, error) {
	layerDirs := make([]string, 0, len(layerDigests))
	for _, digest := range layerDigests {
		d := layerDigestPathComponent(digest)
		target := filepath.Join(c.layerFSDir, d)
		layerDirs = append(layerDirs, target)

		if _, err := os.Stat(target); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat layerfs %s: %w", d, err)
		}

		if err := os.MkdirAll(target, 0755); err != nil {
			return nil, fmt.Errorf("create layerfs dir for %s: %w", d, err)
		}
		if err := c.extractor.ExtractLayer(d, target); err != nil {
			_ = os.RemoveAll(target)
			return nil, fmt.Errorf("extract layer %s to layerfs: %w", d, err)
		}
	}

	return sortedLayerDigestsForOverlay(layerDirs), nil
}

func ensureOverlayDirs(containerPath string) error {
	for _, d := range []string{"upper", "work", "merged"} {
		if err := os.MkdirAll(filepath.Join(containerPath, d), 0755); err != nil {
			return fmt.Errorf("create %s: %w", d, err)
		}
	}
	return nil
}
