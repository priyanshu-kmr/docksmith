package image

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/priyanshu/docksmith/internal/layer"
)

// Manifest represents an image manifest
type Manifest struct {
	Name    string             `json:"name"`
	Tag     string             `json:"tag"`
	Digest  string             `json:"digest"`
	Created time.Time          `json:"created"`
	Config  *Config            `json:"config"`
	Layers  []*layer.LayerInfo `json:"layers"`
}

// NewManifest creates a new manifest with the given name and tag
func NewManifest(name, tag string) *Manifest {
	if tag == "" {
		tag = "latest"
	}

	return &Manifest{
		Name:    name,
		Tag:     tag,
		Created: time.Now().UTC(),
		Config:  NewConfig(),
		Layers:  make([]*layer.LayerInfo, 0),
	}
}

// AddLayer adds a layer to the manifest
func (m *Manifest) AddLayer(l *layer.LayerInfo) {
	m.Layers = append(m.Layers, l)
}

// ComputeDigest computes the image digest from layers and config
// Must be called after all layers are added
func (m *Manifest) ComputeDigest() string {
	h := sha256.New()

	// Hash all layer digests in order
	for _, l := range m.Layers {
		h.Write([]byte(l.Digest))
		h.Write([]byte("\n"))
	}

	// Hash config
	h.Write([]byte(m.Config.Hash()))

	m.Digest = hex.EncodeToString(h.Sum(nil))
	return m.Digest
}

// GetLayerDigests returns all layer digests in order
func (m *Manifest) GetLayerDigests() []string {
	digests := make([]string, len(m.Layers))
	for i, l := range m.Layers {
		digests[i] = l.Digest
	}
	return digests
}

// TotalSize returns the total size of all layers
func (m *Manifest) TotalSize() int64 {
	var total int64
	for _, l := range m.Layers {
		total += l.Size
	}
	return total
}

// Reference returns the image reference (name:tag)
func (m *Manifest) Reference() string {
	return m.Name + ":" + m.Tag
}

// Clone creates a deep copy of the manifest
func (m *Manifest) Clone() *Manifest {
	layers := make([]*layer.LayerInfo, len(m.Layers))
	for i, l := range m.Layers {
		layers[i] = layer.NewLayerInfo(l.Digest, l.Size, l.CreatedBy)
	}

	return &Manifest{
		Name:    m.Name,
		Tag:     m.Tag,
		Digest:  m.Digest,
		Created: m.Created,
		Config:  m.Config.Clone(),
		Layers:  layers,
	}
}
