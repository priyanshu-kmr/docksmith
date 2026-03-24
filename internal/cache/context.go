package cache

import (
	"path/filepath"
	"sort"
	"strings"
)

// BuildContext maintains the cumulative state during image build
type BuildContext struct {
	// Environment variables accumulated from ENV instructions
	Env map[string]string

	// Current working directory from WORKDIR instructions
	Workdir string

	// Base directory for resolving relative paths in COPY
	ContextDir string
}

// NewBuildContext creates a new build context with default values
func NewBuildContext(contextDir string) *BuildContext {
	return &BuildContext{
		Env:        make(map[string]string),
		Workdir:    "/",
		ContextDir: contextDir,
	}
}

// Clone creates a deep copy for cache key computation
func (bc *BuildContext) Clone() *BuildContext {
	envCopy := make(map[string]string, len(bc.Env))
	for k, v := range bc.Env {
		envCopy[k] = v
	}
	return &BuildContext{
		Env:        envCopy,
		Workdir:    bc.Workdir,
		ContextDir: bc.ContextDir,
	}
}

// ApplyEnv updates environment variable
func (bc *BuildContext) ApplyEnv(key, value string) {
	bc.Env[key] = value
}

// ApplyWorkdir updates working directory
// Handles both absolute and relative paths
func (bc *BuildContext) ApplyWorkdir(path string) {
	if filepath.IsAbs(path) {
		bc.Workdir = filepath.Clean(path)
	} else {
		bc.Workdir = filepath.Clean(filepath.Join(bc.Workdir, path))
	}
}

// SerializeEnv returns deterministic string representation of env vars
// Keys are sorted alphabetically for consistent output
func (bc *BuildContext) SerializeEnv() string {
	if len(bc.Env) == 0 {
		return ""
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(bc.Env))
	for k := range bc.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf strings.Builder
	for _, k := range keys {
		buf.WriteString(k)
		buf.WriteString("=")
		buf.WriteString(bc.Env[k])
		buf.WriteString("\n")
	}
	return buf.String()
}
