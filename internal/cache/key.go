package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/priyanshu/docksmith/stubs"
)

// KeyGenerator handles cache key computation
type KeyGenerator struct {
	hasher *FileHasher
}

// NewKeyGenerator creates a new KeyGenerator instance
func NewKeyGenerator() *KeyGenerator {
	return &KeyGenerator{
		hasher: NewFileHasher(),
	}
}

// ComputeKey generates a deterministic cache key based on:
// - Previous layer digest
// - Instruction text
// - Current WORKDIR
// - Current ENV state
// - COPY source file hashes (for COPY instructions)
func (kg *KeyGenerator) ComputeKey(
	prevDigest LayerDigest,
	instruction stubs.Instruction,
	buildCtx *BuildContext,
) (CacheKey, error) {
	h := sha256.New()

	// 1. Include previous layer digest
	h.Write([]byte(prevDigest))
	h.Write([]byte("\n"))

	// 2. Include instruction type and text
	h.Write([]byte(instruction.Type()))
	h.Write([]byte("\n"))
	h.Write([]byte(instruction.Text()))
	h.Write([]byte("\n"))

	// 3. Include current WORKDIR (for both COPY and RUN)
	h.Write([]byte("WORKDIR="))
	h.Write([]byte(buildCtx.Workdir))
	h.Write([]byte("\n"))

	// 4. Include current ENV state (for both COPY and RUN)
	h.Write([]byte("ENV\n"))
	h.Write([]byte(buildCtx.SerializeEnv()))

	// 5. Instruction-specific cache inputs
	switch inst := instruction.(type) {
	case *stubs.CopyInstruction:
		// For COPY, also hash source file contents
		if err := kg.hashCopySourcesInto(h, inst, buildCtx); err != nil {
			return "", fmt.Errorf("hashing COPY sources: %w", err)
		}
	case *stubs.RunInstruction:
		// RUN has no additional inputs beyond what's above
		_ = inst
	}

	sum := h.Sum(nil)
	return CacheKey(hex.EncodeToString(sum)), nil
}

// hashCopySourcesInto hashes all source files for a COPY instruction
func (kg *KeyGenerator) hashCopySourcesInto(
	h io.Writer,
	inst *stubs.CopyInstruction,
	buildCtx *BuildContext,
) error {
	// Sort sources for deterministic ordering
	sources := make([]string, len(inst.Sources))
	copy(sources, inst.Sources)
	sort.Strings(sources)

	for _, src := range sources {
		srcPath := filepath.Join(buildCtx.ContextDir, src)

		// Check if it's a directory or file
		info, err := os.Stat(srcPath)
		if err != nil {
			return fmt.Errorf("stat %s: %w", srcPath, err)
		}

		if info.IsDir() {
			// Hash entire directory tree
			if err := kg.hasher.HashDirectory(h, srcPath); err != nil {
				return err
			}
		} else {
			// Hash single file
			if err := kg.hasher.HashFile(h, srcPath); err != nil {
				return err
			}
		}
	}

	return nil
}
