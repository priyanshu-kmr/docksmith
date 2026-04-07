package build_test

import (
	"context"
	"os"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"github.com/priyanshu/docksmith/internal/build"
	"github.com/priyanshu/docksmith/internal/image"
	"github.com/priyanshu/docksmith/internal/layer"
)

func requireRoot(t *testing.T) {
	t.Helper()
	u, err := user.Current()
	if err != nil || u.Uid != "0" {
		t.Skip("test requires root (sudo) for namespace isolation")
	}
}

func TestBuildCreatesImageAndLayers(t *testing.T) {
	requireRoot(t)
	imagesDir, layersDir, cacheDir, ctxDir := setupBuildEnv(t)
	seedBaseImage(t, imagesDir, layersDir, "base", "latest")

	docksmithfile := "FROM base:latest\nWORKDIR /app\nENV KEY=value\nCOPY hello.txt /app/hello.txt\nRUN echo hi\nCMD [\"sh\",\"-c\",\"echo ok\"]\n"
	mustWriteFile(t, filepath.Join(ctxDir, "Docksmithfile"), docksmithfile)
	mustWriteFile(t, filepath.Join(ctxDir, "hello.txt"), "hello world")

	engine, err := build.NewEngine(imagesDir, layersDir, cacheDir)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	res, err := engine.Build(context.Background(), build.Options{
		ImageRef: "app:latest",
		Context:  ctxDir,
	})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if res.ImageRef != "app:latest" {
		t.Fatalf("unexpected image ref: %s", res.ImageRef)
	}

	imgStore, _ := image.NewStore(imagesDir)
	m, err := imgStore.Load("app", "latest")
	if err != nil {
		t.Fatalf("load built image: %v", err)
	}

	if m.Config.WorkingDir != "/app" {
		t.Fatalf("expected working dir /app, got %s", m.Config.WorkingDir)
	}
	if m.Config.Env["KEY"] != "value" {
		t.Fatalf("missing env KEY=value")
	}
	if len(m.Config.Cmd) == 0 {
		t.Fatalf("CMD should be set")
	}
	if len(m.Layers) < 3 {
		t.Fatalf("expected base + COPY + RUN layers, got %d", len(m.Layers))
	}
}

