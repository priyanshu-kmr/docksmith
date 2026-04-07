package parser_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/priyanshu/docksmith/internal/parser"
	"github.com/priyanshu/docksmith/stubs"
)

func TestParseFile_AllInstructions(t *testing.T) {
	dir := t.TempDir()
	content := strings.Join([]string{
		"FROM alpine:3.18",
		"WORKDIR /app",
		"ENV KEY=value",
		"COPY . /app",
		"RUN echo hello",
		"CMD [\"sh\",\"-c\",\"echo done\"]",
	}, "\n")

	if err := os.WriteFile(filepath.Join(dir, "Docksmithfile"), []byte(content), 0644); err != nil {
		t.Fatalf("write Docksmithfile: %v", err)
	}

	inst, err := parser.ParseFile(dir)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	if len(inst) != 6 {
		t.Fatalf("expected 6 instructions, got %d", len(inst))
	}

	if _, ok := inst[0].(*stubs.FromInstruction); !ok {
		t.Fatalf("first instruction should be FROM")
	}
	if _, ok := inst[5].(*stubs.CmdInstruction); !ok {
		t.Fatalf("last instruction should be CMD")
	}
}

func TestParseFile_UnknownInstruction(t *testing.T) {
	dir := t.TempDir()
	content := "FROM alpine\nBOOM x"
	if err := os.WriteFile(filepath.Join(dir, "Docksmithfile"), []byte(content), 0644); err != nil {
		t.Fatalf("write Docksmithfile: %v", err)
	}

	_, err := parser.ParseFile(dir)
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("error should include line number, got: %v", err)
	}
}

func TestParseFile_InvalidCMD(t *testing.T) {
	dir := t.TempDir()
	content := "FROM alpine\nCMD sh -c echo"
	if err := os.WriteFile(filepath.Join(dir, "Docksmithfile"), []byte(content), 0644); err != nil {
		t.Fatalf("write Docksmithfile: %v", err)
	}

	_, err := parser.ParseFile(dir)
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if !strings.Contains(err.Error(), "invalid CMD JSON") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseFile_RequiresFROMFirst(t *testing.T) {
	dir := t.TempDir()
	content := "RUN echo hi"
	if err := os.WriteFile(filepath.Join(dir, "Docksmithfile"), []byte(content), 0644); err != nil {
		t.Fatalf("write Docksmithfile: %v", err)
	}

	_, err := parser.ParseFile(dir)
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if !strings.Contains(err.Error(), "first instruction must be FROM") {
		t.Fatalf("unexpected error: %v", err)
	}
}
