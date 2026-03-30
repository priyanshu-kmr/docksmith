package image_test

import (
	"testing"

	"github.com/priyanshu/docksmith/internal/image"
	"github.com/priyanshu/docksmith/internal/layer"
)

func TestImageStore_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := image.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	// Create manifest
	m := image.NewManifest("myapp", "v1.0")
	m.AddLayer(layer.NewLayerInfo("sha256:layer123", 1024, "RUN apt-get update"))
	m.Config.SetEnv("FOO", "bar")
	m.Config.SetCmd([]string{"echo", "hello"})

	// Save
	err = store.Save(m)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load
	loaded, err := store.Load("myapp", "v1.0")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify
	if loaded.Name != "myapp" {
		t.Errorf("name mismatch: %s", loaded.Name)
	}
	if loaded.Tag != "v1.0" {
		t.Errorf("tag mismatch: %s", loaded.Tag)
	}
	if loaded.Digest != m.Digest {
		t.Errorf("digest mismatch: %s vs %s", loaded.Digest, m.Digest)
	}
	if len(loaded.Layers) != 1 {
		t.Errorf("expected 1 layer, got %d", len(loaded.Layers))
	}
	if loaded.Config.Env["FOO"] != "bar" {
		t.Error("config env mismatch")
	}
}

func TestImageStore_LoadDefaultTag(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := image.NewStore(tmpDir)

	m := image.NewManifest("myapp", "latest")
	store.Save(m)

	// Load without explicit tag (should use "latest")
	loaded, err := store.Load("myapp", "")
	if err != nil {
		t.Fatalf("Load with empty tag failed: %v", err)
	}

	if loaded.Tag != "latest" {
		t.Errorf("expected tag 'latest', got '%s'", loaded.Tag)
	}
}

func TestImageStore_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := image.NewStore(tmpDir)

	_, err := store.Load("nonexistent", "v1")
	if err != image.ErrImageNotFound {
		t.Errorf("expected ErrImageNotFound, got %v", err)
	}
}

func TestImageStore_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := image.NewStore(tmpDir)

	m := image.NewManifest("myapp", "v1")
	store.Save(m)

	if !store.Exists("myapp", "v1") {
		t.Error("image should exist after Save")
	}

	if store.Exists("nonexistent", "v1") {
		t.Error("nonexistent image should not exist")
	}
}

func TestImageStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := image.NewStore(tmpDir)

	m := image.NewManifest("myapp", "v1")
	store.Save(m)

	// Delete
	err := store.Delete("myapp", "v1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Should not exist
	if store.Exists("myapp", "v1") {
		t.Error("image should not exist after Delete")
	}
}

func TestImageStore_DeleteNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := image.NewStore(tmpDir)

	err := store.Delete("nonexistent", "v1")
	if err != image.ErrImageNotFound {
		t.Errorf("expected ErrImageNotFound, got %v", err)
	}
}

func TestImageStore_List(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := image.NewStore(tmpDir)

	// Save multiple images
	store.Save(image.NewManifest("app1", "v1"))
	store.Save(image.NewManifest("app1", "v2"))
	store.Save(image.NewManifest("app2", "latest"))

	manifests, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(manifests) != 3 {
		t.Errorf("expected 3 images, got %d", len(manifests))
	}
}

func TestImageStore_GetByDigest(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := image.NewStore(tmpDir)

	m := image.NewManifest("myapp", "v1")
	m.AddLayer(layer.NewLayerInfo("sha256:layer1", 100, ""))
	store.Save(m)

	// Get by digest
	found, err := store.GetByDigest(m.Digest)
	if err != nil {
		t.Fatalf("GetByDigest failed: %v", err)
	}

	if found.Name != "myapp" {
		t.Errorf("expected name 'myapp', got '%s'", found.Name)
	}
}

func TestImageStore_GetByDigest_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := image.NewStore(tmpDir)

	_, err := store.GetByDigest("nonexistent")
	if err != image.ErrImageNotFound {
		t.Errorf("expected ErrImageNotFound, got %v", err)
	}
}

func TestImageStore_SaveComputesDigest(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := image.NewStore(tmpDir)

	m := image.NewManifest("myapp", "v1")
	m.AddLayer(layer.NewLayerInfo("sha256:layer1", 100, ""))

	// Digest should be empty before save
	if m.Digest != "" {
		t.Error("digest should be empty before save")
	}

	store.Save(m)

	// Digest should be computed after save
	if m.Digest == "" {
		t.Error("digest should be computed after save")
	}
	if len(m.Digest) != 64 {
		t.Errorf("digest should be 64 chars, got %d", len(m.Digest))
	}
}
