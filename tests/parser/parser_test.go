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

func TestParseFile_WithCommentsAndBlankLines(t *testing.T) {
	dir := t.TempDir()
	content := strings.Join([]string{
		"# This is a comment",
		"FROM alpine:3.18",
		"",
		"# Build stage",
		"WORKDIR /app",
		"COPY . /app",
		"",
		"# Install and run",
		"RUN echo done",
	}, "\n")

	if err := os.WriteFile(filepath.Join(dir, "Docksmithfile"), []byte(content), 0644); err != nil {
		t.Fatalf("write Docksmithfile: %v", err)
	}

	inst, err := parser.ParseFile(dir)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(inst) != 4 {
		t.Fatalf("expected 4 instructions (comments and blanks ignored), got %d", len(inst))
	}
}

func TestParseFile_MultipleEnvInstructions(t *testing.T) {
	dir := t.TempDir()
	content := strings.Join([]string{
		"FROM alpine",
		"ENV KEY1=value1",
		"ENV KEY2=value2",
		"ENV KEY3=value3",
	}, "\n")

	if err := os.WriteFile(filepath.Join(dir, "Docksmithfile"), []byte(content), 0644); err != nil {
		t.Fatalf("write Docksmithfile: %v", err)
	}

	inst, err := parser.ParseFile(dir)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(inst) != 4 {
		t.Fatalf("expected 4 instructions, got %d", len(inst))
	}

	for i := 1; i <= 3; i++ {
		if _, ok := inst[i].(*stubs.EnvInstruction); !ok {
			t.Fatalf("instruction %d should be ENV", i)
		}
	}
}

func TestParseFile_InvalidEnvMissingValue(t *testing.T) {
	dir := t.TempDir()
	content := "FROM alpine\nENV NOVALUE"
	if err := os.WriteFile(filepath.Join(dir, "Docksmithfile"), []byte(content), 0644); err != nil {
		t.Fatalf("write Docksmithfile: %v", err)
	}

	_, err := parser.ParseFile(dir)
	if err == nil {
		t.Fatalf("expected error for invalid ENV")
	}
}

func TestParseFile_CopyMultipleSources(t *testing.T) {
	dir := t.TempDir()
	content := "FROM alpine\nCOPY file1.txt file2.txt file3.txt /app/"
	if err := os.WriteFile(filepath.Join(dir, "Docksmithfile"), []byte(content), 0644); err != nil {
		t.Fatalf("write Docksmithfile: %v", err)
	}

	inst, err := parser.ParseFile(dir)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	copyInst := inst[1].(*stubs.CopyInstruction)
	if len(copyInst.Sources) != 3 {
		t.Fatalf("expected 3 sources, got %d", len(copyInst.Sources))
	}
}

func TestParseFile_InvalidCMDNotArray(t *testing.T) {
	dir := t.TempDir()
	content := "FROM alpine\nCMD {\"invalid\": \"json\"}"
	if err := os.WriteFile(filepath.Join(dir, "Docksmithfile"), []byte(content), 0644); err != nil {
		t.Fatalf("write Docksmithfile: %v", err)
	}

	_, err := parser.ParseFile(dir)
	if err == nil {
		t.Fatalf("expected error for non-array CMD")
	}
}

func TestParseFile_EmptyCMDArray(t *testing.T) {
	dir := t.TempDir()
	content := "FROM alpine\nCMD []"
	if err := os.WriteFile(filepath.Join(dir, "Docksmithfile"), []byte(content), 0644); err != nil {
		t.Fatalf("write Docksmithfile: %v", err)
	}

	_, err := parser.ParseFile(dir)
	if err == nil {
		t.Fatalf("expected error for empty CMD array")
	}
}

func TestParseFile_ImageWithoutTag(t *testing.T) {
	dir := t.TempDir()
	content := "FROM alpine"
	if err := os.WriteFile(filepath.Join(dir, "Docksmithfile"), []byte(content), 0644); err != nil {
		t.Fatalf("write Docksmithfile: %v", err)
	}

	inst, err := parser.ParseFile(dir)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	fromInst := inst[0].(*stubs.FromInstruction)
	if fromInst.Image != "alpine" || fromInst.Tag != "latest" {
		t.Fatalf("should default tag to 'latest', got %s", fromInst.Tag)
	}
}

func TestParseFile_WorkdirRelativePaths(t *testing.T) {
	dir := t.TempDir()
	content := strings.Join([]string{
		"FROM alpine",
		"WORKDIR /app",
		"WORKDIR subdir",
		"WORKDIR ../../other",
	}, "\n")

	if err := os.WriteFile(filepath.Join(dir, "Docksmithfile"), []byte(content), 0644); err != nil {
		t.Fatalf("write Docksmithfile: %v", err)
	}

	inst, err := parser.ParseFile(dir)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(inst) != 4 {
		t.Fatalf("expected 4 instructions, got %d", len(inst))
	}
}

func TestParseFile_EnvWithSpecialChars(t *testing.T) {
	dir := t.TempDir()
	content := "FROM alpine\nENV DB_URL=postgresql://user:pass@host:5432/db\nENV PATH_VAR=/usr/bin:/usr/local/bin"
	if err := os.WriteFile(filepath.Join(dir, "Docksmithfile"), []byte(content), 0644); err != nil {
		t.Fatalf("write Docksmithfile: %v", err)
	}

	inst, err := parser.ParseFile(dir)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	if len(inst) != 3 {
		t.Fatalf("expected 3 instructions, got %d", len(inst))
	}
}

func TestParseFile_CopyWithGlobPattern(t *testing.T) {
	dir := t.TempDir()
	content := "FROM alpine\nCOPY *.txt /app/"
	if err := os.WriteFile(filepath.Join(dir, "Docksmithfile"), []byte(content), 0644); err != nil {
		t.Fatalf("write Docksmithfile: %v", err)
	}

	inst, err := parser.ParseFile(dir)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}

	copyInst := inst[1].(*stubs.CopyInstruction)
	if len(copyInst.Sources) != 1 || copyInst.Sources[0] != "*.txt" {
		t.Fatalf("should preserve glob pattern")
	}
}
