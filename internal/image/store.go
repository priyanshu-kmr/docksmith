package image

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	// ErrImageNotFound is returned when an image is not found
	ErrImageNotFound = errors.New("image not found")
)

// Store manages image manifests in ~/.docksmith/images/
type Store struct {
	baseDir string
	mu      sync.RWMutex
}

// NewStore creates a new image store
func NewStore(baseDir string) (*Store, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create images dir: %w", err)
	}

	return &Store{
		baseDir: baseDir,
	}, nil
}

// manifestPath returns the path for a manifest file
func (s *Store) manifestPath(name, tag string) string {
	// Sanitize name for filesystem (replace / with _)
	safeName := strings.ReplaceAll(name, "/", "_")
	return filepath.Join(s.baseDir, safeName+"_"+tag+".json")
}

// Save persists a manifest to disk
func (s *Store) Save(m *Manifest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure digest is computed
	if m.Digest == "" {
		m.ComputeDigest()
	}

	path := s.manifestPath(m.Name, m.Tag)

	// Marshal to JSON
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	// Write atomically using temp file + rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename manifest: %w", err)
	}

	return nil
}

// Load reads a manifest from disk
func (s *Store) Load(name, tag string) (*Manifest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if tag == "" {
		tag = "latest"
	}

	path := s.manifestPath(name, tag)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrImageNotFound
		}
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal manifest: %w", err)
	}

	return &m, nil
}

// Exists checks if an image exists
func (s *Store) Exists(name, tag string) bool {
	if tag == "" {
		tag = "latest"
	}

	path := s.manifestPath(name, tag)
	_, err := os.Stat(path)
	return err == nil
}

// Delete removes an image manifest
func (s *Store) Delete(name, tag string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tag == "" {
		tag = "latest"
	}

	path := s.manifestPath(name, tag)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return ErrImageNotFound
		}
		return fmt.Errorf("delete manifest: %w", err)
	}

	return nil
}

// List returns all images in the store
func (s *Store) List() ([]*Manifest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("read images dir: %w", err)
	}

	var manifests []*Manifest
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(s.baseDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // Skip unreadable files
		}

		var m Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			continue // Skip invalid manifests
		}

		manifests = append(manifests, &m)
	}

	return manifests, nil
}

// GetByDigest finds an image by its digest
func (s *Store) GetByDigest(digest string) (*Manifest, error) {
	manifests, err := s.List()
	if err != nil {
		return nil, err
	}

	for _, m := range manifests {
		if m.Digest == digest {
			return m, nil
		}
	}

	return nil, ErrImageNotFound
}
