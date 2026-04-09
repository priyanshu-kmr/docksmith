package cache_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/priyanshu/docksmith/internal/cache"
	"github.com/priyanshu/docksmith/stubs"
)

func TestComputeKey_Deterministic(t *testing.T) {
	keygen := cache.NewKeyGenerator()
	ctx := cache.NewBuildContext("/build")

	inst := &stubs.RunInstruction{
		Command: "echo hello",
		Raw:     "RUN echo hello",
	}

	key1, err1 := keygen.ComputeKey("sha256:base", inst, ctx)
	key2, err2 := keygen.ComputeKey("sha256:base", inst, ctx)

	if err1 != nil || err2 != nil {
		t.Fatalf("ComputeKey failed: %v, %v", err1, err2)
	}

	if key1 != key2 {
		t.Error("same inputs should produce same cache key")
	}

	// Cache key should be 64-char hex string (SHA-256)
	if len(key1) != 64 {
		t.Errorf("expected 64-char cache key, got %d chars", len(key1))
	}
}

func TestComputeKey_FromInstruction(t *testing.T) {
	keygen := cache.NewKeyGenerator()
	ctx := cache.NewBuildContext("/build")

	inst := &stubs.FromInstruction{
		Image: "ubuntu",
		Tag:   "20.04",
		Raw:   "FROM ubuntu:20.04",
	}

	key, err := keygen.ComputeKey("", inst, ctx)
	if err != nil {
		t.Fatalf("ComputeKey failed: %v", err)
	}

	if len(key) != 64 {
		t.Errorf("expected 64-char cache key, got %d", len(key))
	}
}

func TestComputeKey_RunInstruction(t *testing.T) {
	keygen := cache.NewKeyGenerator()
	ctx := cache.NewBuildContext("/build")

	ctx.ApplyEnv("FOO", "bar")
	ctx.ApplyWorkdir("/app")

	inst := &stubs.RunInstruction{
		Command: "echo $FOO",
		Raw:     "RUN echo $FOO",
	}

	key, err := keygen.ComputeKey("sha256:prev", inst, ctx)
	if err != nil {
		t.Fatalf("ComputeKey failed: %v", err)
	}

	// Different ENV should produce different key
	ctx2 := cache.NewBuildContext("/build")
	ctx2.ApplyEnv("FOO", "different")
	ctx2.ApplyWorkdir("/app")

	key2, _ := keygen.ComputeKey("sha256:prev", inst, ctx2)

	if key == key2 {
		t.Error("different ENV values should produce different cache keys")
	}
}

func TestComputeKey_WorkdirInstruction(t *testing.T) {
	keygen := cache.NewKeyGenerator()
	ctx := cache.NewBuildContext("/build")

	inst := &stubs.WorkdirInstruction{
		Path: "/app",
		Raw:  "WORKDIR /app",
	}

	key, err := keygen.ComputeKey("sha256:prev", inst, ctx)
	if err != nil {
		t.Fatalf("ComputeKey failed: %v", err)
	}

	if len(key) != 64 {
		t.Errorf("expected 64-char cache key, got %d", len(key))
	}
}

func TestComputeKey_EnvInstruction(t *testing.T) {
	keygen := cache.NewKeyGenerator()
	ctx := cache.NewBuildContext("/build")

	inst := &stubs.EnvInstruction{
		Key:   "FOO",
		Value: "bar",
		Raw:   "ENV FOO=bar",
	}

	key, err := keygen.ComputeKey("sha256:prev", inst, ctx)
	if err != nil {
		t.Fatalf("ComputeKey failed: %v", err)
	}

	if len(key) != 64 {
		t.Errorf("expected 64-char cache key, got %d", len(key))
	}
}

func TestComputeKey_CmdInstruction(t *testing.T) {
	keygen := cache.NewKeyGenerator()
	ctx := cache.NewBuildContext("/build")

	inst := &stubs.CmdInstruction{
		Command: []string{"echo", "hello"},
		Raw:     "CMD [\"echo\", \"hello\"]",
	}

	key, err := keygen.ComputeKey("sha256:prev", inst, ctx)
	if err != nil {
		t.Fatalf("ComputeKey failed: %v", err)
	}

	if len(key) != 64 {
		t.Errorf("expected 64-char cache key, got %d", len(key))
	}
}

