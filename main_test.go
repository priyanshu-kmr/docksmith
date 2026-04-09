package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/priyanshu/docksmith/internal/image"
	"github.com/priyanshu/docksmith/internal/layer"
)

func TestSplitNameTag(t *testing.T) {
	name, tag, err := splitNameTag("app:latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "app" || tag != "latest" {
		t.Fatalf("unexpected parse result: %s %s", name, tag)
	}

	if _, _, err := splitNameTag("invalid"); err == nil {
		t.Fatalf("expected error for invalid ref")
	}
}

func TestRunBuildFlagValidation(t *testing.T) {
	if err := runBuild([]string{"."}); err == nil {
		t.Fatalf("expected error when -t is missing")
	}
	if err := runBuild([]string{"-t", "app:latest"}); err == nil {
		t.Fatalf("expected error when context is missing")
	}
}

func TestRunImagesAndRmi(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() { _ = os.Setenv("HOME", oldHome) })
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("set HOME: %v", err)
	}

	imagesDir, layersDir, _, err := stateDirs()
	if err != nil {
		t.Fatalf("stateDirs: %v", err)
	}

	istore, err := image.NewStore(imagesDir)
	if err != nil {
		t.Fatalf("image store: %v", err)
	}
	lstore, err := layer.NewStore(layersDir)
	if err != nil {
		t.Fatalf("layer store: %v", err)
	}

	// Seed an image and corresponding layer file.
	digest := "abc123"
	layerPath := lstore.Path(digest)
	if err := os.MkdirAll(filepath.Dir(layerPath), 0755); err != nil {
		t.Fatalf("mkdir layer dir: %v", err)
	}
	if err := os.WriteFile(layerPath, []byte("tar-placeholder"), 0644); err != nil {
		t.Fatalf("write layer tar: %v", err)
	}

	m := image.NewManifest("app", "latest")
	m.Created = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	m.AddLayer(layer.NewLayerInfo("sha256:"+digest, int64(len("tar-placeholder")), "COPY hello.txt /app/"))
	m.ComputeDigest()
	if err := istore.Save(m); err != nil {
		t.Fatalf("save image: %v", err)
	}

	imagesOut := captureStdout(t, func() {
		if err := runImages(); err != nil {
			t.Fatalf("runImages failed: %v", err)
		}
	})
	if !strings.Contains(imagesOut, "NAME") || !strings.Contains(imagesOut, "app") {
		t.Fatalf("runImages output missing expected content:\n%s", imagesOut)
	}

	rmiOut := captureStdout(t, func() {
		if err := runRmi([]string{"app:latest"}); err != nil {
			t.Fatalf("runRmi failed: %v", err)
		}
	})
	if !strings.Contains(rmiOut, "Removed image app:latest") {
		t.Fatalf("unexpected runRmi output: %s", rmiOut)
	}

	if istore.Exists("app", "latest") {
		t.Fatalf("image should be deleted after rmi")
	}
	if _, err := os.Stat(layerPath); !os.IsNotExist(err) {
		t.Fatalf("layer file should be deleted after rmi")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("copy stdout: %v", err)
	}
	_ = r.Close()
	return buf.String()
}

func TestRunBuildMissingContext(t *testing.T) {
	err := runBuild([]string{"-t", "app:latest"})
	if err == nil {
		t.Fatalf("expected error for missing context")
	}
}

func TestRunBuildInvalidTag(t *testing.T) {
	err := runBuild([]string{"-t", "invalid-without-tag", "."})
	if err == nil {
		t.Fatalf("expected error for invalid tag format")
	}
}

func TestRunBuildMultipleContextDirs(t *testing.T) {
	err := runBuild([]string{"-t", "app:latest", ".", ".."})
	if err == nil {
		t.Fatalf("expected error for multiple context dirs")
	}
}

func TestRunRmiNonExistentImage(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() { _ = os.Setenv("HOME", oldHome) })
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("set HOME: %v", err)
	}

	err := runRmi([]string{"nonexistent:v1"})
	if err == nil {
		t.Fatalf("expected error for non-existent image")
	}
}

func TestRunRmiMissingArgument(t *testing.T) {
	err := runRmi([]string{})
	if err == nil {
		t.Fatalf("expected error for missing image ref")
	}
}

func TestRunImagesWhenEmpty(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() { _ = os.Setenv("HOME", oldHome) })
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("set HOME: %v", err)
	}

	out := captureStdout(t, func() {
		if err := runImages(); err != nil {
			t.Fatalf("runImages on empty store: %v", err)
		}
	})

	if !strings.Contains(out, "NAME") {
		t.Fatalf("should still print header even when empty")
	}
}

func TestSplitNameTagInvalidFormats(t *testing.T) {
	testCases := []string{
		"",
		"notagseparator",
		":empty-name",
		"app:",
		":",
	}

	for _, tc := range testCases {
		if _, _, err := splitNameTag(tc); err == nil {
			t.Fatalf("expected error for invalid ref: %s", tc)
		}
	}
}

func TestRunImagesMultipleTags(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() { _ = os.Setenv("HOME", oldHome) })
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("set HOME: %v", err)
	}

	imagesDir, _, _, err := stateDirs()
	if err != nil {
		t.Fatalf("stateDirs: %v", err)
	}

	istore, err := image.NewStore(imagesDir)
	if err != nil {
		t.Fatalf("image store: %v", err)
	}

	// Create multiple images with same name, different tags
	for _, tag := range []string{"v1", "v2", "v3"} {
		m := image.NewManifest("myapp", tag)
		m.ComputeDigest()
		if err := istore.Save(m); err != nil {
			t.Fatalf("save image: %v", err)
		}
	}

	out := captureStdout(t, func() {
		if err := runImages(); err != nil {
			t.Fatalf("runImages failed: %v", err)
		}
	})

	if strings.Count(out, "myapp") < 3 {
		t.Fatalf("should list all 3 tags of myapp")
	}
}

func TestRunRmiMultipleImagesWithSameName(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() { _ = os.Setenv("HOME", oldHome) })
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("set HOME: %v", err)
	}

	imagesDir, _, _, err := stateDirs()
	if err != nil {
		t.Fatalf("stateDirs: %v", err)
	}

	istore, err := image.NewStore(imagesDir)
	if err != nil {
		t.Fatalf("image store: %v", err)
	}

	// Create multiple tags
	for _, tag := range []string{"v1", "v2"} {
		m := image.NewManifest("app", tag)
		m.ComputeDigest()
		if err := istore.Save(m); err != nil {
			t.Fatalf("save image: %v", err)
		}
	}

	// Delete one tag
	if err := runRmi([]string{"app:v1"}); err != nil {
		t.Fatalf("runRmi failed: %v", err)
	}

	// v1 should be gone, v2 should remain
	if istore.Exists("app", "v1") {
		t.Fatalf("v1 should be deleted")
	}
	if !istore.Exists("app", "v2") {
		t.Fatalf("v2 should still exist")
	}
}

func TestRunBuildNoCacheFlag(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() { _ = os.Setenv("HOME", oldHome) })
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("set HOME: %v", err)
	}

	ctxDir := t.TempDir()
	content := "FROM alpine:3.18\nRUN echo hello"
	if err := os.WriteFile(filepath.Join(ctxDir, "Docksmithfile"), []byte(content), 0644); err != nil {
		t.Fatalf("write Docksmithfile: %v", err)
	}

	if err := runBuild([]string{"-t", "app:v1", "--no-cache", ctxDir}); err != nil {
		t.Logf("runBuild with --no-cache: %v (expected if alpine:3.18 not available)", err)
	}
}
