package layer_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/priyanshu/docksmith/internal/layer"
)

func TestLayerStore_StoreAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := layer.NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}

	// Create a test tar
	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("test content"), 0644)

	tc := layer.NewTarCreator()
	_, tarPath, _, _ := tc.CreateTar(srcDir)
	defer os.Remove(tarPath)

	// Open tar file
	f, _ := os.Open(tarPath)
	defer f.Close()

	// Store it
	digest, size, err := store.Store(f)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	if digest == "" {
		t.Error("digest should not be empty")
	}
	if size == 0 {
		t.Error("size should not be zero")
	}

	// Get it back
	reader, err := store.Get(digest)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	reader.Close()

	// Verify file exists at expected path
	expectedPath := store.Path(digest)
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Error("layer file should exist at expected path")
	}
}

func TestLayerStore_ContentAddressed(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := layer.NewStore(tmpDir)

	// Create identical content twice
	content := []byte("identical content")

	digest1, _, _ := store.Store(bytes.NewReader(content))
	digest2, _, _ := store.Store(bytes.NewReader(content))

	// Same content = same digest
	if digest1 != digest2 {
		t.Errorf("same content should produce same digest: %s vs %s", digest1, digest2)
	}

	// Should only be one file
	digests, _ := store.List()
	if len(digests) != 1 {
		t.Errorf("expected 1 layer, got %d", len(digests))
	}
}

func TestLayerStore_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := layer.NewStore(tmpDir)

	content := []byte("test")
	digest, _, _ := store.Store(bytes.NewReader(content))

	if !store.Exists(digest) {
		t.Error("layer should exist after Store")
	}

	if store.Exists("nonexistent") {
		t.Error("nonexistent layer should not exist")
	}
}

func TestLayerStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := layer.NewStore(tmpDir)

	content := []byte("test")
	digest, _, _ := store.Store(bytes.NewReader(content))

	// Delete it
	err := store.Delete(digest)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Should not exist anymore
	if store.Exists(digest) {
		t.Error("layer should not exist after Delete")
	}
}

func TestLayerStore_List(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := layer.NewStore(tmpDir)

	// Store multiple layers
	store.Store(bytes.NewReader([]byte("layer 1")))
	store.Store(bytes.NewReader([]byte("layer 2")))
	store.Store(bytes.NewReader([]byte("layer 3")))

	digests, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(digests) != 3 {
		t.Errorf("expected 3 layers, got %d", len(digests))
	}
}

func TestLayerStore_GetNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := layer.NewStore(tmpDir)

	_, err := store.Get("nonexistent")
	if err == nil {
		t.Error("Get should fail for nonexistent layer")
	}
}
