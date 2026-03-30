package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// FileHasher provides SHA-256 hashing utilities for files and directories
type FileHasher struct{}

// NewFileHasher creates a new FileHasher instance
func NewFileHasher() *FileHasher {
	return &FileHasher{}
}

// HashFile computes SHA-256 hash of a file and writes to the provided writer
// Includes file path for uniqueness in the hash
func (fh *FileHasher) HashFile(w io.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file %s: %w", path, err)
	}
	defer f.Close()

	// Include file path for uniqueness
	if _, err := w.Write([]byte("FILE:")); err != nil {
		return err
	}
	if _, err := w.Write([]byte(filepath.Base(path))); err != nil {
		return err
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		return err
	}

	// Hash file contents
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hash file contents %s: %w", path, err)
	}

	// Write hash to output
	if _, err := w.Write(h.Sum(nil)); err != nil {
		return err
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		return err
	}

	return nil
}

// HashDirectory recursively hashes all files in a directory
// Files are processed in sorted order for deterministic output
func (fh *FileHasher) HashDirectory(w io.Writer, dirPath string) error {
	// Collect all files first for deterministic ordering
	var files []string
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, err := filepath.Rel(dirPath, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk directory %s: %w", dirPath, err)
	}

	// Sort for deterministic order
	sort.Strings(files)

	// Hash each file
	for _, relPath := range files {
		fullPath := filepath.Join(dirPath, relPath)
		if err := fh.HashFile(w, fullPath); err != nil {
			return err
		}
	}

	return nil
}

// ComputeFileDigest returns hex-encoded SHA-256 of a file
func (fh *FileHasher) ComputeFileDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file %s: %w", path, err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash file %s: %w", path, err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
