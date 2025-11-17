# Workflow Test Fixtures

This directory contains JSON configuration fixtures used for testing workflow operations (set, commit, apply, diff, revert) in integration tests.

## Files

### initial-state.json
Basic initial configuration with:
- Single physical interface (eth0) with static IP
- One static route

Used for testing basic workflow operations and persistence.

### staged-changes.json
Modified configuration demonstrating staging:
- eth0 with changed IP (192.168.1.20) and MTU (9000)
- Additional eth1 interface
- Additional route

Used for testing diff operations and staged vs committed state.

### invalid-vlan-no-parent.json
Invalid configuration with VLAN referencing non-existent parent interface.

Used for testing validation and error handling during apply.

### invalid-duplicate-ip.json
Invalid configuration with two interfaces having the same IP address.

Used for testing conflict detection and validation.

### complex-multi-component.json
Complex configuration with multiple component types:
- 3 physical interfaces
- 1 bridge (br-lan) with 2 members
- 2 VLANs (100, 200) on eth0
- 3 routes including default gateway

Used for testing:
- Complex workflow operations
- Multi-step transactions
- Comprehensive diff output
- Full system integration

## Usage

These fixtures are referenced by integration tests in `testing/integration/`:
- `workflow_test.go` - Set/Commit/Apply workflow
- `diff_test.go` - Configuration diff operations
- `validation_test.go` - Configuration validation
- `error_handling_test.go` - Error recovery

While these fixtures can be loaded directly for testing, most integration tests create configurations programmatically to ensure test isolation and use dummy interfaces specific to each test's network namespace.

## Notes

- All interface names should be adapted to match test-created dummy interfaces
- IP addresses use private ranges to avoid conflicts
- MTU values use realistic ranges (1500, 9000)
- VLAN IDs use valid ranges (1-4094)
