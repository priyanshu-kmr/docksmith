package main

import (
	"bytes"
	"encoding/json"
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

	imagesDir, layersDir, _, _, _, err := stateDirs()
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

	imagesDir, _, _, _, _, err := stateDirs()
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

	imagesDir, _, _, _, _, err := stateDirs()
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

func TestRunPsFiltersRunningAndAll(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() { _ = os.Setenv("HOME", oldHome) })
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("set HOME: %v", err)
	}

	_, _, _, containersDir, _, err := stateDirs()
	if err != nil {
		t.Fatalf("stateDirs: %v", err)
	}

	now := time.Now().UTC()
	started := now.Add(-2 * time.Minute)
	finished := now.Add(-1 * time.Minute)
	exitCode := 7

	runningCfg := map[string]any{
		"id":        "run12345",
		"name":      "ctr-run12345",
		"image":     "app:latest",
		"command":   []string{"sleep", "60"},
		"created":   now.Add(-3 * time.Minute).Format(time.RFC3339Nano),
		"startedAt": started.Format(time.RFC3339Nano),
		"pid":       os.Getpid(),
		"layers":    []string{"sha256:aaa"},
	}
	exitedCfg := map[string]any{
		"id":         "exi12345",
		"name":       "ctr-exi12345",
		"image":      "app:v2",
		"command":    []string{"sh", "-c", "echo hi"},
		"created":    now.Add(-5 * time.Minute).Format(time.RFC3339Nano),
		"startedAt":  now.Add(-4 * time.Minute).Format(time.RFC3339Nano),
		"finishedAt": finished.Format(time.RFC3339Nano),
		"exitCode":   exitCode,
		"layers":     []string{"sha256:bbb"},
	}

	writeContainerConfigJSON(t, containersDir, "run12345", runningCfg)
	writeContainerConfigJSON(t, containersDir, "exi12345", exitedCfg)

	outRunning := captureStdout(t, func() {
		if err := runPs([]string{}); err != nil {
			t.Fatalf("runPs: %v", err)
		}
	})
	if !strings.Contains(outRunning, "ID") || !strings.Contains(outRunning, "run12345") {
		t.Fatalf("expected running container in ps output:\n%s", outRunning)
	}
	if strings.Contains(outRunning, "exi12345") {
		t.Fatalf("did not expect exited container without -a:\n%s", outRunning)
	}

	outAll := captureStdout(t, func() {
		if err := runPs([]string{"-a"}); err != nil {
			t.Fatalf("runPs -a: %v", err)
		}
	})
	if !strings.Contains(outAll, "run12345") || !strings.Contains(outAll, "exi12345") {
		t.Fatalf("expected both containers with -a:\n%s", outAll)
	}
	if !strings.Contains(outAll, "exited(7)") {
		t.Fatalf("expected exited status with code:\n%s", outAll)
	}
}

func TestRunStartValidatesArguments(t *testing.T) {
	if _, err := runStart([]string{}); err == nil {
		t.Fatalf("expected error for missing start argument")
	}
	if _, err := runStart([]string{"one", "two"}); err == nil {
		t.Fatalf("expected error for too many start arguments")
	}
}

func TestLoadContainerConfigByIdentifier(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() { _ = os.Setenv("HOME", oldHome) })
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("set HOME: %v", err)
	}

	_, _, _, containersDir, _, err := stateDirs()
	if err != nil {
		t.Fatalf("stateDirs: %v", err)
	}

	writeContainerConfigJSON(t, containersDir, "abc12345", map[string]any{
		"id":      "abc12345",
		"name":    "ctr-abc12345",
		"image":   "demo:v1",
		"command": []string{"sh"},
		"layers":  []string{"sha256:aaa"},
	})

	cfgByID, err := loadContainerConfigByIdentifier(containersDir, "abc12345")
	if err != nil {
		t.Fatalf("lookup by id failed: %v", err)
	}
	if cfgByID.Image != "demo:v1" {
		t.Fatalf("unexpected image: %s", cfgByID.Image)
	}

	cfgByName, err := loadContainerConfigByIdentifier(containersDir, "ctr-abc12345")
	if err != nil {
		t.Fatalf("lookup by name failed: %v", err)
	}
	if cfgByName.ID != "abc12345" {
		t.Fatalf("unexpected id from name lookup: %s", cfgByName.ID)
	}
}

