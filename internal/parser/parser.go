package parser

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/priyanshu/docksmith/stubs"
)

// ParseError provides a line-aware parse error.
type ParseError struct {
	Line int
	Msg  string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

// ParseFile parses a Docksmithfile from disk.
func ParseFile(contextDir string) ([]stubs.Instruction, error) {
	filePath := filepath.Join(contextDir, "Docksmithfile")
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open Docksmithfile: %w", err)
	}
	defer f.Close()

	return ParseReader(f)
}

// ParseReader parses Docksmithfile content from a reader.
func ParseReader(r *os.File) ([]stubs.Instruction, error) {
	scanner := bufio.NewScanner(r)
	instructions := make([]stubs.Instruction, 0)

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		inst, err := parseLine(lineNo, line)
		if err != nil {
			return nil, err
		}
		instructions = append(instructions, inst)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read Docksmithfile: %w", err)
	}

	if len(instructions) == 0 {
		return nil, &ParseError{Line: 1, Msg: "no instructions found"}
	}

	if _, ok := instructions[0].(*stubs.FromInstruction); !ok {
		return nil, &ParseError{Line: 1, Msg: "first instruction must be FROM"}
	}

	return instructions, nil
}

func parseLine(lineNo int, line string) (stubs.Instruction, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil, &ParseError{Line: lineNo, Msg: "empty instruction"}
	}

	op := strings.ToUpper(fields[0])
	rest := strings.TrimSpace(line[len(fields[0]):])

	switch op {
	case "FROM":
		if rest == "" {
			return nil, &ParseError{Line: lineNo, Msg: "FROM requires image[:tag]"}
		}
		image, tag := splitImageTag(rest)
		return &stubs.FromInstruction{Image: image, Tag: tag, Raw: line}, nil

	case "COPY":
		if len(fields) < 3 {
			return nil, &ParseError{Line: lineNo, Msg: "COPY requires at least one source and one destination"}
		}
		src := fields[1 : len(fields)-1]
		dest := fields[len(fields)-1]
		return &stubs.CopyInstruction{Sources: src, Dest: dest, Raw: line}, nil

	case "RUN":
		if rest == "" {
			return nil, &ParseError{Line: lineNo, Msg: "RUN requires a command"}
		}
		return &stubs.RunInstruction{Command: rest, Raw: line}, nil

	case "WORKDIR":
		if rest == "" {
			return nil, &ParseError{Line: lineNo, Msg: "WORKDIR requires a path"}
		}
		return &stubs.WorkdirInstruction{Path: rest, Raw: line}, nil

	case "ENV":
		if !strings.Contains(rest, "=") {
			return nil, &ParseError{Line: lineNo, Msg: "ENV requires KEY=VALUE"}
		}
		parts := strings.SplitN(rest, "=", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, &ParseError{Line: lineNo, Msg: "ENV key cannot be empty"}
		}
		return &stubs.EnvInstruction{Key: key, Value: value, Raw: line}, nil

	case "CMD":
		if rest == "" {
			return nil, &ParseError{Line: lineNo, Msg: "CMD requires JSON array form"}
		}
		var cmd []string
		if err := json.Unmarshal([]byte(rest), &cmd); err != nil {
			return nil, &ParseError{Line: lineNo, Msg: fmt.Sprintf("invalid CMD JSON array: %v", err)}
		}
		if len(cmd) == 0 {
			return nil, &ParseError{Line: lineNo, Msg: "CMD array cannot be empty"}
		}
		return &stubs.CmdInstruction{Command: cmd, Raw: line}, nil
	default:
		return nil, &ParseError{Line: lineNo, Msg: fmt.Sprintf("unsupported instruction %q", op)}
	}
}

func splitImageTag(ref string) (string, string) {
	parts := strings.Split(ref, ":")
	if len(parts) == 1 {
		return strings.TrimSpace(parts[0]), "latest"
	}
	tag := parts[len(parts)-1]
	image := strings.Join(parts[:len(parts)-1], ":")
	if image == "" {
		return ref, "latest"
	}
	if tag == "" {
		return image, "latest"
	}
	return image, tag
}
