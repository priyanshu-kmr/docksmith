package cache

import (
	"context"
	"errors"

	"github.com/priyanshu/docksmith/stubs"
)

var (
	// ErrCacheMiss is returned when a cache entry is not found
	ErrCacheMiss = errors.New("cache miss")

	// ErrInvalidKey is returned when a cache key is invalid
	ErrInvalidKey = errors.New("invalid cache key")
)

// Cache provides the main caching interface
type Cache interface {
	// Lookup checks if a cache entry exists for the given key
	Lookup(ctx context.Context, key CacheKey) (*CacheEntry, error)

	// Store saves a cache entry
	Store(ctx context.Context, entry *CacheEntry) error

	// Invalidate removes a specific cache entry
	Invalidate(ctx context.Context, key CacheKey) error

	// InvalidateAll clears the entire cache
	InvalidateAll(ctx context.Context) error

	// Stats returns cache statistics
	Stats(ctx context.Context) (*CacheStats, error)
}

// Manager handles cache operations and key computation
type Manager struct {
	cache   Cache
	keygen  *KeyGenerator
	baseDir string // ~/.docksmith/cache/
}

// NewManager creates a new cache manager with disk-based storage
func NewManager(baseDir string) (*Manager, error) {
	storage, err := NewDiskStorage(baseDir)
	if err != nil {
		return nil, err
	}

	return &Manager{
		cache:   storage,
		keygen:  NewKeyGenerator(),
		baseDir: baseDir,
	}, nil
}

// ComputeKey computes the cache key for an instruction
func (m *Manager) ComputeKey(
	prevDigest LayerDigest,
	instruction stubs.Instruction,
	buildCtx *BuildContext,
) (CacheKey, error) {
	return m.keygen.ComputeKey(prevDigest, instruction, buildCtx)
}

// ComputeAndLookup computes cache key and looks up entry in one operation
func (m *Manager) ComputeAndLookup(
	ctx context.Context,
	prevDigest LayerDigest,
	instruction stubs.Instruction,
	buildCtx *BuildContext,
) (*CacheEntry, error) {
	key, err := m.ComputeKey(prevDigest, instruction, buildCtx)
	if err != nil {
		return nil, err
	}

	return m.cache.Lookup(ctx, key)
}

// Store saves a cache entry
func (m *Manager) Store(ctx context.Context, entry *CacheEntry) error {
	return m.cache.Store(ctx, entry)
}

// Lookup retrieves a cache entry by key
func (m *Manager) Lookup(ctx context.Context, key CacheKey) (*CacheEntry, error) {
	return m.cache.Lookup(ctx, key)
}

// Invalidate removes a specific cache entry
func (m *Manager) Invalidate(ctx context.Context, key CacheKey) error {
	return m.cache.Invalidate(ctx, key)
}

// InvalidateAll clears the entire cache
func (m *Manager) InvalidateAll(ctx context.Context) error {
	return m.cache.InvalidateAll(ctx)
}

// Stats returns cache statistics
func (m *Manager) Stats(ctx context.Context) (*CacheStats, error) {
	return m.cache.Stats(ctx)
}
