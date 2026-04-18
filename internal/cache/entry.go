package cache

import (
	"time"
)

// CacheKey is a SHA-256 hash representing a unique build state
type CacheKey string

// LayerDigest is the digest of the resulting layer
type LayerDigest string

// CacheEntry represents a cached layer
type CacheEntry struct {
	Key         CacheKey      `json:"key"`
	LayerDigest LayerDigest   `json:"layer_digest"`
	CreatedAt   time.Time     `json:"created_at"`
	Size        int64         `json:"size"`
	Metadata    EntryMetadata `json:"metadata"`
}

// EntryMetadata stores additional information about cached layer
type EntryMetadata struct {
	InstructionType string `json:"instruction_type"` // FROM, RUN, COPY, etc.
	InstructionText string `json:"instruction_text"` // Raw instruction text
}

// CacheStats provides cache usage statistics
type CacheStats struct {
	TotalEntries int     `json:"total_entries"`
	TotalSize    int64   `json:"total_size"`
	HitRate      float64 `json:"hit_rate"`
}