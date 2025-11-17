# Testing Framework for Jack

## Overview

Jack now has a comprehensive testing framework using Go's standard testing tools plus testify for better assertions.

## Current Test Coverage: 5.2%

Critical components tested:
- **State management** - Config tracking (committed vs pending)
- **Plugin registry** - Plugin registration and lookup
- **RPC protocol** - Plugin communication layer
- **Jack config** - Plugin list management

## Running Tests

### Quick test run
```bash
make test
```

### With coverage report
```bash
make coverage
# Open coverage.html in browser to see detailed coverage
```

### Linting (requires golangci-lint)
```bash
# Install linter first
make install-tools

# Run linter
make lint
```

### Full check (before deployment)
```bash
make check-full
```

## Test Structure

Tests follow Go conventions - test files live next to the code they test:

```
daemon/
├── server.go
├── server_test.go      ← Tests for server.go
├── state.go
├── state_test.go       ← Tests for state.go
└── registry_test.go    ← Tests for registry.go
```

## Current Test Files

### daemon/state_test.go
Tests for state management:
- Config loading (committed/pending)
- Commit/revert operations
- Interface and route handling
- Concurrent access safety

### daemon/registry_test.go
Tests for plugin registry:
- Plugin registration
- Duplicate prevention
- Plugin lookup
- Concurrent access

### plugins/rpc_test.go
Tests for RPC protocol:
- Metadata structure
- CLI command descriptors
- Provider interface methods
- Error handling

### state/jack_test.go
Tests for Jack configuration:
- Default config generation
- Plugin list management
- Config structure validation

## Dependencies

- **testify** - Better assertions and mocks
  ```bash
  go get github.com/stretchr/testify
  ```

- **golangci-lint** - Dead code detection and linting
  ```bash
  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
  ```

## Linter Configuration

See `.golangci.yml` for linter settings:
- Dead code detection
- Unused variables/functions
- Security issues (gosec)
- Code formatting
- Import organization

## Adding New Tests

### 1. Create test file next to source
```bash
# For daemon/foo.go, create:
daemon/foo_test.go
```

### 2. Use testify for assertions
```go
package daemon

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestFoo(t *testing.T) {
    result := DoSomething()

    // require stops test on failure
    require.NoError(t, err)
    require.NotNil(t, result)

    // assert continues test on failure
    assert.Equal(t, expected, actual)
    assert.True(t, condition)
}
```

### 3. Test private functions
Tests in the same package can access unexported (private) functions:

```go
// foo.go
func doInternalThing() error {
    // private function
}

// foo_test.go (same package)
func TestDoInternalThing(t *testing.T) {
    // Can test private functions!
    err := doInternalThing()
    assert.NoError(t, err)
}
```

## Test Levels

### Unit Tests (Fast)
Test individual functions/methods in isolation:
- Config parsing
- Plugin registration
- State transitions

### Integration Tests (Medium)
Test component interactions:
- Plugin loading and RPC communication
- CLI command execution
- Config persistence

### E2E Tests (Slow, future)
Test complete workflows:
- Full daemon lifecycle
- Plugin discovery and loading
- CLI to daemon communication

## Coverage Goals

- **Critical paths**: 80%+ coverage
- **Overall project**: 60%+ coverage
- **New code**: Always add tests

Run `make coverage` to see current coverage and identify gaps.

## Continuous Integration

Before every deployment, run:
```bash
make check-full
```

This will:
1. Run linter (find dead code, security issues)
2. Run all tests with race detector
3. Generate coverage report
4. Fail if coverage is too low

## Finding Dead Code

The linter automatically detects:
- Unused functions
- Unused variables
- Unused struct fields
- Unreachable code

Run `make lint` to see warnings.

## Best Practices

1. **Write tests first** - TDD helps design better APIs
2. **Test behavior, not implementation** - Focus on what, not how
3. **Use descriptive test names** - `TestState_CommitPending_Success`
4. **One assert per test** (when possible) - Makes failures clear
5. **Use table-driven tests** for multiple similar cases
6. **Mock external dependencies** - File system, network, etc.
7. **Test error paths** - Don't just test happy paths

## Example: Table-Driven Test

```go
func TestPluginValidation(t *testing.T) {
    tests := []struct {
        name    string
        plugin  string
        wantErr bool
    }{
        {"valid plugin", "nftables", false},
        {"empty name", "", true},
        {"invalid chars", "foo@bar", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidatePlugin(tt.plugin)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

## Makefile Targets

- `make test` - Run all tests
- `make coverage` - Generate coverage report
- `make lint` - Run linter
- `make check` - Quick check (lint + test)
- `make check-full` - Full check with coverage
- `make install-tools` - Install dev tools
- `make clean` - Clean artifacts
