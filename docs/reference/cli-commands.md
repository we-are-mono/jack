# Jack CLI Command Reference

Complete reference for all jack commands.

## Core Commands

### `jack daemon`

Run Jack as a background daemon.

```bash
jack daemon [--apply]
```

**Options:**
- `--apply` - Apply configuration on startup

**Description:**
Starts the Jack daemon which listens for commands on `/var/run/jack.sock`. The daemon manages all network configuration and plugin lifecycle.

**Logs:** `/var/log/jack/jack.log`

**Example:**
```bash
# Start daemon without applying config
jack daemon

# Start daemon and apply config on startup
jack daemon --apply
```

---

### `jack apply`

Apply committed configuration to the system.

```bash
jack apply
```

**Description:**
Applies the currently committed configuration to network interfaces, firewall rules, DHCP, and other services. Changes are idempotent - running apply multiple times with the same config won't cause service restarts or disruptions.

**Example:**
```bash
jack apply
```

---

### `jack validate`

Validate configuration files without applying them.

```bash
jack validate
```

**Description:**
Checks all JSON configuration files for syntax errors and validates their structure. Returns exit code 0 if all files are valid, 1 if errors are found.

**Output:**
```
✓ interfaces.json: valid
✓ routes.json: valid
❌ nftables.json: failed to parse at line 42, column 15: invalid character ']'
```

**See also:** [Configuration Validation Guide](validation.md)

---

### `jack status`

Show system status and running services.

```bash
jack status
```

**Description:**
Displays current status of interfaces, routes, firewall, DHCP server, and other managed services.

---

### `jack show`

Show current configuration.

```bash
jack show [path]
```

**Arguments:**
- `path` (optional) - Configuration path to show (e.g., `interfaces.lan`)

**Examples:**
```bash
# Show all configuration
jack show

# Show specific interface
jack show interfaces lan

# Show firewall rules
jack show firewall
```

---

## Configuration Management

### `jack set`

Set a configuration value.

```bash
jack set <path> <value>
```

**Arguments:**
- `path` - Configuration path components as space-separated arguments (e.g., `interfaces lan ipaddr`)
- `value` - New value (JSON for complex types)

**Description:**
Sets a configuration value in pending state. Changes are not applied until you run `jack commit` and `jack apply`.

**Examples:**
```bash
# Set IP address
jack set interfaces lan ipaddr 192.168.2.1

# Set bridge ports (JSON array)
jack set interfaces lan bridge_ports '["eth1","eth2"]'

# Enable interface
jack set interfaces wan enabled true
```

---

### `jack get`

Get a configuration value.

```bash
jack get <path>
```

**Arguments:**
- `path` - Configuration path to retrieve

**Examples:**
```bash
jack get interfaces lan ipaddr
jack get interfaces lan
```

---

### `jack diff`

Show pending changes.

```bash
jack diff
```

**Description:**
Displays differences between committed and pending configuration.

**Example output:**
```
interfaces.lan.ipaddr: "192.168.1.1" → "192.168.2.1"
interfaces.lan.bridge_ports: ["eth1"] → ["eth1", "eth2"]
```

---

### `jack commit`

Commit pending changes.

```bash
jack commit
```

**Description:**
Commits pending changes to make them the active configuration. Changes are not applied to the system until you run `jack apply`.

**Workflow:**
```bash
jack set interfaces lan ipaddr 192.168.2.1  # Pending
jack diff                                    # Review
jack commit                                  # Commit
jack apply                                   # Apply to system
```

---

### `jack revert`

Discard pending changes.

```bash
jack revert
```

**Description:**
Discards all pending changes and reverts to the committed configuration.

---

## Plugin Management

### `jack plugin list`

List all available plugins.

```bash
jack plugin list
```

**Example output:**
```
Available plugins:
  nftables   (firewall)  - firewall firewall management
  dnsmasq    (dhcp)      - dnsmasq-based DHCP server
  wireguard  (vpn)       - WireGuard VPN management
```

---

### `jack plugin status`

Show plugin configuration status.

```bash
jack plugin status [plugin-name]
```

**Arguments:**
- `plugin-name` (optional) - Specific plugin to show

**Examples:**
```bash
# Show all plugins
jack plugin status

# Show specific plugin
jack plugin status nftables
```

---

### `jack plugin info`

Show detailed plugin information.

```bash
jack plugin info <plugin-name>
```

**Arguments:**
- `plugin-name` - Plugin to query

**Example:**
```bash
jack plugin info nftables
```

---

### `jack plugin enable`

Enable a plugin.

```bash
jack plugin enable <plugin-name>
```

**Arguments:**
- `plugin-name` - Plugin to enable

**Example:**
```bash
jack plugin enable wireguard
```

---

### `jack plugin disable`

Disable a plugin.

```bash
jack plugin disable <plugin-name>
```

**Arguments:**
- `plugin-name` - Plugin to disable

**Example:**
```bash
jack plugin disable wireguard
```

---

## Logging

### `jack logs`

Show Jack daemon logs.

```bash
jack logs [-n lines] [-f]
```

**Options:**
- `-n <lines>` - Number of lines to show (default: 50)
- `-f` - Follow log output (like tail -f)

**Examples:**
```bash
# Show last 50 lines
jack logs

# Show last 100 lines
jack logs -n 100

# Follow logs in real-time
jack logs -f
```

**Direct file access:**
```bash
tail -f /var/log/jack/jack.log
```

---

## Help

### `jack help`

Show help for any command.

```bash
jack help [command]
```

**Examples:**
```bash
jack help
jack help apply
jack help plugin
```

---

### `jack version`

Show Jack version.

```bash
jack version
```

---

## Exit Codes

All jack commands return standard exit codes:

- `0` - Success
- `1` - Error or validation failure
- `2` - Command usage error

This makes jack suitable for scripting and CI/CD pipelines:

```bash
if jack validate; then
    jack apply
else
    echo "Configuration invalid!"
    exit 1
fi
```

---

## Environment Variables

### `JACK_CONFIG_DIR`

Override the default configuration directory (`/etc/jack`).

```bash
export JACK_CONFIG_DIR="/custom/config/path"
jack validate
```

---

## Files and Directories

- `/etc/jack/` - Default configuration directory
- `/etc/jack/*.json` - Configuration files
- `/var/run/jack.sock` - Daemon Unix socket
- `/var/log/jack/jack.log` - Daemon log file
- `/usr/local/bin/jack` - Main binary
- `/usr/lib/jack/plugins/` - Plugin binaries

---

## See Also

- [Getting Started Guide](../getting-started.md)
- [Configuration Reference](../configuration.md)
- [Configuration Validation](validation.md)
