# Integration Test Fixtures

This directory contains reusable JSON configuration files for integration testing.

## Available Fixtures

### Network Interfaces

**interface-static.json**
- Single physical interface with static IP configuration
- Use case: Basic network connectivity tests
- Interface: eth0 (192.168.1.100/24)

**bridge-lan.json**
- Bridge interface with two member ports
- Use case: Bridge creation and port management tests
- Bridge: br-lan with ports eth0, eth1 (192.168.1.1/24)

**vlan-multi.json**
- Multiple VLAN interfaces on same parent
- Use case: VLAN segmentation and isolation tests
- VLANs: eth0.10, eth0.20, eth0.30 (IDs 10, 20, 30)

### Routing

**routes-static.json**
- Multiple static routes across different interfaces
- Use case: Route management and metric testing
- Routes: 3 static routes with different metrics

### Complex Scenarios

**complex-network.json**
- Bridge + VLAN + physical interfaces + routes
- Use case: Real-world network topology testing
- Components:
  - Bridge (br-lan) with 2 ports
  - VLAN on bridge (br-lan.100)
  - Standalone physical interface (eth2)
  - Default route + guest network route

## Usage in Tests

Load fixtures in your integration tests:

```go
import (
    "encoding/json"
    "os"
)

func loadFixture(path string, v interface{}) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return err
    }
    return json.Unmarshal(data, v)
}

// Example usage
var config types.InterfacesConfig
err := loadFixture("testing/fixtures/bridge-lan.json", &config)
```

Or use directly with the daemon:

```go
// Copy fixture to test config directory
fixtureData, _ := os.ReadFile("testing/fixtures/bridge-lan.json")
os.WriteFile(filepath.Join(harness.configDir, "interfaces.json"), fixtureData, 0644)

// Apply configuration
resp, err := harness.SendRequest(daemon.Request{Command: "apply"})
```

## Adding New Fixtures

When adding new fixtures:

1. Follow JSON naming convention: `<component>-<scenario>.json`
2. Use realistic network configurations
3. Document the fixture in this README
4. Keep fixtures focused on specific test scenarios
5. Validate JSON syntax before committing

## Fixture Format

All fixtures follow the Jack configuration schema:

- **interfaces**: Map of interface configurations (see [types/types.go](../../types/types.go))
- **routes**: Map of route configurations (see [types/types.go](../../types/types.go))
- **firewall**: Firewall zone configurations (future)
- **dhcp**: DHCP server configurations (future)
- **vpn**: VPN tunnel configurations (future)
