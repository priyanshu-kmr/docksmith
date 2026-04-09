package layer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/priyanshu/docksmith/internal/layer"
)

func TestCreateTar_Deterministic(t *testing.T) {
	// Create temporary directory with test files
	tmpDir := t.TempDir()

	// Create test structure
	os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content 1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "subdir", "file2.txt"), []byte("content 2"), 0644)

	tc := layer.NewTarCreator()

	// Create tar twice
	digest1, path1, size1, err1 := tc.CreateTar(tmpDir)
	if err1 != nil {
		t.Fatalf("first CreateTar failed: %v", err1)
	}
	defer os.Remove(path1)

	digest2, path2, size2, err2 := tc.CreateTar(tmpDir)
	if err2 != nil {
		t.Fatalf("second CreateTar failed: %v", err2)
	}
	defer os.Remove(path2)

	// CRITICAL: Same directory must produce same digest
	if digest1 != digest2 {
		t.Errorf("CreateTar is not deterministic: digest1=%s, digest2=%s", digest1, digest2)
	}

	if size1 != size2 {
		t.Errorf("sizes differ: %d vs %d", size1, size2)
	}
}

func TestCreateTar_DifferentContent(t *testing.T) {
	tc := layer.NewTarCreator()

	// Create first directory
	tmpDir1 := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir1, "file.txt"), []byte("content 1"), 0644)

	// Create second directory with different content
	tmpDir2 := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir2, "file.txt"), []byte("content 2"), 0644)

	digest1, path1, _, _ := tc.CreateTar(tmpDir1)
	defer os.Remove(path1)

	digest2, path2, _, _ := tc.CreateTar(tmpDir2)
	defer os.Remove(path2)

	if digest1 == digest2 {
		t.Error("different content should produce different digests")
	}
}

func TestCreateTar_SortedEntries(t *testing.T) {
	tc := layer.NewTarCreator()

	// Create directory with files in non-alphabetical order
	tmpDir := t.TempDir()

	// Create files with names that would sort differently
	os.WriteFile(filepath.Join(tmpDir, "zebra.txt"), []byte("z"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "apple.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "mango.txt"), []byte("m"), 0644)

	// Create tar twice
	digest1, path1, _, _ := tc.CreateTar(tmpDir)
	defer os.Remove(path1)

	// Create same files in different order (shouldn't matter)
	os.Remove(filepath.Join(tmpDir, "apple.txt"))
	os.WriteFile(filepath.Join(tmpDir, "apple.txt"), []byte("a"), 0644)

	digest2, path2, _, _ := tc.CreateTar(tmpDir)
	defer os.Remove(path2)

	if digest1 != digest2 {
		t.Error("tar should sort entries - creation order shouldn't matter")
	}
}

func TestCreateTar_EmptyDirectory(t *testing.T) {
	tc := layer.NewTarCreator()
	tmpDir := t.TempDir()

	digest, path, size, err := tc.CreateTar(tmpDir)
	if err != nil {
		t.Fatalf("CreateTar failed on empty dir: %v", err)
	}
	defer os.Remove(path)

	if digest == "" {
		t.Error("digest should not be empty")
	}

	if size == 0 {
		t.Error("tar should have some size even if empty")
	}
}

func TestCreateTar_WithSubdirectories(t *testing.T) {
	tc := layer.NewTarCreator()
	tmpDir := t.TempDir()

	// Create nested structure
	os.MkdirAll(filepath.Join(tmpDir, "a", "b", "c"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "root.txt"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "a", "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "a", "b", "b.txt"), []byte("b"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "a", "b", "c", "c.txt"), []byte("c"), 0644)

	digest, path, _, err := tc.CreateTar(tmpDir)
	if err != nil {
		t.Fatalf("CreateTar failed: %v", err)
	}
	defer os.Remove(path)

	if digest == "" {
		t.Error("digest should not be empty")
	}

	// Should be deterministic
	digest2, path2, _, _ := tc.CreateTar(tmpDir)
	defer os.Remove(path2)

	if digest != digest2 {
		t.Error("nested directories should produce deterministic output")
	}
}

func TestComputeTarDigest(t *testing.T) {
	tc := layer.NewTarCreator()
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644)

	digest1, path, _, _ := tc.CreateTar(tmpDir)
	defer os.Remove(path)

	// Compute digest from file
	digest2, err := layer.ComputeTarDigest(path)
	if err != nil {
		t.Fatalf("ComputeTarDigest failed: %v", err)
	}

	if digest1 != digest2 {
		t.Errorf("digests don't match: %s vs %s", digest1, digest2)
	}
}
