# Test Coverage Guide

## Overview

Jack uses a **combined coverage approach** that includes both unit tests and integration tests to provide an accurate picture of test coverage.

## Coverage Types

### Unit Tests (35.2%)
- Fast, isolated tests that don't require system resources
- Run with: `make coverage-unit`
- Test pure logic, data structures, and helper functions
- Files: `*_test.go` (no build tags)

### Integration Tests (~15-20%)
- End-to-end tests that exercise the full system
- Require: Docker, privileged container, compiled plugins
- Run with: `make coverage-integration`
- Test daemon lifecycle, CLI commands, network configuration, plugin interactions
- Files: `test/integration/*_test.go` (with `//go:build integration` tag)

### Combined Coverage (~50-55%)
- Merges both unit and integration test coverage
- Run with: `make coverage`
- Enforces minimum threshold (currently 50%)

## Running Coverage

```bash
# Quick unit-only coverage (fast, ~30 seconds)
make coverage-unit

# Integration-only coverage (slow, ~5-10 minutes, requires Docker)
make coverage-integration

# Combined coverage with threshold enforcement
make coverage

# Just run tests without coverage
make test                 # Unit tests
make test-integration     # Integration tests
```

## Coverage Workflow

```
make coverage
├── coverage-unit
│   └── Runs: go test -coverprofile=coverage-unit.out ./...
│   └── Outputs: coverage-unit.out
│
├── coverage-integration
│   ├── Builds Docker container with test environment
│   ├── Runs: go test -tags=integration -cover ./test/integration/...
│   ├── Collects binary coverage via -test.gocoverdir
│   └── Outputs: coverage-integration.out
│
├── coverage-merge
│   ├── Merges coverage-unit.out + coverage-integration.out
│   └── Outputs: coverage-combined.out, coverage.html
│
└── Threshold Check
    └── Fails if combined coverage < MIN_COVERAGE (50%)
```

## Coverage Reports

After running `make coverage`, you can view:

1. **Terminal output** - Shows combined coverage percentage
2. **coverage.html** - Interactive HTML report showing covered/uncovered lines
3. **coverage-combined.out** - Raw coverage data for CI/CD tools

```bash
# View HTML report in browser
open coverage.html  # macOS
xdg-open coverage.html  # Linux
```

## Why Combined Coverage?

Many critical code paths (daemon startup, CLI commands, plugin loading, network configuration) cannot be tested in unit tests because they require:
- Running daemon process
- Unix socket communication
- Root privileges for netlink/sysctl
- Actual network interfaces
- Compiled plugin binaries

Integration tests cover these paths, providing a complete picture of what's actually tested.

## Coverage Goals

| Coverage Type | Current | Target |
|---------------|---------|--------|
| Unit Only | 35.2% | 40% |
| Integration Only | ~15-20% | ~20% |
| **Combined** | **~50-55%** | **60%** |

## Troubleshooting

### Integration tests fail

```bash
# Check Docker is running
docker ps

# Rebuild test container
docker build -t jack-integration-test -f Dockerfile.test .

# Run integration tests manually
docker run --rm --privileged jack-integration-test
```

### Coverage merge fails

```bash
# Clean coverage artifacts
make clean

# Run coverage steps individually to debug
make coverage-unit
make coverage-integration
make coverage-merge
```

### Low coverage warning

If combined coverage falls below 50%, this indicates:
1. New code was added without tests
2. Integration tests are not running (Docker issue)
3. Tests are failing and not generating coverage

Check which packages have low coverage:
```bash
go tool cover -func=coverage-combined.out | sort -k3 -n
```

## CI/CD Integration

For continuous integration:

```yaml
# Example GitHub Actions
- name: Run combined coverage
  run: make coverage

# Or run separately for better reporting
- name: Unit tests
  run: make coverage-unit
  
- name: Integration tests
  run: make coverage-integration
  
- name: Check combined coverage
  run: make coverage
```
