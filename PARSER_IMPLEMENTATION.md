# Parser Module Implementation

## Overview
The Parser module (`internal/parser/parser.go`) implements the `Parser` interface defined in `internal/interfaces.go`. It reads and parses `Docksmithfile` instructions with strict validation.

## Status
✅ **Fully Implemented** - Ready for integration

## Features

### Supported Instructions
- `FROM <image>` - Base image specification
- `COPY <src> <dest> [...]` - Copy files/directories  
- `RUN <command> [args...]` - Execute command during build
- `WORKDIR <path>` - Set working directory
- `ENV <KEY>=<VALUE>` - Set environment variables
- `CMD ["exec", "arg", ...]` - Default container command (JSON array format)

### Validation Rules
✅ **Raw String Preservation** - Critical for cache layer
- Original instruction text is preserved exactly as written
- Whitespace and formatting are maintained
- Cache depends on this raw string for digest calculation

✅ **Strict CMD Parsing**
- Must be valid JSON array format: `["cmd", "arg"]`
- Invalid JSON returns: `line N: CMD invalid JSON array`
- Empty arrays are rejected

✅ **ENV Validation**
- Must follow `KEY=VALUE` format
- Whitespace trimmed from key only
- Values can contain spaces and special characters
- Invalid format returns: `line N: ENV invalid format`

✅ **Error Handling**
- Line numbers included in all error messages
- Format: `line N: error description`
- Unknown instructions rejected immediately
- Missing/invalid arguments for each instruction

✅ **Comment & Whitespace Handling**
- Lines starting with `#` are ignored
- Empty lines are skipped
- Leading/trailing whitespace trimmed before parsing

## Usage

### Basic Usage
```go
import "docksmith-engine/internal/parser"

parser := parser.New()
instructions, err := parser.Parse("path/to/Docksmithfile")
if err != nil {
    log.Fatal(err)
}

for _, inst := range instructions {
    fmt.Printf("Op: %s, Args: %v\n", inst.Op, inst.Args)
}
```

### Output Structure
Each instruction is returned as:
```go
type Instruction struct {
    Raw  string   // Original line text (e.g., "FROM   ubuntu:20.04")
    Op   string   // Uppercase instruction (e.g., "FROM")
    Args []string // Parsed arguments (e.g., ["ubuntu:20.04"])
}
```

## Example

### Input Docksmithfile
```dockerfile
FROM ubuntu:20.04
WORKDIR /app
COPY . /app
ENV PATH=/usr/bin:$PATH
RUN echo "Building"
CMD ["./app", "--verbose"]
```

### Output Instructions
```
Instruction 1: Op=FROM, Args=["ubuntu:20.04"]
Instruction 2: Op=WORKDIR, Args=["/app"]
Instruction 3: Op=COPY, Args=[".", "/app"]
Instruction 4: Op=ENV, Args=["PATH", "/usr/bin:$PATH"]
Instruction 5: Op=RUN, Args=["echo", "Building"]
Instruction 6: Op=CMD, Args=["./app", "--verbose"]
```

## Error Examples

### Invalid CMD (not JSON)
```
Input: CMD echo hello
Error: line 2: CMD must be a JSON array, got: echo hello
```

### Invalid ENV (missing =)
```
Input: ENV KEY value
Error: line 2: ENV invalid format: KEY value
```

### Unknown Instruction
```
Input: INVALID arg
Error: line 1: unknown instruction "INVALID"
```

### Missing FROM
```
Input: RUN echo hello
Error: parse error (no FROM to base on)
```

## Testing

### Test File Location
`internal/parser/parser_test.go`

### Running Tests (requires Go 1.22+)
```bash
cd docksmith-engine
go test ./internal/parser -v
```

### Test Coverage
- ✅ Valid Docksmithfile parsing
- ✅ Invalid CMD format detection
- ✅ Invalid ENV format detection
- ✅ Unknown instruction rejection
- ✅ Raw string preservation
- ✅ Comment and empty line handling
- ✅ Multi-argument support
- ✅ ENV with spaces/special chars
- ✅ CMD JSON array validation

### Sample Test Command
```bash
go test ./internal/parser -run TestParseValidDocksmithfile -v
```

## Integration with CLI

### Before Integration
The CLI currently uses mock parser. To integrate the real parser:

```go
// In cmd/mocks.go, replace:
Parser: &mockParser{},

// With:
Parser: parser.New(),
```

### After Integration
```bash
go run ./cmd build -t myimage:latest sample-app
```

This will:
1. Read `sample-app/Docksmithfile`
2. Parse instructions with validation
3. Pass to Builder module
4. Builder uses raw strings for cache keys

## Important Notes for Integration

### ⚠️ Cache Dependency
The Cache and Layer modules depend on the **exact raw instruction text** for digest calculation:
```go
// Bad (modifying instruction):
cacheKey := fmt.Sprintf("%s %s", inst.Op, strings.Join(inst.Args, " "))

// Good (using raw string):
cacheKey := inst.Raw
```

### ⚠️ Instruction Order
Parser preserves instruction order exactly as written. Builder must process them sequentially.

### ⚠️ No STATE CHANGES
Parser only:
- Reads files
- Validates syntax
- Returns instructions

Parser does NOT:
- Execute commands
- Access filesystem beyond reading input file
- Modify any state
- Create layers or cache entries

## Files Modified/Created

1. **Created:**
   - `internal/parser/parser.go` - Main parser implementation
   - `internal/parser/parser_test.go` - Comprehensive test suite
   - `sample-app/Docksmithfile` - Sample for testing

2. **Modified:**
   - `internal/interfaces.go` - Added `ReadFile()` helper function

## Next Steps

Once parser is merged to `main`:

1. ✅ Parser complete
2. 🔜 Layer module (Chinmay)
3. 🔜 Image module (Chinmay)
4. 🔜 Cache module (Vishnu)
5. 🔜 Builder orchestration (Chinmay/Deepak)
6. 🔜 Runtime (ALL)

## Questions?

Refer to:
- Interface contracts: `internal/interfaces.go`
- Mock implementation pattern: `cmd/mocks.go`
- CLI integration: `cmd/commands.go:buildCommand()`
