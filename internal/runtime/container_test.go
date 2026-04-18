package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/priyanshu/docksmith/internal/image"
	"github.com/priyanshu/docksmith/internal/layer"
)

func TestDeterministicContainerID(t *testing.T) {
	a := deterministicContainerID("app:latest")
	b := deterministicContainerID("app:latest")
	c := deterministicContainerID("app:v2")

	if a != b {
		t.Fatalf("same image ref should generate same ID")
	}
	if a == c {
		t.Fatalf("different image refs should generate different IDs")
	}
	if len(a) != 8 {
		t.Fatalf("expected 8-char container ID, got %q", a)
	}
}

func TestSortedLayerDigestsForOverlay(t *testing.T) {
	in := []string{"A", "B", "C"}
	out := sortedLayerDigestsForOverlay(in)

	expected := []string{"C", "B", "A"}
	for i := range expected {
		if out[i] != expected[i] {
			t.Fatalf("unexpected overlay order: got %v want %v", out, expected)
		}
	}
}

func TestEnsureContainerConfigCreatesAndReusesConfig(t *testing.T) {
	tmp := t.TempDir()
	m := image.NewManifest("myapp", "latest")
	m.AddLayer(layer.NewLayerInfo("sha256:aaa", 10, "COPY a /a"))
	m.AddLayer(layer.NewLayerInfo("sha256:bbb", 20, "COPY b /b"))

	c := &Container{
		manifest:      m,
		containersDir: filepath.Join(tmp, "containers"),
		layerFSDir:    filepath.Join(tmp, "layerfs"),
		envOverrides:  map[string]string{},
	}

	id := deterministicContainerID(m.Reference())
	containerPath := filepath.Join(c.containersDir, id)

	cfg1, err := c.ensureContainerConfig(containerPath, id, []string{"echo", "hello"})
	if err != nil {
		t.Fatalf("ensureContainerConfig create: %v", err)
	}
	if cfg1.ID != id {
		t.Fatalf("unexpected id: %s", cfg1.ID)
	}
	if cfg1.Image != "myapp:latest" {
		t.Fatalf("unexpected image ref: %s", cfg1.Image)
	}
	if len(cfg1.Layers) != 2 || cfg1.Layers[0] != "sha256:aaa" || cfg1.Layers[1] != "sha256:bbb" {
		t.Fatalf("unexpected layers: %v", cfg1.Layers)
	}

	configPath := filepath.Join(containerPath, "config.json")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var diskCfg containerConfig
	if err := json.Unmarshal(raw, &diskCfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if diskCfg.ID != id {
		t.Fatalf("unexpected disk id: %s", diskCfg.ID)
	}

	cfg2, err := c.ensureContainerConfig(containerPath, id, []string{"ignored"})
	if err != nil {
		t.Fatalf("ensureContainerConfig reload: %v", err)
	}
	if cfg2.Created != cfg1.Created {
		t.Fatalf("existing config should be reused")
	}
}
