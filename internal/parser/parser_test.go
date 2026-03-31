package parser

import (
	"strings"
	"testing"

	"docksmith-engine/internal"
)

func TestParseValidDocksmithfile(t *testing.T) {
	// Create a temporary test file
	content := `FROM ubuntu:20.04
WORKDIR /app
COPY . /app
ENV KEY=value
RUN echo hello
CMD ["echo","hi"]`

	// Mock reading
	originalReadFile := readFile
	readFile = func(filePath string) (string, error) {
		return content, nil
	}
	defer func() { readFile = originalReadFile }()

	parser := New()
	instructions, err := parser.Parse("Docksmithfile")

	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(instructions) != 6 {
		t.Errorf("Expected 6 instructions, got %d", len(instructions))
	}

	// Verify each instruction
	tests := []struct {
		idx    int
		op     string
		rawLen int
	}{
		{0, "FROM", 1},
		{1, "WORKDIR", 1},
		{2, "COPY", 2},
		{3, "ENV", 2},
		{4, "RUN", 2},
		{5, "CMD", 2},
	}

	for _, test := range tests {
		if instructions[test.idx].Op != test.op {
			t.Errorf("Instruction %d: expected Op=%s, got %s", test.idx, test.op, instructions[test.idx].Op)
		}
		if len(instructions[test.idx].Args) != test.rawLen {
			t.Errorf("Instruction %d: expected %d args, got %d", test.idx, test.rawLen, len(instructions[test.idx].Args))
		}
		if instructions[test.idx].Raw == "" {
			t.Errorf("Instruction %d: Raw string should not be empty", test.idx)
		}
	}
}

func TestParseInvalidCMD(t *testing.T) {
	content := `FROM base
CMD echo hello`

	originalReadFile := readFile
	readFile = func(filePath string) (string, error) {
		return content, nil
	}
	defer func() { readFile = originalReadFile }()

	parser := New()
	_, err := parser.Parse("Docksmithfile")

	if err == nil {
		t.Fatal("Expected error for invalid CMD format, got nil")
	}

	if !strings.Contains(err.Error(), "JSON array") {
		t.Errorf("Expected JSON array error, got: %v", err)
	}
}

func TestParseInvalidENV(t *testing.T) {
	content := `FROM base
ENV KEY value`

	originalReadFile := readFile
	readFile = func(filePath string) (string, error) {
		return content, nil
	}
	defer func() { readFile = originalReadFile }()

	parser := New()
	_, err := parser.Parse("Docksmithfile")

	if err == nil {
		t.Fatal("Expected error for invalid ENV format, got nil")
	}

	if !strings.Contains(err.Error(), "KEY=VALUE") {
		t.Errorf("Expected KEY=VALUE error, got: %v", err)
	}
}

func TestParseUnknownInstruction(t *testing.T) {
	content := `FROM base
UNKNOWN arg`

	originalReadFile := readFile
	readFile = func(filePath string) (string, error) {
		return content, nil
	}
	defer func() { readFile = originalReadFile }()

	parser := New()
	_, err := parser.Parse("Docksmithfile")

	if err == nil {
		t.Fatal("Expected error for unknown instruction, got nil")
	}

	if !strings.Contains(err.Error(), "unknown instruction") {
		t.Errorf("Expected unknown instruction error, got: %v", err)
	}
}

func TestParsePreservesRawString(t *testing.T) {
	content := `FROM   ubuntu:20.04
RUN    echo    hello`

	originalReadFile := readFile
	readFile = func(filePath string) (string, error) {
		return content, nil
	}
	defer func() { readFile = originalReadFile }()

	parser := New()
	instructions, err := parser.Parse("Docksmithfile")

	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Raw string should be preserved with original formatting
	if instructions[0].Raw != "FROM   ubuntu:20.04" {
		t.Errorf("Raw string not preserved: got %q", instructions[0].Raw)
	}
}

func TestParseSkipsComments(t *testing.T) {
	content := `FROM base
# This is a comment
RUN echo hello`

	originalReadFile := readFile
	readFile = func(filePath string) (string, error) {
		return content, nil
	}
	defer func() { readFile = originalReadFile }()

	parser := New()
	instructions, err := parser.Parse("Docksmithfile")

	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(instructions) != 2 {
		t.Errorf("Expected 2 instructions (comment skipped), got %d", len(instructions))
	}
}

func TestParseEmptyLines(t *testing.T) {
	content := `FROM base

RUN echo hello

`

	originalReadFile := readFile
	readFile = func(filePath string) (string, error) {
		return content, nil
	}
	defer func() { readFile = originalReadFile }()

	parser := New()
	instructions, err := parser.Parse("Docksmithfile")

	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(instructions) != 2 {
		t.Errorf("Expected 2 instructions (empty lines skipped), got %d", len(instructions))
	}
}

func TestParseENVWithValue(t *testing.T) {
	content := `FROM base
ENV PATH=/usr/bin:$PATH`

	originalReadFile := readFile
	readFile = func(filePath string) (string, error) {
		return content, nil
	}
	defer func() { readFile = originalReadFile }()

	parser := New()
	instructions, err := parser.Parse("Docksmithfile")

	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	envInst := instructions[1]
	if envInst.Op != "ENV" {
		t.Errorf("Expected ENV instruction, got %s", envInst.Op)
	}
	if len(envInst.Args) != 2 {
		t.Errorf("Expected 2 args for ENV, got %d", len(envInst.Args))
	}
	if envInst.Args[0] != "PATH" {
		t.Errorf("Expected ENV key=PATH, got %s", envInst.Args[0])
	}
	if !strings.Contains(envInst.Args[1], "/usr/bin") {
		t.Errorf("Expected ENV value to contain /usr/bin, got %s", envInst.Args[1])
	}
}

func TestParseCMDValidJSON(t *testing.T) {
	content := `FROM base
CMD ["python", "app.py"]`

	originalReadFile := readFile
	readFile = func(filePath string) (string, error) {
		return content, nil
	}
	defer func() { readFile = originalReadFile }()

	parser := New()
	instructions, err := parser.Parse("Docksmithfile")

	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	cmdInst := instructions[1]
	if cmdInst.Op != "CMD" {
		t.Errorf("Expected CMD instruction, got %s", cmdInst.Op)
	}
	if len(cmdInst.Args) != 2 || cmdInst.Args[0] != "python" || cmdInst.Args[1] != "app.py" {
		t.Errorf("Expected CMD args [python, app.py], got %v", cmdInst.Args)
	}
}