func TestRunStartFailsWhenContainerDoesNotExist(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() { _ = os.Setenv("HOME", oldHome) })
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("set HOME: %v", err)
	}

	if _, err := runStart([]string{"missing"}); err == nil {
		t.Fatalf("expected container not found error")
	}
}

func TestRunStartFailsOnInvalidContainerImageRef(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() { _ = os.Setenv("HOME", oldHome) })
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("set HOME: %v", err)
	}

	_, _, _, containersDir, _, err := stateDirs()
	if err != nil {
		t.Fatalf("stateDirs: %v", err)
	}

	writeContainerConfigJSON(t, containersDir, "abc12345", map[string]any{
		"id":      "abc12345",
		"name":    "ctr-abc12345",
		"image":   "invalid-image-ref",
		"command": []string{"sh"},
		"layers":  []string{"sha256:aaa"},
	})

	if _, err := runStart([]string{"abc12345"}); err == nil {
		t.Fatalf("expected invalid image ref error")
	}
}

func TestValidateContainerName(t *testing.T) {
	valid := []string{"web", "web-1", "web_1", "web.1", "A1"}
	for _, name := range valid {
		if err := validateContainerName(name); err != nil {
			t.Fatalf("expected valid container name %q: %v", name, err)
		}
	}
	invalid := []string{"", "-bad", ".bad", "_bad", "bad name", "bad/name"}
	for _, name := range invalid {
		if err := validateContainerName(name); err == nil {
			t.Fatalf("expected invalid container name %q", name)
		}
	}
}

func TestRunRunRejectsInvalidContainerName(t *testing.T) {
	if _, err := runRun([]string{"--name", "bad name", "app:latest"}); err == nil {
		t.Fatalf("expected invalid --name error")
	}
}

func TestRunRmByIDAndName(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() { _ = os.Setenv("HOME", oldHome) })
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("set HOME: %v", err)
	}

	_, _, _, containersDir, _, err := stateDirs()
	if err != nil {
		t.Fatalf("stateDirs: %v", err)
	}

	writeContainerConfigJSON(t, containersDir, "abc12345", map[string]any{
		"id":      "abc12345",
		"name":    "web",
		"image":   "demo:v1",
		"command": []string{"sh"},
		"layers":  []string{"sha256:aaa"},
	})
	writeContainerConfigJSON(t, containersDir, "def67890", map[string]any{
		"id":      "def67890",
		"name":    "api",
		"image":   "demo:v2",
		"command": []string{"sh"},
		"layers":  []string{"sha256:bbb"},
	})

	if err := runRm([]string{"abc12345"}); err != nil {
		t.Fatalf("runRm by id: %v", err)
	}
	if _, err := os.Stat(filepath.Join(containersDir, "abc12345")); !os.IsNotExist(err) {
		t.Fatalf("expected abc12345 dir removed")
	}

	if err := runRm([]string{"api"}); err != nil {
		t.Fatalf("runRm by name: %v", err)
	}
	if _, err := os.Stat(filepath.Join(containersDir, "def67890")); !os.IsNotExist(err) {
		t.Fatalf("expected def67890 dir removed")
	}
}

func TestRunRmFailsOnRunningAndMissingSelectors(t *testing.T) {
	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() { _ = os.Setenv("HOME", oldHome) })
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("set HOME: %v", err)
	}

	_, _, _, containersDir, _, err := stateDirs()
	if err != nil {
		t.Fatalf("stateDirs: %v", err)
	}

	writeContainerConfigJSON(t, containersDir, "run12345", map[string]any{
		"id":        "run12345",
		"name":      "running",
		"image":     "demo:v1",
		"command":   []string{"sleep", "60"},
		"created":   time.Now().UTC().Format(time.RFC3339Nano),
		"startedAt": time.Now().UTC().Add(-1 * time.Minute).Format(time.RFC3339Nano),
		"pid":       os.Getpid(),
		"layers":    []string{"sha256:aaa"},
	})

	if err := runRm([]string{"run12345"}); err == nil {
		t.Fatalf("expected error when removing running container")
	}
	if err := runRm([]string{"demo"}); err == nil {
		t.Fatalf("expected error for non-container selector")
	}
	if err := runRm([]string{"missing-selector"}); err == nil {
		t.Fatalf("expected error for missing selector")
	}
}

func writeContainerConfigJSON(t *testing.T, containersDir, id string, cfg map[string]any) {
	t.Helper()
	dir := filepath.Join(containersDir, id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir container dir: %v", err)
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), raw, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}
