package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"docksmith-engine/internal"
)

// readFile is overridable for tests.
var readFile = func(filePath string) (string, error) {
	return internal.ReadFile(filePath)
}

// Parser implements the internal.Parser interface for parsing Docksmithfile instructions.
type Parser struct{}

// New creates a new Parser instance.
func New() *Parser {
	return &Parser{}
}

// Parse reads and parses a Docksmithfile, returning a list of instructions.
func (p *Parser) Parse(filePath string) ([]internal.Instruction, error) {
	if strings.TrimSpace(filePath) == "" {
		return nil, fmt.Errorf("Docksmithfile path cannot be empty")
	}

	// Read file
	content, err := readFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Docksmithfile: %w", err)
	}

	lines := strings.Split(content, "\n")
	var instructions []internal.Instruction
	lineNum := 0

	for _, line := range lines {
		lineNum++
		// Normalize CRLF line endings.
		line = strings.TrimSuffix(line, "\r")
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Parse instruction
		inst, err := parseInstruction(line, trimmed, lineNum)
		if err != nil {
			return nil, err
		}

		instructions = append(instructions, inst)
	}

	if len(instructions) == 0 {
		return nil, fmt.Errorf("Docksmithfile contains no valid instructions")
	}

	return instructions, nil
}

// parseInstruction parses a single instruction line.
// rawLine is preserved verbatim in Instruction.Raw.
func parseInstruction(rawLine string, trimmedLine string, lineNum int) (internal.Instruction, error) {
	parts := strings.Fields(trimmedLine)
	if len(parts) == 0 {
		return internal.Instruction{}, fmt.Errorf("line %d: empty instruction", lineNum)
	}

	op := strings.ToUpper(parts[0])
	args := parts[1:]

	// Validate instruction type
	switch op {
	case "FROM":
		if len(args) != 1 {
			return internal.Instruction{}, fmt.Errorf("line %d: FROM requires exactly one argument (image name)", lineNum)
		}
		return internal.Instruction{
			Raw:  rawLine,
			Op:   op,
			Args: args,
		}, nil

	case "COPY":
		if len(args) < 2 {
			return internal.Instruction{}, fmt.Errorf("line %d: COPY requires at least source and destination", lineNum)
		}
		return internal.Instruction{
			Raw:  rawLine,
			Op:   op,
			Args: args,
		}, nil

	case "RUN":
		if len(args) == 0 {
			return internal.Instruction{}, fmt.Errorf("line %d: RUN requires a command", lineNum)
		}
		return internal.Instruction{
			Raw:  rawLine,
			Op:   op,
			Args: args,
		}, nil

	case "WORKDIR":
		if len(args) != 1 {
			return internal.Instruction{}, fmt.Errorf("line %d: WORKDIR requires exactly one path argument", lineNum)
		}
		return internal.Instruction{
			Raw:  rawLine,
			Op:   op,
			Args: args,
		}, nil

	case "ENV":
		// Strict ENV format: exactly one token KEY=value
		if len(args) != 1 {
			return internal.Instruction{}, fmt.Errorf("line %d: invalid ENV format", lineNum)
		}
		k, v, ok := strings.Cut(args[0], "=")
		if !ok || strings.TrimSpace(k) == "" {
			return internal.Instruction{}, fmt.Errorf("line %d: invalid ENV format", lineNum)
		}
		_ = v
		return internal.Instruction{
			Raw:  rawLine,
			Op:   op,
			Args: []string{args[0]},
		}, nil

	case "CMD":
		// CMD must be a JSON array
		if len(args) == 0 {
			return internal.Instruction{}, fmt.Errorf("line %d: CMD must be JSON array", lineNum)
		}
		cmdStr := strings.Join(args, " ")
		var cmdArray []string
		if err := json.Unmarshal([]byte(cmdStr), &cmdArray); err != nil {
			return internal.Instruction{}, fmt.Errorf("line %d: CMD must be JSON array", lineNum)
		}
		if len(cmdArray) == 0 {
			return internal.Instruction{}, fmt.Errorf("line %d: CMD must be JSON array", lineNum)
		}
		return internal.Instruction{
			Raw:  rawLine,
			Op:   op,
			Args: cmdArray,
		}, nil

	default:
		return internal.Instruction{}, fmt.Errorf("line %d: unknown instruction %s", lineNum, op)
	}
}