func TestComputeKey_CopyInstruction(t *testing.T) {
	keygen := cache.NewKeyGenerator()

	// Use testdata directory for COPY testing
	testdataDir, _ := filepath.Abs("testdata")
	ctx := cache.NewBuildContext(testdataDir)

	inst := &stubs.CopyInstruction{
		Sources: []string{"file1.txt"},
		Dest:    "/app/",
		Raw:     "COPY file1.txt /app/",
	}

	key, err := keygen.ComputeKey("sha256:prev", inst, ctx)
	if err != nil {
		t.Fatalf("ComputeKey failed: %v", err)
	}

	if len(key) != 64 {
		t.Errorf("expected 64-char cache key, got %d", len(key))
	}
}

func TestComputeKey_CopyInstruction_FileContentChange(t *testing.T) {
	keygen := cache.NewKeyGenerator()

	// Create temporary directory with test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	os.WriteFile(testFile, []byte("initial content"), 0644)

	ctx := cache.NewBuildContext(tmpDir)
	inst := &stubs.CopyInstruction{
		Sources: []string{"test.txt"},
		Dest:    "/app/",
		Raw:     "COPY test.txt /app/",
	}

	key1, _ := keygen.ComputeKey("sha256:prev", inst, ctx)

	// Change file content
	os.WriteFile(testFile, []byte("modified content"), 0644)

	key2, _ := keygen.ComputeKey("sha256:prev", inst, ctx)

	if key1 == key2 {
		t.Error("changing COPY source content should change cache key")
	}
}

func TestComputeKey_StateChaining(t *testing.T) {
	keygen := cache.NewKeyGenerator()
	ctx := cache.NewBuildContext("/build")

	inst := &stubs.RunInstruction{
		Command: "echo hello",
		Raw:     "RUN echo hello",
	}

	// Same instruction with different previous digest
	key1, _ := keygen.ComputeKey("sha256:digest1", inst, ctx)
	key2, _ := keygen.ComputeKey("sha256:digest2", inst, ctx)

	if key1 == key2 {
		t.Error("different previous digest should produce different cache key")
	}
}

func TestComputeKey_DifferentInstructions(t *testing.T) {
	keygen := cache.NewKeyGenerator()
	ctx := cache.NewBuildContext("/build")

	inst1 := &stubs.RunInstruction{
		Command: "echo hello",
		Raw:     "RUN echo hello",
	}

	inst2 := &stubs.RunInstruction{
		Command: "echo world",
		Raw:     "RUN echo world",
	}

	key1, _ := keygen.ComputeKey("sha256:base", inst1, ctx)
	key2, _ := keygen.ComputeKey("sha256:base", inst2, ctx)

	if key1 == key2 {
		t.Error("different instructions should produce different cache keys")
	}
}

func TestComputeKey_RunWithWorkdir(t *testing.T) {
	keygen := cache.NewKeyGenerator()

	ctx1 := cache.NewBuildContext("/build")
	ctx1.ApplyWorkdir("/app")

	ctx2 := cache.NewBuildContext("/build")
	ctx2.ApplyWorkdir("/different")

	inst := &stubs.RunInstruction{
		Command: "pwd",
		Raw:     "RUN pwd",
	}

	key1, _ := keygen.ComputeKey("sha256:base", inst, ctx1)
	key2, _ := keygen.ComputeKey("sha256:base", inst, ctx2)

	if key1 == key2 {
		t.Error("different WORKDIR should produce different cache keys for RUN")
	}
}

func TestComputeKey_CopyMultipleSources(t *testing.T) {
	keygen := cache.NewKeyGenerator()

	testdataDir, _ := filepath.Abs("testdata")
	ctx := cache.NewBuildContext(testdataDir)

	inst := &stubs.CopyInstruction{
		Sources: []string{"file1.txt", "file2.txt"},
		Dest:    "/app/",
		Raw:     "COPY file1.txt file2.txt /app/",
	}

	key, err := keygen.ComputeKey("sha256:prev", inst, ctx)
	if err != nil {
		t.Fatalf("ComputeKey failed: %v", err)
	}

	// Should handle multiple sources
	if len(key) != 64 {
		t.Errorf("expected 64-char cache key, got %d", len(key))
	}
}

func TestComputeKey_CopyDirectory(t *testing.T) {
	keygen := cache.NewKeyGenerator()

	testdataDir, _ := filepath.Abs("testdata")
	ctx := cache.NewBuildContext(testdataDir)

	inst := &stubs.CopyInstruction{
		Sources: []string{"subdir"},
		Dest:    "/app/",
		Raw:     "COPY subdir /app/",
	}

	key, err := keygen.ComputeKey("sha256:prev", inst, ctx)
	if err != nil {
		t.Fatalf("ComputeKey failed: %v", err)
	}

	if len(key) != 64 {
		t.Errorf("expected 64-char cache key, got %d", len(key))
	}
}
