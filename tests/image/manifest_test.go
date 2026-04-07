package image_test

import (
	"strings"
	"testing"

	"github.com/priyanshu/docksmith/internal/image"
	"github.com/priyanshu/docksmith/internal/layer"
)

func TestManifest_NewManifest(t *testing.T) {
	m := image.NewManifest("myapp", "v1.0")

	if m.Name != "myapp" {
		t.Errorf("expected name 'myapp', got '%s'", m.Name)
	}
	if m.Tag != "v1.0" {
		t.Errorf("expected tag 'v1.0', got '%s'", m.Tag)
	}
	if m.Config == nil {
		t.Error("config should not be nil")
	}
	if len(m.Layers) != 0 {
		t.Error("layers should be empty initially")
	}
}

func TestManifest_DefaultTag(t *testing.T) {
	m := image.NewManifest("myapp", "")

	if m.Tag != "latest" {
		t.Errorf("expected default tag 'latest', got '%s'", m.Tag)
	}
}

func TestManifest_AddLayer(t *testing.T) {
	m := image.NewManifest("myapp", "v1")

	l := layer.NewLayerInfo("sha256:abc123", 1024, "RUN apt-get update")
	m.AddLayer(l)

	if len(m.Layers) != 1 {
		t.Errorf("expected 1 layer, got %d", len(m.Layers))
	}
	if m.Layers[0].Digest != "sha256:abc123" {
		t.Error("layer digest mismatch")
	}
}

func TestManifest_ComputeDigest_Deterministic(t *testing.T) {
	m1 := image.NewManifest("myapp", "v1")
	m1.AddLayer(layer.NewLayerInfo("sha256:layer1", 100, "RUN cmd1"))
	m1.AddLayer(layer.NewLayerInfo("sha256:layer2", 200, "RUN cmd2"))
	m1.Config.SetEnv("FOO", "bar")

	m2 := image.NewManifest("myapp", "v1")
	m2.AddLayer(layer.NewLayerInfo("sha256:layer1", 100, "RUN cmd1"))
	m2.AddLayer(layer.NewLayerInfo("sha256:layer2", 200, "RUN cmd2"))
	m2.Config.SetEnv("FOO", "bar")

	digest1 := m1.ComputeDigest()
	digest2 := m2.ComputeDigest()

	if digest1 != digest2 {
		t.Errorf("same manifest should produce same digest: %s vs %s", digest1, digest2)
	}

	if !strings.HasPrefix(digest1, "sha256:") {
		t.Errorf("digest should have sha256: prefix, got %s", digest1)
	}

	if len(strings.TrimPrefix(digest1, "sha256:")) != 64 {
		t.Errorf("digest payload should be 64 chars (SHA-256 hex), got %d", len(strings.TrimPrefix(digest1, "sha256:")))
	}
}

func TestManifest_ComputeDigest_DifferentLayers(t *testing.T) {
	m1 := image.NewManifest("myapp", "v1")
	m1.AddLayer(layer.NewLayerInfo("sha256:layer1", 100, "RUN cmd1"))

	m2 := image.NewManifest("myapp", "v1")
	m2.AddLayer(layer.NewLayerInfo("sha256:layer2", 100, "RUN cmd1"))

	digest1 := m1.ComputeDigest()
	digest2 := m2.ComputeDigest()

	if digest1 == digest2 {
		t.Error("different layers should produce different digest")
	}
}

func TestManifest_GetLayerDigests(t *testing.T) {
	m := image.NewManifest("myapp", "v1")
	m.AddLayer(layer.NewLayerInfo("sha256:layer1", 100, ""))
	m.AddLayer(layer.NewLayerInfo("sha256:layer2", 200, ""))
	m.AddLayer(layer.NewLayerInfo("sha256:layer3", 300, ""))

	digests := m.GetLayerDigests()

	if len(digests) != 3 {
		t.Errorf("expected 3 digests, got %d", len(digests))
	}
	if digests[0] != "sha256:layer1" || digests[1] != "sha256:layer2" || digests[2] != "sha256:layer3" {
		t.Error("layer order incorrect")
	}
}

func TestManifest_TotalSize(t *testing.T) {
	m := image.NewManifest("myapp", "v1")
	m.AddLayer(layer.NewLayerInfo("sha256:layer1", 100, ""))
	m.AddLayer(layer.NewLayerInfo("sha256:layer2", 200, ""))
	m.AddLayer(layer.NewLayerInfo("sha256:layer3", 300, ""))

	total := m.TotalSize()

	if total != 600 {
		t.Errorf("expected total size 600, got %d", total)
	}
}

func TestManifest_Reference(t *testing.T) {
	m := image.NewManifest("myapp", "v1.0")

	ref := m.Reference()

	if ref != "myapp:v1.0" {
		t.Errorf("expected 'myapp:v1.0', got '%s'", ref)
	}
}

func TestManifest_Clone(t *testing.T) {
	m := image.NewManifest("myapp", "v1")
	m.AddLayer(layer.NewLayerInfo("sha256:layer1", 100, "RUN cmd"))
	m.Config.SetEnv("FOO", "bar")
	m.ComputeDigest()

	clone := m.Clone()

	// Verify clone has same values
	if clone.Name != m.Name || clone.Tag != m.Tag || clone.Digest != m.Digest {
		t.Error("clone should have same basic values")
	}

	if len(clone.Layers) != len(m.Layers) {
		t.Error("clone should have same number of layers")
	}

	if clone.Config.Env["FOO"] != "bar" {
		t.Error("clone should have same config")
	}

	// Modify clone - should not affect original
	clone.Config.SetEnv("FOO", "modified")
	if m.Config.Env["FOO"] != "bar" {
		t.Error("modifying clone should not affect original")
	}
}
