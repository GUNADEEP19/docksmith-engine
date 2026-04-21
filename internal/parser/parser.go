
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
	// Extract the operation token (first word) and the remainder of the line.
	// We preserve the remainder verbatim for instructions that need it (CMD expects a JSON array).
	firstSep := -1
	for i, r := range trimmedLine {
		if r == ' ' || r == '\t' {
			firstSep = i
			break
		}
	}
	var op string
	var remainder string
	if firstSep == -1 {
		op = strings.ToUpper(trimmedLine)
		remainder = ""
	} else {
		op = strings.ToUpper(strings.TrimSpace(trimmedLine[:firstSep]))
		remainder = strings.TrimSpace(trimmedLine[firstSep+1:])
	}

	// Split remainder into args for all instructions except CMD (CMD expects raw JSON).
	var args []string
	if op != "CMD" {
		var err error
		args, err = splitArgs(remainder)
		if err != nil {
			return internal.Instruction{}, fmt.Errorf("line %d: %w", lineNum, err)
		}
	}

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
		// CMD must be a JSON array; preserve the remainder verbatim and unmarshal it.
		if strings.TrimSpace(remainder) == "" {
			return internal.Instruction{}, fmt.Errorf("line %d: CMD must be JSON array", lineNum)
		}
		var cmdArray []string
		if err := json.Unmarshal([]byte(remainder), &cmdArray); err != nil {
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

// splitArgs splits a command line into arguments respecting single and double quotes
// and simple backslash escapes. It returns an error when a quoted string is not closed.
func splitArgs(s string) ([]string, error) {
	var out []string
	var cur strings.Builder
	inSingle := false
	inDouble := false
	escaped := false

	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}

	for _, r := range s {
		if escaped {
			cur.WriteRune(r)
			escaped = false
			continue
		}
		switch r {
		case '\\':
			// start escape for next char
			escaped = true
		case '\'':
			if inDouble {
				cur.WriteRune(r)
				continue
			}
			if inSingle {
				inSingle = false
				// do not append the quote
				continue
			}
			inSingle = true
		case '"':
			if inSingle {
				cur.WriteRune(r)
				continue
			}
			if inDouble {
				inDouble = false
				// do not append the quote
				continue
			}
			inDouble = true
		case ' ', '\t':
			if inSingle || inDouble {
				cur.WriteRune(r)
				continue
			}
			// separator
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	if escaped {
		// trailing backslash: treat as literal
		cur.WriteRune('\\')
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("unterminated quoted string")
	}
	flush()
	return out, nil
}

