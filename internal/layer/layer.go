package layer

// LayerInfo contains metadata about a layer
type LayerInfo struct {
	Digest    string `json:"digest"`     // SHA-256 of tar contents
	Size      int64  `json:"size"`       // Size in bytes
	CreatedBy string `json:"createdBy"` // Instruction that created this layer
}

// NewLayerInfo creates a new LayerInfo instance
func NewLayerInfo(digest string, size int64, createdBy string) *LayerInfo {
	return &LayerInfo{
		Digest:    digest,
		Size:      size,
		CreatedBy: createdBy,
	}
}
