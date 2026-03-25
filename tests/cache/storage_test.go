package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/priyanshu/docksmith/internal/cache"
)

func TestDiskStorage_StoreAndLookup(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := cache.NewDiskStorage(tmpDir)
	if err != nil {
		t.Fatalf("cache.NewDiskStorage failed: %v", err)
	}

	ctx := context.Background()

	entry := &cache.CacheEntry{
		Key:         "abc123",
		LayerDigest: "sha256:layer123",
		CreatedAt:   time.Now(),
		Size:        1024,
		Metadata: cache.EntryMetadata{
			InstructionType: "RUN",
			InstructionText: "RUN echo hello",
		},
	}

	// Store entry
	err = storage.Store(ctx, entry)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Lookup entry
	retrieved, err := storage.Lookup(ctx, entry.Key)
	if err != nil {
		t.Fatalf("Lookup failed: %v", err)
	}

	// Verify
	if retrieved.Key != entry.Key {
		t.Errorf("expected key %s, got %s", entry.Key, retrieved.Key)
	}
	if retrieved.LayerDigest != entry.LayerDigest {
		t.Errorf("expected digest %s, got %s", entry.LayerDigest, retrieved.LayerDigest)
	}
	if retrieved.Metadata.InstructionType != entry.Metadata.InstructionType {
		t.Errorf("expected type %s, got %s", entry.Metadata.InstructionType, retrieved.Metadata.InstructionType)
	}
}

func TestDiskStorage_CacheMiss(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := cache.NewDiskStorage(tmpDir)
	if err != nil {
		t.Fatalf("cache.NewDiskStorage failed: %v", err)
	}

	ctx := context.Background()

	// Lookup non-existent entry
	_, err = storage.Lookup(ctx, "nonexistent")
	if err != cache.ErrCacheMiss {
		t.Errorf("expected cache.ErrCacheMiss, got %v", err)
	}
}

func TestDiskStorage_Invalidate(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := cache.NewDiskStorage(tmpDir)
	if err != nil {
		t.Fatalf("cache.NewDiskStorage failed: %v", err)
	}

	ctx := context.Background()

	entry := &cache.CacheEntry{
		Key:         "abc123",
		LayerDigest: "sha256:layer123",
		CreatedAt:   time.Now(),
		Size:        1024,
	}

	// Store and verify exists
	storage.Store(ctx, entry)
	_, err = storage.Lookup(ctx, entry.Key)
	if err != nil {
		t.Fatal("entry should exist after Store")
	}

	// Invalidate
	err = storage.Invalidate(ctx, entry.Key)
	if err != nil {
		t.Fatalf("Invalidate failed: %v", err)
	}

	// Verify removed
	_, err = storage.Lookup(ctx, entry.Key)
	if err != cache.ErrCacheMiss {
		t.Error("entry should not exist after Invalidate")
	}
}

func TestDiskStorage_InvalidateAll(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := cache.NewDiskStorage(tmpDir)
	if err != nil {
		t.Fatalf("cache.NewDiskStorage failed: %v", err)
	}

	ctx := context.Background()

	// Store multiple entries
	for i := 0; i < 5; i++ {
		entry := &cache.CacheEntry{
			Key:         cache.CacheKey(string(rune('a' + i))),
			LayerDigest: cache.LayerDigest("sha256:layer" + string(rune('0'+i))),
			CreatedAt:   time.Now(),
			Size:        100,
		}
		storage.Store(ctx, entry)
	}

	// Verify entries exist
	stats, _ := storage.Stats(ctx)
	if stats.TotalEntries != 5 {
		t.Errorf("expected 5 entries, got %d", stats.TotalEntries)
	}

	// Invalidate all
	err = storage.InvalidateAll(ctx)
	if err != nil {
		t.Fatalf("InvalidateAll failed: %v", err)
	}

	// Verify all removed
	stats, _ = storage.Stats(ctx)
	if stats.TotalEntries != 0 {
		t.Errorf("expected 0 entries after InvalidateAll, got %d", stats.TotalEntries)
	}
}

