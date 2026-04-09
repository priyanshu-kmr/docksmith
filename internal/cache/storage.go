package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// DiskStorage implements Cache interface with file-based storage
type DiskStorage struct {
	baseDir string
	mu      sync.RWMutex // Protects concurrent access
	stats   *CacheStats  // In-memory statistics
}

// NewDiskStorage creates a new disk-based cache storage
func NewDiskStorage(baseDir string) (*DiskStorage, error) {
	// Ensure cache directory exists
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}

	storage := &DiskStorage{
		baseDir: baseDir,
		stats: &CacheStats{
			TotalEntries: 0,
			TotalSize:    0,
			HitRate:      0.0,
		},
	}

	// Load existing cache statistics
	if err := storage.loadStats(); err != nil {
		// Non-fatal, just log
		fmt.Fprintf(os.Stderr, "Warning: failed to load cache stats: %v\n", err)
	}

	return storage, nil
}

// entryPath returns the file path for a cache entry
// Uses first 2 chars for sharding to avoid too many files in one directory
func (ds *DiskStorage) entryPath(key CacheKey) string {
	keyStr := string(key)
	if len(keyStr) < 2 {
		return filepath.Join(ds.baseDir, "entries", keyStr+".json")
	}
	return filepath.Join(ds.baseDir, "entries", keyStr[:2], keyStr+".json")
}

// Lookup retrieves a cache entry by key
func (ds *DiskStorage) Lookup(ctx context.Context, key CacheKey) (*CacheEntry, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	path := ds.entryPath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("read cache entry: %w", err)
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("unmarshal cache entry: %w", err)
	}

	return &entry, nil
}

// Store saves a cache entry
func (ds *DiskStorage) Store(ctx context.Context, entry *CacheEntry) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	path := ds.entryPath(entry.Key)

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create cache subdir: %w", err)
	}

	// Serialize entry
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache entry: %w", err)
	}

	// Write atomically using temp file + rename
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("write cache entry: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath) // Clean up on failure
		return fmt.Errorf("rename cache entry: %w", err)
	}

	// Update statistics
	ds.stats.TotalEntries++
	ds.stats.TotalSize += entry.Size

	return nil
}

// Invalidate removes a specific cache entry
func (ds *DiskStorage) Invalidate(ctx context.Context, key CacheKey) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	path := ds.entryPath(key)

	// Get entry to update stats before deletion
	data, err := os.ReadFile(path)
	if err == nil {
		var entry CacheEntry
		if json.Unmarshal(data, &entry) == nil {
			ds.stats.TotalSize -= entry.Size
			ds.stats.TotalEntries--
		}
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove cache entry: %w", err)
	}

	return nil
}

// InvalidateAll clears the entire cache
func (ds *DiskStorage) InvalidateAll(ctx context.Context) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	entriesDir := filepath.Join(ds.baseDir, "entries")
	if err := os.RemoveAll(entriesDir); err != nil {
		return fmt.Errorf("remove cache entries: %w", err)
	}

	// Reset statistics
	ds.stats.TotalEntries = 0
	ds.stats.TotalSize = 0
	ds.stats.HitRate = 0.0

	return nil
}

// Stats returns cache statistics
func (ds *DiskStorage) Stats(ctx context.Context) (*CacheStats, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	statsCopy := *ds.stats
	return &statsCopy, nil
}

// loadStats loads statistics from disk
func (ds *DiskStorage) loadStats() error {
	statsPath := filepath.Join(ds.baseDir, "stats.json")
	data, err := os.ReadFile(statsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No stats yet, that's okay
		}
		return err
	}

	return json.Unmarshal(data, ds.stats)
}

// SaveStats persists statistics to disk
func (ds *DiskStorage) SaveStats() error {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	statsPath := filepath.Join(ds.baseDir, "stats.json")
	data, err := json.MarshalIndent(ds.stats, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(statsPath, data, 0644)
}