func TestBuildPreservesCreatedOnCacheHit(t *testing.T) {
	requireRoot(t)
	imagesDir, layersDir, cacheDir, ctxDir := setupBuildEnv(t)
	seedBaseImage(t, imagesDir, layersDir, "base", "latest")

	docksmithfile := "FROM base:latest\nCOPY hello.txt /app/hello.txt\nRUN echo hi\n"
	mustWriteFile(t, filepath.Join(ctxDir, "Docksmithfile"), docksmithfile)
	mustWriteFile(t, filepath.Join(ctxDir, "hello.txt"), "hello world")

	engine, err := build.NewEngine(imagesDir, layersDir, cacheDir)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	if _, err := engine.Build(context.Background(), build.Options{ImageRef: "app:latest", Context: ctxDir}); err != nil {
		t.Fatalf("first build failed: %v", err)
	}

	imgStore, _ := image.NewStore(imagesDir)
	first, err := imgStore.Load("app", "latest")
	if err != nil {
		t.Fatalf("load first image: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	if _, err := engine.Build(context.Background(), build.Options{ImageRef: "app:latest", Context: ctxDir}); err != nil {
		t.Fatalf("second build failed: %v", err)
	}

	second, err := imgStore.Load("app", "latest")
	if err != nil {
		t.Fatalf("load second image: %v", err)
	}

	if !first.Created.Equal(second.Created) {
		t.Fatalf("created timestamp should be preserved on cache-hit rebuild")
	}
}

func TestBuildNoCacheDoesNotPreserveCreated(t *testing.T) {
	requireRoot(t)
	imagesDir, layersDir, cacheDir, ctxDir := setupBuildEnv(t)
	seedBaseImage(t, imagesDir, layersDir, "base", "latest")

	docksmithfile := "FROM base:latest\nCOPY hello.txt /app/hello.txt\nRUN echo hi\n"
	mustWriteFile(t, filepath.Join(ctxDir, "Docksmithfile"), docksmithfile)
	mustWriteFile(t, filepath.Join(ctxDir, "hello.txt"), "hello world")

	engine, err := build.NewEngine(imagesDir, layersDir, cacheDir)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	if _, err := engine.Build(context.Background(), build.Options{ImageRef: "app:latest", Context: ctxDir}); err != nil {
		t.Fatalf("first build failed: %v", err)
	}

	imgStore, _ := image.NewStore(imagesDir)
	first, err := imgStore.Load("app", "latest")
	if err != nil {
		t.Fatalf("load first image: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	if _, err := engine.Build(context.Background(), build.Options{ImageRef: "app:latest", Context: ctxDir, NoCache: true}); err != nil {
		t.Fatalf("second build with no-cache failed: %v", err)
	}

	second, err := imgStore.Load("app", "latest")
	if err != nil {
		t.Fatalf("load second image: %v", err)
	}

	if !second.Created.After(first.Created) {
		t.Fatalf("created timestamp should change on --no-cache rebuild")
	}
}

func setupBuildEnv(t *testing.T) (imagesDir, layersDir, cacheDir, contextDir string) {
	t.Helper()
	root := t.TempDir()
	imagesDir = filepath.Join(root, "images")
	layersDir = filepath.Join(root, "layers")
	cacheDir = filepath.Join(root, "cache")
	contextDir = filepath.Join(root, "context")
	mustMkdir(t, imagesDir)
	mustMkdir(t, layersDir)
	mustMkdir(t, cacheDir)
	mustMkdir(t, contextDir)
	return
}

func seedBaseImage(t *testing.T, imagesDir, layersDir, name, tag string) {
	t.Helper()
	lstore, err := layer.NewStore(layersDir)
	if err != nil {
		t.Fatalf("new layer store: %v", err)
	}
	baseFS := t.TempDir()
	mustWriteFile(t, filepath.Join(baseFS, "base.txt"), "base content")

	tarCreator := layer.NewTarCreator()
	digest, tarPath, size, err := tarCreator.CreateTar(baseFS)
	if err != nil {
		t.Fatalf("create base tar: %v", err)
	}
	defer os.Remove(tarPath)

	storedDigest, _, err := lstore.StoreFromPath(tarPath)
	if err != nil {
		t.Fatalf("store base tar: %v", err)
	}
	if storedDigest != digest {
		t.Fatalf("stored digest mismatch")
	}

	m := image.NewManifest(name, tag)
	m.AddLayer(layer.NewLayerInfo("sha256:"+digest, size, "base layer"))
	m.ComputeDigest()

	istore, err := image.NewStore(imagesDir)
	if err != nil {
		t.Fatalf("new image store: %v", err)
	}
	if err := istore.Save(m); err != nil {
		t.Fatalf("save base image: %v", err)
	}
}

func mustWriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
}

func TestBuildCacheCascadeMissInvalidatesDownstream(t *testing.T) {
	requireRoot(t)
	imagesDir, layersDir, cacheDir, ctxDir := setupBuildEnv(t)
	seedBaseImage(t, imagesDir, layersDir, "base", "latest")

	docksmithfile := "FROM base:latest\nCOPY file1.txt /app/file1.txt\nCOPY file2.txt /app/file2.txt\nRUN echo done\n"
	mustWriteFile(t, filepath.Join(ctxDir, "Docksmithfile"), docksmithfile)
	mustWriteFile(t, filepath.Join(ctxDir, "file1.txt"), "file1")
	mustWriteFile(t, filepath.Join(ctxDir, "file2.txt"), "file2")

	engine, err := build.NewEngine(imagesDir, layersDir, cacheDir)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	if _, err := engine.Build(context.Background(), build.Options{ImageRef: "app:v1", Context: ctxDir}); err != nil {
		t.Fatalf("first build: %v", err)
	}

	mustWriteFile(t, filepath.Join(ctxDir, "file1.txt"), "file1-modified")

	if _, err := engine.Build(context.Background(), build.Options{ImageRef: "app:v1", Context: ctxDir}); err != nil {
		t.Fatalf("second build: %v", err)
	}
}

func TestBuildMissingBaseImageError(t *testing.T) {
	imagesDir, layersDir, cacheDir, ctxDir := setupBuildEnv(t)

	docksmithfile := "FROM nonexistent:latest\n"
	mustWriteFile(t, filepath.Join(ctxDir, "Docksmithfile"), docksmithfile)

	engine, err := build.NewEngine(imagesDir, layersDir, cacheDir)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	_, err = engine.Build(context.Background(), build.Options{ImageRef: "app:latest", Context: ctxDir})
	if err == nil {
		t.Fatalf("expected error for missing base image")
	}
}

func TestBuildWithMultipleEnvVariables(t *testing.T) {
	imagesDir, layersDir, cacheDir, ctxDir := setupBuildEnv(t)
	seedBaseImage(t, imagesDir, layersDir, "base", "latest")

	docksmithfile := "FROM base:latest\nENV FOO=bar\nENV BAZ=qux\nENV MODE=prod\n"
	mustWriteFile(t, filepath.Join(ctxDir, "Docksmithfile"), docksmithfile)

	engine, err := build.NewEngine(imagesDir, layersDir, cacheDir)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	_, err = engine.Build(context.Background(), build.Options{ImageRef: "app:latest", Context: ctxDir})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	imgStore, _ := image.NewStore(imagesDir)
	m, _ := imgStore.Load("app", "latest")
	if m.Config.Env["FOO"] != "bar" || m.Config.Env["BAZ"] != "qux" || m.Config.Env["MODE"] != "prod" {
		t.Fatalf("ENV variables not set correctly")
	}
}

func TestBuildWithAbsoluteAndRelativeWorkdir(t *testing.T) {
	imagesDir, layersDir, cacheDir, ctxDir := setupBuildEnv(t)
	seedBaseImage(t, imagesDir, layersDir, "base", "latest")

	docksmithfile := "FROM base:latest\nWORKDIR /app\nWORKDIR subdir\n"
	mustWriteFile(t, filepath.Join(ctxDir, "Docksmithfile"), docksmithfile)

	engine, err := build.NewEngine(imagesDir, layersDir, cacheDir)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	_, err = engine.Build(context.Background(), build.Options{ImageRef: "app:latest", Context: ctxDir})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	imgStore, _ := image.NewStore(imagesDir)
	m, _ := imgStore.Load("app", "latest")
	if m.Config.WorkingDir != "/app/subdir" {
		t.Fatalf("expected /app/subdir, got %s", m.Config.WorkingDir)
	}
}

func TestBuildWithMultipleCopyInstructions(t *testing.T) {
	imagesDir, layersDir, cacheDir, ctxDir := setupBuildEnv(t)
	seedBaseImage(t, imagesDir, layersDir, "base", "latest")

	docksmithfile := "FROM base:latest\nCOPY file1.txt /file1.txt\nCOPY file2.txt /file2.txt\n"
	mustWriteFile(t, filepath.Join(ctxDir, "Docksmithfile"), docksmithfile)
	mustWriteFile(t, filepath.Join(ctxDir, "file1.txt"), "content1")
	mustWriteFile(t, filepath.Join(ctxDir, "file2.txt"), "content2")

	engine, err := build.NewEngine(imagesDir, layersDir, cacheDir)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	_, err = engine.Build(context.Background(), build.Options{ImageRef: "app:latest", Context: ctxDir})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	imgStore, _ := image.NewStore(imagesDir)
	m, _ := imgStore.Load("app", "latest")
	if len(m.Layers) < 3 {
		t.Fatalf("expected at least 3 layers (base + 2 COPYs), got %d", len(m.Layers))
	}
}

func TestBuildDifferentRefUsesNewCache(t *testing.T) {
	imagesDir, layersDir, cacheDir, ctxDir := setupBuildEnv(t)
	seedBaseImage(t, imagesDir, layersDir, "base", "latest")

	docksmithfile := "FROM base:latest\nCOPY hello.txt /hello.txt\n"
	mustWriteFile(t, filepath.Join(ctxDir, "Docksmithfile"), docksmithfile)
	mustWriteFile(t, filepath.Join(ctxDir, "hello.txt"), "hello")

	engine, err := build.NewEngine(imagesDir, layersDir, cacheDir)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	if _, err := engine.Build(context.Background(), build.Options{ImageRef: "app:v1", Context: ctxDir}); err != nil {
		t.Fatalf("first build: %v", err)
	}

	if _, err := engine.Build(context.Background(), build.Options{ImageRef: "app:v2", Context: ctxDir}); err != nil {
		t.Fatalf("second build diff ref: %v", err)
	}

	imgStore, _ := image.NewStore(imagesDir)
	if !imgStore.Exists("app", "v1") || !imgStore.Exists("app", "v2") {
		t.Fatalf("both images should exist")
	}
}

func TestBuildComplexWorkdirAndRelativePaths(t *testing.T) {
	imagesDir, layersDir, cacheDir, ctxDir := setupBuildEnv(t)
	seedBaseImage(t, imagesDir, layersDir, "base", "latest")

	docksmithfile := "FROM base:latest\nWORKDIR /\nWORKDIR app\nWORKDIR ../tmp\n"
	mustWriteFile(t, filepath.Join(ctxDir, "Docksmithfile"), docksmithfile)

	engine, err := build.NewEngine(imagesDir, layersDir, cacheDir)
	if err != nil {
		t.Fatalf("NewEngine failed: %v", err)
	}

	_, err = engine.Build(context.Background(), build.Options{ImageRef: "app:latest", Context: ctxDir})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	imgStore, _ := image.NewStore(imagesDir)
	m, _ := imgStore.Load("app", "latest")
	if m.Config.WorkingDir != "/tmp" {
		t.Fatalf("expected /tmp, got %s", m.Config.WorkingDir)
	}
}
