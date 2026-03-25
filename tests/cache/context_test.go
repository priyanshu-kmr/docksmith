package cache_test

import (
	"testing"

	"github.com/priyanshu/docksmith/internal/cache"
)

func TestBuildContext_ApplyEnv(t *testing.T) {
	ctx := cache.NewBuildContext("/build")

	ctx.ApplyEnv("FOO", "bar")
	ctx.ApplyEnv("BAZ", "qux")

	if ctx.Env["FOO"] != "bar" {
		t.Errorf("expected FOO=bar, got FOO=%s", ctx.Env["FOO"])
	}

	if ctx.Env["BAZ"] != "qux" {
		t.Errorf("expected BAZ=qux, got BAZ=%s", ctx.Env["BAZ"])
	}
}

func TestBuildContext_ApplyWorkdir_Absolute(t *testing.T) {
	ctx := cache.NewBuildContext("/build")

	ctx.ApplyWorkdir("/app")

	if ctx.Workdir != "/app" {
		t.Errorf("expected workdir /app, got %s", ctx.Workdir)
	}
}

func TestBuildContext_ApplyWorkdir_Relative(t *testing.T) {
	ctx := cache.NewBuildContext("/build")
	ctx.Workdir = "/app"

	ctx.ApplyWorkdir("src")

	if ctx.Workdir != "/app/src" {
		t.Errorf("expected workdir /app/src, got %s", ctx.Workdir)
	}
}

func TestBuildContext_SerializeEnv(t *testing.T) {
	ctx := cache.NewBuildContext("/build")

	// Add env vars in non-alphabetical order
	ctx.ApplyEnv("ZZZ", "last")
	ctx.ApplyEnv("AAA", "first")
	ctx.ApplyEnv("MMM", "middle")

	serialized := ctx.SerializeEnv()

	// Should be sorted alphabetically
	expected := "AAA=first\nMMM=middle\nZZZ=last\n"
	if serialized != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, serialized)
	}
}

func TestBuildContext_SerializeEnv_Empty(t *testing.T) {
	ctx := cache.NewBuildContext("/build")

	serialized := ctx.SerializeEnv()

	if serialized != "" {
		t.Errorf("expected empty string for no env vars, got %s", serialized)
	}
}

func TestBuildContext_Clone(t *testing.T) {
	ctx := cache.NewBuildContext("/build")
	ctx.ApplyEnv("FOO", "bar")
	ctx.ApplyWorkdir("/app")

	clone := ctx.Clone()

	// Verify clone has same values
	if clone.Env["FOO"] != "bar" {
		t.Errorf("clone should have FOO=bar")
	}
	if clone.Workdir != "/app" {
		t.Errorf("clone should have workdir /app")
	}
	if clone.ContextDir != "/build" {
		t.Errorf("clone should have context dir /build")
	}

	// Modify clone - should not affect original
	clone.ApplyEnv("FOO", "modified")
	clone.ApplyWorkdir("/changed")

	if ctx.Env["FOO"] != "bar" {
		t.Errorf("modifying clone should not affect original")
	}
	if ctx.Workdir != "/app" {
		t.Errorf("modifying clone workdir should not affect original")
	}
}