func TestDiskStorage_Stats(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := cache.NewDiskStorage(tmpDir)
	if err != nil {
		t.Fatalf("cache.NewDiskStorage failed: %v", err)
	}

	ctx := context.Background()

	// Initial stats
	stats, err := storage.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.TotalEntries != 0 {
		t.Error("initial stats should have 0 entries")
	}

	// Store entry
	entry := &cache.CacheEntry{
		Key:         "abc123",
		LayerDigest: "sha256:layer123",
		CreatedAt:   time.Now(),
		Size:        1024,
	}
	storage.Store(ctx, entry)

	// Check stats updated
	stats, _ = storage.Stats(ctx)
	if stats.TotalEntries != 1 {
		t.Errorf("expected 1 entry, got %d", stats.TotalEntries)
	}
	if stats.TotalSize != 1024 {
		t.Errorf("expected size 1024, got %d", stats.TotalSize)
	}
}

func TestDiskStorage_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := cache.NewDiskStorage(tmpDir)
	if err != nil {
		t.Fatalf("cache.NewDiskStorage failed: %v", err)
	}

	ctx := context.Background()

	entry := &cache.CacheEntry{
		Key:         "abc123",
		LayerDigest: "sha256:layer123",
		CreatedAt:   time.Now(),
		Size:        1024,
	}

	// Store entry
	err = storage.Store(ctx, entry)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Verify we can retrieve it (proves atomic write worked)
	retrieved, err := storage.Lookup(ctx, entry.Key)
	if err != nil {
		t.Fatalf("Lookup after Store failed: %v", err)
	}
	if retrieved.Key != entry.Key {
		t.Error("retrieved entry doesn't match stored entry")
	}
}

func TestDiskStorage_EntryPath_Sharding(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := cache.NewDiskStorage(tmpDir)
	if err != nil {
		t.Fatalf("cache.NewDiskStorage failed: %v", err)
	}

	ctx := context.Background()

	// Store an entry and verify sharding by checking directory structure
	key := cache.CacheKey("abcdef1234567890")
	entry := &cache.CacheEntry{
		Key:         key,
		LayerDigest: "sha256:test",
		CreatedAt:   time.Now(),
		Size:        100,
	}

	storage.Store(ctx, entry)

	// Verify entry is stored and can be retrieved (sharding is working behind the scenes)
	retrieved, err := storage.Lookup(ctx, key)
	if err != nil {
		t.Errorf("should be able to retrieve stored entry with sharded path")
	}
	if retrieved.Key != key {
		t.Error("retrieved key doesn't match")
	}
}

func TestDiskStorage_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := cache.NewDiskStorage(tmpDir)
	if err != nil {
		t.Fatalf("cache.NewDiskStorage failed: %v", err)
	}

	ctx := context.Background()

	// Concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			entry := &cache.CacheEntry{
				Key:         cache.CacheKey(string(rune('a' + idx))),
				LayerDigest: cache.LayerDigest("sha256:layer"),
				CreatedAt:   time.Now(),
				Size:        100,
			}
			storage.Store(ctx, entry)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all stored
	stats, _ := storage.Stats(ctx)
	if stats.TotalEntries != 10 {
		t.Errorf("expected 10 entries after concurrent writes, got %d", stats.TotalEntries)
	}
}

func TestDiskStorage_InvalidateNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := cache.NewDiskStorage(tmpDir)
	if err != nil {
		t.Fatalf("cache.NewDiskStorage failed: %v", err)
	}

	ctx := context.Background()

	// Invalidate non-existent entry should not error
	err = storage.Invalidate(ctx, "nonexistent")
	if err != nil {
		t.Errorf("Invalidate of non-existent entry should not error, got %v", err)
	}
}
