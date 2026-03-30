package cache_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/priyanshu/docksmith/internal/cache"
)

func TestFileHasher_HashFile(t *testing.T) {
	hasher := cache.NewFileHasher()
	var buf bytes.Buffer

	testFile := filepath.Join("testdata", "file1.txt")
	err := hasher.HashFile(&buf, testFile)
	if err != nil {
		t.Fatalf("HashFile failed: %v", err)
	}

	// Output should contain file marker and hash
	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("FILE:")) {
		t.Error("output should contain FILE: marker")
	}
}

func TestFileHasher_HashFile_Deterministic(t *testing.T) {
	hasher := cache.NewFileHasher()

	testFile := filepath.Join("testdata", "file1.txt")

	// Hash twice
	var buf1, buf2 bytes.Buffer
	err1 := hasher.HashFile(&buf1, testFile)
	err2 := hasher.HashFile(&buf2, testFile)

	if err1 != nil || err2 != nil {
		t.Fatalf("HashFile failed: %v, %v", err1, err2)
	}

	// Should be identical
	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Error("hashing same file twice should produce identical output")
	}
}

func TestFileHasher_HashFile_DifferentContent(t *testing.T) {
	hasher := cache.NewFileHasher()

	file1 := filepath.Join("testdata", "file1.txt")
	file2 := filepath.Join("testdata", "file2.txt")

	var buf1, buf2 bytes.Buffer
	hasher.HashFile(&buf1, file1)
	hasher.HashFile(&buf2, file2)

	// Different files should have different hashes
	if bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Error("different files should produce different hashes")
	}
}

func TestFileHasher_HashDirectory(t *testing.T) {
	hasher := cache.NewFileHasher()
	var buf bytes.Buffer

	testDir := filepath.Join("testdata", "subdir")
	err := hasher.HashDirectory(&buf, testDir)
	if err != nil {
		t.Fatalf("HashDirectory failed: %v", err)
	}

	// Should have produced some output
	if buf.Len() == 0 {
		t.Error("HashDirectory should produce output for non-empty directory")
	}
}

func TestFileHasher_HashDirectory_Deterministic(t *testing.T) {
	hasher := cache.NewFileHasher()

	testDir := filepath.Join("testdata", "subdir")

	// Hash twice
	var buf1, buf2 bytes.Buffer
	hasher.HashDirectory(&buf1, testDir)
	hasher.HashDirectory(&buf2, testDir)

	// Should be identical
	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Error("hashing same directory twice should produce identical output")
	}
}

func TestFileHasher_ComputeFileDigest(t *testing.T) {
	hasher := cache.NewFileHasher()

	testFile := filepath.Join("testdata", "file1.txt")
	digest, err := hasher.ComputeFileDigest(testFile)
	if err != nil {
		t.Fatalf("ComputeFileDigest failed: %v", err)
	}

	// Should be 64-char hex string (SHA-256)
	if len(digest) != 64 {
		t.Errorf("expected 64-char hex digest, got %d chars", len(digest))
	}

	// Should be deterministic
	digest2, _ := hasher.ComputeFileDigest(testFile)
	if digest != digest2 {
		t.Error("digest should be deterministic")
	}
}

func TestFileHasher_ComputeFileDigest_DifferentFiles(t *testing.T) {
	hasher := cache.NewFileHasher()

	file1 := filepath.Join("testdata", "file1.txt")
	file2 := filepath.Join("testdata", "file2.txt")

	digest1, _ := hasher.ComputeFileDigest(file1)
	digest2, _ := hasher.ComputeFileDigest(file2)

	if digest1 == digest2 {
		t.Error("different files should have different digests")
	}
}

func TestFileHasher_HashFile_NonExistent(t *testing.T) {
	hasher := cache.NewFileHasher()
	var buf bytes.Buffer

	err := hasher.HashFile(&buf, "nonexistent.txt")
	if err == nil {
		t.Error("should error on non-existent file")
	}
}

func TestFileHasher_FileContentChange(t *testing.T) {
	hasher := cache.NewFileHasher()

	// Create temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")

	// Write initial content
	os.WriteFile(tmpFile, []byte("initial content"), 0644)
	digest1, _ := hasher.ComputeFileDigest(tmpFile)

	// Change content
	os.WriteFile(tmpFile, []byte("modified content"), 0644)
	digest2, _ := hasher.ComputeFileDigest(tmpFile)

	// Digests should be different
	if digest1 == digest2 {
		t.Error("changing file content should change digest")
	}
}
