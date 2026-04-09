package layer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Store manages layer storage in ~/.docksmith/layers/
type Store struct {
	baseDir string
	mu      sync.RWMutex
}

// NewStore creates a new layer store
func NewStore(baseDir string) (*Store, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create layers dir: %w", err)
	}

	return &Store{
		baseDir: baseDir,
	}, nil
}

// Store saves a tar file and returns its digest
// The tar is read, hashed, and stored with its digest as the filename
func (s *Store) Store(tarReader io.Reader) (string, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create temp file to store content
	tmpFile, err := os.CreateTemp(s.baseDir, "layer-*.tar.tmp")
	if err != nil {
		return "", 0, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Copy content to temp file
	size, err := io.Copy(tmpFile, tarReader)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", 0, fmt.Errorf("copy tar content: %w", err)
	}
	tmpFile.Close()

	// Compute digest
	digest, err := ComputeTarDigest(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		return "", 0, fmt.Errorf("compute digest: %w", err)
	}

	// Final path based on digest
	finalPath := s.Path(digest)

	// If layer already exists, remove temp and return existing
	if s.Exists(digest) {
		os.Remove(tmpPath)
		return digest, size, nil
	}

	// Rename to final location
	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return "", 0, fmt.Errorf("rename to final path: %w", err)
	}

	return digest, size, nil
}

// StoreFromPath stores an existing tar file
func (s *Store) StoreFromPath(tarPath string) (string, int64, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return "", 0, fmt.Errorf("open tar: %w", err)
	}
	defer f.Close()

	return s.Store(f)
}

// Get retrieves a layer by its digest
func (s *Store) Get(digest string) (io.ReadCloser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := s.Path(digest)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("layer not found: %s", digest)
		}
		return nil, err
	}

	return f, nil
}

// Exists checks if a layer exists
func (s *Store) Exists(digest string) bool {
	path := s.Path(digest)
	_, err := os.Stat(path)
	return err == nil
}

// Delete removes a layer by its digest
func (s *Store) Delete(digest string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.Path(digest)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete layer: %w", err)
	}
	return nil
}

// Path returns the filesystem path for a layer
func (s *Store) Path(digest string) string {
	return filepath.Join(s.baseDir, digest+".tar")
}

// List returns all layer digests in the store
func (s *Store) List() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("read layers dir: %w", err)
	}

	var digests []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) == ".tar" {
			digest := name[:len(name)-4] // Remove .tar extension
			digests = append(digests, digest)
		}
	}

	return digests, nil
}
