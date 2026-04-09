package image

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

// Config holds the runtime configuration for an image.
// Internally uses map for Env, but serializes as ["KEY=value"] array per spec.
type Config struct {
	Env        map[string]string `json:"-"`        // Internal: map for easy lookup
	Cmd        []string          `json:"-"`        // Internal
	WorkingDir string            `json:"-"`        // Internal
}

// configJSON is the JSON-serializable form matching the spec.
type configJSON struct {
	Env        []string `json:"Env"`
	Cmd        []string `json:"Cmd"`
	WorkingDir string   `json:"WorkingDir"`
}

// NewConfig creates a new empty Config
func NewConfig() *Config {
	return &Config{
		Env:        make(map[string]string),
		Cmd:        nil,
		WorkingDir: "",
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

// EnvSlice returns env as sorted "KEY=value" slice (for JSON serialization).
func (c *Config) EnvSlice() []string {
	if len(c.Env) == 0 {
		return []string{}
	}
	keys := make([]string, 0, len(c.Env))
	for k := range c.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := make([]string, 0, len(keys))
	for _, k := range keys {
		result = append(result, k+"="+c.Env[k])
	}
	return result
}

// Hash computes a deterministic hash of the config.
func (c *Config) Hash() string {
	h := sha256.New()

	for _, entry := range c.EnvSlice() {
		h.Write([]byte(entry))
		h.Write([]byte("\n"))
	}

	for _, arg := range c.Cmd {
		h.Write([]byte(arg))
		h.Write([]byte("\x00"))
	}

	h.Write([]byte(c.WorkingDir))

	return hex.EncodeToString(h.Sum(nil))
}

// MarshalJSON serializes Config to spec-compliant JSON.
func (c *Config) MarshalJSON() ([]byte, error) {
	return json.Marshal(configJSON{
		Env:        c.EnvSlice(),
		Cmd:        c.Cmd,
		WorkingDir: c.WorkingDir,
	})
}

// UnmarshalJSON deserializes Config from JSON (handles array Env format).
func (c *Config) UnmarshalJSON(data []byte) error {
	var raw configJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		// Try legacy map format for backward compatibility
		type legacyConfig struct {
			Env        map[string]string `json:"env"`
			Cmd        []string          `json:"cmd"`
			WorkingDir string            `json:"workingDir"`
		}
		var legacy legacyConfig
		if err2 := json.Unmarshal(data, &legacy); err2 != nil {
			return err // return original error
		}
		c.Env = legacy.Env
		if c.Env == nil {
			c.Env = make(map[string]string)
		}
		c.Cmd = legacy.Cmd
		c.WorkingDir = legacy.WorkingDir
		return nil
	}

	c.Env = make(map[string]string)
	for _, entry := range raw.Env {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			c.Env[parts[0]] = parts[1]
		}
	}
	c.Cmd = raw.Cmd
	c.WorkingDir = raw.WorkingDir
	return nil
}
