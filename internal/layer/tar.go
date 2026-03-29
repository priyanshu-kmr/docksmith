package layer

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// TarCreator handles deterministic tar archive creation
type TarCreator struct{}

// NewTarCreator creates a new TarCreator instance
func NewTarCreator() *TarCreator {
	return &TarCreator{}
}

// fileEntry represents a file to be added to the tar archive
type fileEntry struct {
	path    string      // Relative path within tar
	absPath string      // Absolute path on filesystem
	info    os.FileInfo // File info
}

// CreateTar creates a deterministic tar archive from a directory
// Returns the SHA-256 digest, temporary tar path, size, and any error
// The caller is responsible for moving the tar to its final location
func (tc *TarCreator) CreateTar(srcDir string) (digest string, tarPath string, size int64, err error) {
	// Collect all files
	entries, err := tc.collectFiles(srcDir)
	if err != nil {
		return "", "", 0, fmt.Errorf("collect files: %w", err)
	}

	// Sort entries for deterministic output
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].path < entries[j].path
	})

	// Create temporary file for tar
	tmpFile, err := os.CreateTemp("", "docksmith-layer-*.tar")
	if err != nil {
		return "", "", 0, fmt.Errorf("create temp file: %w", err)
	}
	tarPath = tmpFile.Name()

	// Create hasher to compute digest while writing
	hasher := sha256.New()
	multiWriter := io.MultiWriter(tmpFile, hasher)

	// Create tar writer
	tw := tar.NewWriter(multiWriter)

	// Add all entries to tar
	for _, entry := range entries {
		if err := tc.addToTar(tw, entry, srcDir); err != nil {
			tmpFile.Close()
			tw.Close()
			os.Remove(tarPath)
			return "", "", 0, fmt.Errorf("add %s to tar: %w", entry.path, err)
		}
	}

	// Close tar writer
	if err := tw.Close(); err != nil {
		tmpFile.Close()
		os.Remove(tarPath)
		return "", "", 0, fmt.Errorf("close tar writer: %w", err)
	}

	// Get file size
	info, err := tmpFile.Stat()
	if err != nil {
		tmpFile.Close()
		os.Remove(tarPath)
		return "", "", 0, fmt.Errorf("stat tar file: %w", err)
	}
	size = info.Size()

	// Close file
	if err := tmpFile.Close(); err != nil {
		os.Remove(tarPath)
		return "", "", 0, fmt.Errorf("close tar file: %w", err)
	}

	// Compute final digest
	digest = hex.EncodeToString(hasher.Sum(nil))

	return digest, tarPath, size, nil
}

// collectFiles recursively collects all files in a directory
func (tc *TarCreator) collectFiles(srcDir string) ([]fileEntry, error) {
	var entries []fileEntry

	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if path == srcDir {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		entries = append(entries, fileEntry{
			path:    relPath,
			absPath: path,
			info:    info,
		})

		return nil
	})

	return entries, err
}

// addToTar adds a single entry to the tar archive with deterministic headers
func (tc *TarCreator) addToTar(tw *tar.Writer, entry fileEntry, srcDir string) error {
	// Create deterministic header
	header, err := tar.FileInfoHeader(entry.info, "")
	if err != nil {
		return err
	}

	// Set deterministic values
	header.Name = entry.path
	header.ModTime = time.Unix(0, 0) // Zero timestamp for reproducibility
	header.AccessTime = time.Unix(0, 0)
	header.ChangeTime = time.Unix(0, 0)
	header.Uid = 0
	header.Gid = 0
	header.Uname = ""
	header.Gname = ""

	// Handle symlinks
	if entry.info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(entry.absPath)
		if err != nil {
			return err
		}
		header.Linkname = link
	}

	// Write header
	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	// Write file content if regular file
	if entry.info.Mode().IsRegular() {
		f, err := os.Open(entry.absPath)
		if err != nil {
			return err
		}
		defer f.Close()

		if _, err := io.Copy(tw, f); err != nil {
			return err
		}
	}

	return nil
}

// ComputeTarDigest computes the SHA-256 digest of an existing tar file
func ComputeTarDigest(tarPath string) (string, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
