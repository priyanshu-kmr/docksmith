package layer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/priyanshu/docksmith/internal/layer"
)

func TestExtract_SingleLayer(t *testing.T) {
	// Create source directory with files
	srcDir := t.TempDir()
	os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content 1"), 0644)
	os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("content 2"), 0644)

	// Create tar
	tc := layer.NewTarCreator()
	_, tarPath, _, _ := tc.CreateTar(srcDir)
	defer os.Remove(tarPath)

	// Extract to new directory
	destDir := t.TempDir()

	tarFile, _ := os.Open(tarPath)
	defer tarFile.Close()

	err := layer.Extract(tarFile, destDir)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// Verify files exist
	content1, err := os.ReadFile(filepath.Join(destDir, "file1.txt"))
	if err != nil {
		t.Fatalf("file1.txt not extracted: %v", err)
	}
	if string(content1) != "content 1" {
		t.Errorf("file1.txt content mismatch: %s", content1)
	}

	content2, err := os.ReadFile(filepath.Join(destDir, "subdir", "file2.txt"))
	if err != nil {
		t.Fatalf("subdir/file2.txt not extracted: %v", err)
	}
	if string(content2) != "content 2" {
		t.Errorf("subdir/file2.txt content mismatch: %s", content2)
	}
}

func TestExtractor_ExtractLayers_MultipleOrdered(t *testing.T) {
	storeDir := t.TempDir()
	store, _ := layer.NewStore(storeDir)
	extractor := layer.NewExtractor(store)

	// Create and store first layer
	src1 := t.TempDir()
	os.WriteFile(filepath.Join(src1, "base.txt"), []byte("base"), 0644)
	os.WriteFile(filepath.Join(src1, "overwrite.txt"), []byte("original"), 0644)

	tc := layer.NewTarCreator()
	digest1, tarPath1, _, _ := tc.CreateTar(src1)
	store.StoreFromPath(tarPath1)
	os.Remove(tarPath1)

	// Create and store second layer (overwrites one file, adds another)
	src2 := t.TempDir()
	os.WriteFile(filepath.Join(src2, "overwrite.txt"), []byte("modified"), 0644)
	os.WriteFile(filepath.Join(src2, "new.txt"), []byte("new file"), 0644)

	digest2, tarPath2, _, _ := tc.CreateTar(src2)
	store.StoreFromPath(tarPath2)
	os.Remove(tarPath2)

	// Extract layers in order
	destDir := t.TempDir()
	err := extractor.ExtractLayers([]string{digest1, digest2}, destDir)
	if err != nil {
		t.Fatalf("ExtractLayers failed: %v", err)
	}

	// base.txt should exist from first layer
	base, err := os.ReadFile(filepath.Join(destDir, "base.txt"))
	if err != nil {
		t.Fatal("base.txt should exist from first layer")
	}
	if string(base) != "base" {
		t.Error("base.txt content incorrect")
	}

	// overwrite.txt should have second layer's content
	overwrite, _ := os.ReadFile(filepath.Join(destDir, "overwrite.txt"))
	if string(overwrite) != "modified" {
		t.Errorf("overwrite.txt should be 'modified', got '%s'", overwrite)
	}

	// new.txt should exist from second layer
	newFile, err := os.ReadFile(filepath.Join(destDir, "new.txt"))
	if err != nil {
		t.Fatal("new.txt should exist from second layer")
	}
	if string(newFile) != "new file" {
		t.Error("new.txt content incorrect")
	}
}

func TestExtractor_ExtractLayer(t *testing.T) {
	storeDir := t.TempDir()
	store, _ := layer.NewStore(storeDir)
	extractor := layer.NewExtractor(store)

	// Create and store a layer
	srcDir := t.TempDir()
	os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("test"), 0644)

	tc := layer.NewTarCreator()
	digest, tarPath, _, _ := tc.CreateTar(srcDir)
	store.StoreFromPath(tarPath)
	os.Remove(tarPath)

	// Extract single layer
	destDir := t.TempDir()
	err := extractor.ExtractLayer(digest, destDir)
	if err != nil {
		t.Fatalf("ExtractLayer failed: %v", err)
	}

	// Verify
	content, _ := os.ReadFile(filepath.Join(destDir, "test.txt"))
	if string(content) != "test" {
		t.Error("extracted content incorrect")
	}
}
