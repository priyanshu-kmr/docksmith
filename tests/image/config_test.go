package image_test

import (
	"testing"

	"github.com/priyanshu/docksmith/internal/image"
)

func TestConfig_NewConfig(t *testing.T) {
	c := image.NewConfig()

	if c.Env == nil {
		t.Error("env should not be nil")
	}
	if len(c.Env) != 0 {
		t.Error("env should be empty")
	}
	if c.Cmd != nil {
		t.Error("cmd should be nil initially")
	}
	if c.WorkingDir != "/" {
		t.Errorf("workingDir should default to '/', got '%s'", c.WorkingDir)
	}
}

func TestConfig_SetEnv(t *testing.T) {
	c := image.NewConfig()

	c.SetEnv("FOO", "bar")
	c.SetEnv("BAZ", "qux")

	if c.Env["FOO"] != "bar" {
		t.Errorf("expected FOO=bar, got FOO=%s", c.Env["FOO"])
	}
	if c.Env["BAZ"] != "qux" {
		t.Errorf("expected BAZ=qux, got BAZ=%s", c.Env["BAZ"])
	}
}

func TestConfig_SetCmd(t *testing.T) {
	c := image.NewConfig()

	c.SetCmd([]string{"echo", "hello", "world"})

	if len(c.Cmd) != 3 {
		t.Errorf("expected 3 cmd args, got %d", len(c.Cmd))
	}
	if c.Cmd[0] != "echo" || c.Cmd[1] != "hello" || c.Cmd[2] != "world" {
		t.Error("cmd args mismatch")
	}
}

func TestConfig_SetWorkingDir(t *testing.T) {
	c := image.NewConfig()

	c.SetWorkingDir("/app")

	if c.WorkingDir != "/app" {
		t.Errorf("expected /app, got %s", c.WorkingDir)
	}
}

func TestConfig_Clone(t *testing.T) {
	c := image.NewConfig()
	c.SetEnv("FOO", "bar")
	c.SetCmd([]string{"echo", "hello"})
	c.SetWorkingDir("/app")

	clone := c.Clone()

	// Verify values match
	if clone.Env["FOO"] != "bar" {
		t.Error("clone env mismatch")
	}
	if len(clone.Cmd) != 2 || clone.Cmd[0] != "echo" {
		t.Error("clone cmd mismatch")
	}
	if clone.WorkingDir != "/app" {
		t.Error("clone workingDir mismatch")
	}

	// Modify clone - should not affect original
	clone.SetEnv("FOO", "modified")
	clone.Cmd[0] = "modified"

	if c.Env["FOO"] != "bar" {
		t.Error("modifying clone env should not affect original")
	}
	if c.Cmd[0] != "echo" {
		t.Error("modifying clone cmd should not affect original")
	}
}

func TestConfig_Hash_Deterministic(t *testing.T) {
	c1 := image.NewConfig()
	c1.SetEnv("FOO", "bar")
	c1.SetEnv("BAZ", "qux")
	c1.SetCmd([]string{"echo", "hello"})
	c1.SetWorkingDir("/app")

	c2 := image.NewConfig()
	c2.SetEnv("BAZ", "qux") // Different order
	c2.SetEnv("FOO", "bar")
	c2.SetCmd([]string{"echo", "hello"})
	c2.SetWorkingDir("/app")

	hash1 := c1.Hash()
	hash2 := c2.Hash()

	// Should be same despite different env insertion order
	if hash1 != hash2 {
		t.Errorf("config hash should be deterministic: %s vs %s", hash1, hash2)
	}
}

func TestConfig_Hash_DifferentValues(t *testing.T) {
	c1 := image.NewConfig()
	c1.SetEnv("FOO", "bar")

	c2 := image.NewConfig()
	c2.SetEnv("FOO", "different")

	hash1 := c1.Hash()
	hash2 := c2.Hash()

	if hash1 == hash2 {
		t.Error("different config values should produce different hash")
	}
}
