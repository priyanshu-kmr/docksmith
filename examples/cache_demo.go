package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/priyanshu/docksmith/internal/cache"
	"github.com/priyanshu/docksmith/stubs"
)

func main() {
	// Create a temporary cache directory
	tmpDir := filepath.Join(os.TempDir(), "docksmith-cache-demo")
	os.RemoveAll(tmpDir) // Clean up any previous run
	defer os.RemoveAll(tmpDir)

	// Initialize cache manager
	manager, err := cache.NewManager(tmpDir)
	if err != nil {
		log.Fatalf("Failed to create cache manager: %v", err)
	}

	fmt.Println("=== Docksmith Cache System Demo ===")

	// Simulate a build context
	buildCtx := cache.NewBuildContext("/build")
	buildCtx.ApplyEnv("NODE_ENV", "production")
	buildCtx.ApplyWorkdir("/app")

	ctx := context.Background()

	// Example 1: RUN instruction
	fmt.Println("1. Computing cache key for RUN instruction...")
	runInst := &stubs.RunInstruction{
		Command: "npm install",
		Raw:     "RUN npm install",
	}

	key1, err := manager.ComputeKey("sha256:baseimage", runInst, buildCtx)
	if err != nil {
		log.Fatalf("ComputeKey failed: %v", err)
	}
	fmt.Printf("   Cache key: %s\n", key1)

	// Store cache entry
	entry1 := &cache.CacheEntry{
		Key:         key1,
		LayerDigest: "sha256:layer123abc",
		CreatedAt:   time.Now(),
		Size:        10240,
		Metadata: cache.EntryMetadata{
			InstructionType: "RUN",
			InstructionText: runInst.Raw,
		},
	}
	manager.Store(ctx, entry1)
	fmt.Println("   ✓ Cache entry stored")

	// Example 2: Cache hit - same instruction
	fmt.Println("\n2. Looking up same instruction (should hit cache)...")
	retrieved, err := manager.ComputeAndLookup(ctx, "sha256:baseimage", runInst, buildCtx)
	if err == nil {
		fmt.Printf("   ✓ Cache hit! Layer: %s\n", retrieved.LayerDigest)
	} else {
		fmt.Printf("   ✗ Cache miss: %v\n", err)
	}

	// Example 3: Cache miss - different ENV
	fmt.Println("\n3. Changing ENV variable...")
	buildCtx.ApplyEnv("NODE_ENV", "development")
	key2, _ := manager.ComputeKey("sha256:baseimage", runInst, buildCtx)
	fmt.Printf("   New cache key: %s\n", key2)
	fmt.Printf("   Keys are different: %v\n", key1 != key2)

	_, err = manager.Lookup(ctx, key2)
	if err == cache.ErrCacheMiss {
		fmt.Println("   ✓ Cache miss (expected - ENV changed)")
	}

	// Example 4: WORKDIR instruction
	fmt.Println("\n4. Computing cache key for WORKDIR instruction...")
	workdirInst := &stubs.WorkdirInstruction{
		Path: "/usr/src/app",
		Raw:  "WORKDIR /usr/src/app",
	}
	key3, _ := manager.ComputeKey("sha256:layer123abc", workdirInst, buildCtx)
	fmt.Printf("   Cache key: %s\n", key3)

	// Example 5: ENV instruction
	fmt.Println("\n5. Computing cache key for ENV instruction...")
	envInst := &stubs.EnvInstruction{
		Key:   "PORT",
		Value: "8080",
		Raw:   "ENV PORT=8080",
	}
	key4, _ := manager.ComputeKey("sha256:layer123abc", envInst, buildCtx)
	fmt.Printf("   Cache key: %s\n", key4)

	// Example 6: Cache statistics
	fmt.Println("\n6. Cache statistics...")
	stats, _ := manager.Stats(ctx)
	fmt.Printf("   Total entries: %d\n", stats.TotalEntries)
	fmt.Printf("   Total size: %d bytes\n", stats.TotalSize)

	// Example 7: Cache invalidation
	fmt.Println("\n7. Invalidating cache entry...")
	err = manager.Invalidate(ctx, key1)
	if err != nil {
		log.Fatalf("Invalidate failed: %v", err)
	}
	fmt.Println("   ✓ Cache entry invalidated")

	_, err = manager.Lookup(ctx, key1)
	if err == cache.ErrCacheMiss {
		fmt.Println("   ✓ Confirmed: entry no longer exists")
	}

	fmt.Println("\n=== Demo Complete ===")
	fmt.Printf("\nCache files created in: %s\n", tmpDir)
	fmt.Println("Inspect the directory to see the JSON cache entries!")
}
