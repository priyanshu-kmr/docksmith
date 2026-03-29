package image

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// Config holds the runtime configuration for an image
type Config struct {
	Env        map[string]string `json:"env"`
	Cmd        []string          `json:"cmd"`
	WorkingDir string            `json:"workingDir"`
}

// NewConfig creates a new empty Config
func NewConfig() *Config {
	return &Config{
		Env:        make(map[string]string),
		Cmd:        nil,
		WorkingDir: "/",
	}
}

// Clone creates a deep copy of the Config
func (c *Config) Clone() *Config {
	envCopy := make(map[string]string, len(c.Env))
	for k, v := range c.Env {
		envCopy[k] = v
	}

	cmdCopy := make([]string, len(c.Cmd))
	copy(cmdCopy, c.Cmd)

	return &Config{
		Env:        envCopy,
		Cmd:        cmdCopy,
		WorkingDir: c.WorkingDir,
	}
}

// SetEnv sets an environment variable
func (c *Config) SetEnv(key, value string) {
	c.Env[key] = value
}

// SetCmd sets the default command
func (c *Config) SetCmd(cmd []string) {
	c.Cmd = make([]string, len(cmd))
	copy(c.Cmd, cmd)
}

// SetWorkingDir sets the working directory
func (c *Config) SetWorkingDir(dir string) {
	c.WorkingDir = dir
}

// Hash computes a deterministic hash of the config
// Used for image digest computation
func (c *Config) Hash() string {
	h := sha256.New()

	// Hash env in sorted order
	keys := make([]string, 0, len(c.Env))
	for k := range c.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte("="))
		h.Write([]byte(c.Env[k]))
		h.Write([]byte("\n"))
	}

	// Hash cmd
	for _, arg := range c.Cmd {
		h.Write([]byte(arg))
		h.Write([]byte("\x00"))
	}

	// Hash working dir
	h.Write([]byte(c.WorkingDir))

	return hex.EncodeToString(h.Sum(nil))
}

// MarshalJSON returns deterministic JSON representation
func (c *Config) MarshalJSON() ([]byte, error) {
	// Create ordered struct for deterministic output
	type orderedConfig struct {
		Env        map[string]string `json:"env"`
		Cmd        []string          `json:"cmd"`
		WorkingDir string            `json:"workingDir"`
	}

	return json.Marshal(orderedConfig{
		Env:        c.Env,
		Cmd:        c.Cmd,
		WorkingDir: c.WorkingDir,
	})
}
