package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"docksmith-engine/internal"
)

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
		// Trim whitespace
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Parse instruction
		inst, err := parseInstruction(trimmed, lineNum)
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
func parseInstruction(line string, lineNum int) (internal.Instruction, error) {
	parts := strings.Fields(line)
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
			Raw:  line,
			Op:   op,
			Args: args,
		}, nil

	case "COPY":
		if len(args) < 2 {
			return internal.Instruction{}, fmt.Errorf("line %d: COPY requires at least source and destination", lineNum)
		}
		return internal.Instruction{
			Raw:  line,
			Op:   op,
			Args: args,
		}, nil

	case "RUN":
		if len(args) == 0 {
			return internal.Instruction{}, fmt.Errorf("line %d: RUN requires a command", lineNum)
		}
		return internal.Instruction{
			Raw:  line,
			Op:   op,
			Args: args,
		}, nil

	case "WORKDIR":
		if len(args) != 1 {
			return internal.Instruction{}, fmt.Errorf("line %d: WORKDIR requires exactly one path argument", lineNum)
		}
		return internal.Instruction{
			Raw:  line,
			Op:   op,
			Args: args,
		}, nil

	case "ENV":
		if len(args) < 1 {
			return internal.Instruction{}, fmt.Errorf("line %d: ENV requires KEY=VALUE format", lineNum)
		}
		// Rejoin all args in case ENV has spaces
		envStr := strings.Join(args, " ")
		if !strings.Contains(envStr, "=") {
			return internal.Instruction{}, fmt.Errorf("line %d: ENV requires KEY=VALUE format, got: %s", lineNum, envStr)
		}
		parts := strings.SplitN(envStr, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return internal.Instruction{}, fmt.Errorf("line %d: ENV invalid format: %s", lineNum, envStr)
		}
		return internal.Instruction{
			Raw:  line,
			Op:   op,
			Args: []string{strings.TrimSpace(parts[0]), parts[1]},
		}, nil

	case "CMD":
		// CMD must be a JSON array
		if len(args) == 0 {
			return internal.Instruction{}, fmt.Errorf("line %d: CMD requires JSON array format, e.g., CMD [\"cmd\",\"arg\"]", lineNum)
		}
		// Rejoin all args (JSON array might have spaces)
		cmdStr := strings.Join(args, " ")
		if !strings.HasPrefix(cmdStr, "[") || !strings.HasSuffix(cmdStr, "]") {
			return internal.Instruction{}, fmt.Errorf("line %d: CMD must be a JSON array, got: %s", lineNum, cmdStr)
		}
		// Validate JSON structure
		var cmdArray []string
		if err := json.Unmarshal([]byte(cmdStr), &cmdArray); err != nil {
			return internal.Instruction{}, fmt.Errorf("line %d: CMD invalid JSON array: %v", lineNum, err)
		}
		if len(cmdArray) == 0 {
			return internal.Instruction{}, fmt.Errorf("line %d: CMD array cannot be empty", lineNum)
		}
		return internal.Instruction{
			Raw:  line,
			Op:   op,
			Args: cmdArray,
		}, nil

	default:
		return internal.Instruction{}, fmt.Errorf("line %d: unknown instruction %q", lineNum, op)
	}
}

// readFile reads the content of a file.
// This is separated for easier testing and mocking.
func readFile(filePath string) (string, error) {
	content, err := internal.ReadFile(filePath)
	return content, err
}
