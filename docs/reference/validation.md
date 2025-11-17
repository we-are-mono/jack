# Configuration Validation

Learn how to validate Jack configuration files before applying them.

## The `jack validate` Command

The `jack validate` command checks all JSON configuration files for syntax errors and structural issues without making any changes to the running system.

### Basic Usage

```bash
jack validate
```

### When to Use Validation

**Always validate before:**
- Restarting the Jack daemon
- Applying configuration changes
- Deploying to production
- Making manual edits to JSON files

**Best practice workflow:**
```bash
# 1. Edit configuration
vim /etc/jack/interfaces.json

# 2. Validate before applying
jack validate

# 3. If valid, restart daemon
systemctl restart jack
```

---

## Understanding Validation Output

### All Valid

When all files are valid:

```
Validating configuration files in /etc/jack...

✓ interfaces.json: valid
✓ routes.json: valid
✓ jack.json: valid
✓ nftables.json: valid
✓ dnsmasq.json: valid
✓ wireguard.json: valid

✓ All configuration files are valid
```

**Exit code:** `0`

---

### Syntax Errors

When JSON syntax is invalid:

```
Validating configuration files in /etc/jack...

❌ interfaces.json: failed to parse interfaces config at /etc/jack/interfaces.json line 7, column 8: invalid character ']' looking for beginning of value
✓ routes.json: valid
✓ jack.json: valid

❌ Validation failed - please fix the errors above
```

**Exit code:** `1`

The error message shows:
- **File path**: `/etc/jack/interfaces.json`
- **Line number**: `7`
- **Column number**: `8`
- **Error details**: `invalid character ']' looking for beginning of value`

---

### Optional Files

Some configuration files are optional:

```
⊘ wireguard.json: not found (optional)
```

This is normal if you haven't configured that feature yet.

---

## Common JSON Errors

### Trailing Commas

**Invalid:**
```json
{
  "bridge_ports": [
    "eth1",
    "eth2",  ← trailing comma
  ]
}
```

**Valid:**
```json
{
  "bridge_ports": [
    "eth1",
    "eth2"
  ]
}
```

---

### Missing Quotes

**Invalid:**
```json
{
  "ipaddr": 192.168.1.1
}
```

**Valid:**
```json
{
  "ipaddr": "192.168.1.1"
}
```

---

### Missing Commas

**Invalid:**
```json
{
  "ipaddr": "192.168.1.1"
  "netmask": "255.255.255.0"
}
```

**Valid:**
```json
{
  "ipaddr": "192.168.1.1",
  "netmask": "255.255.255.0"
}
```

---

### Unmatched Brackets

**Invalid:**
```json
{
  "interfaces": {
    "lan": {
      "enabled": true
    }
  }
}  ← missing closing brace
```

**Valid:**
```json
{
  "interfaces": {
    "lan": {
      "enabled": true
    }
  }
}
```

---

## Files Validated

The `jack validate` command checks these files:

### Required Files

- **interfaces.json** - Network interface configuration
- **jack.json** - Plugin configuration

### Optional Files

- **routes.json** - Static routes
- **nftables.json** - Firewall rules (if nftables plugin enabled)
- **dnsmasq.json** - DHCP/DNS configuration (if dnsmasq plugin enabled)
- **wireguard.json** - VPN configuration (if wireguard plugin enabled)

---

## Validation in CI/CD

Use `jack validate` in continuous integration pipelines:

### GitHub Actions Example

```yaml
name: Validate Configuration

on: [push, pull_request]

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Install Jack
        run: |
          ./build.sh
          sudo cp jack /usr/local/bin/

      - name: Validate Configuration
        run: |
          sudo jack validate
```

### Shell Script Example

```bash
#!/bin/bash
set -e

echo "Validating Jack configuration..."

if jack validate; then
    echo "✓ Configuration is valid"

    echo "Applying configuration..."
    systemctl restart jack

    echo "✓ Configuration applied successfully"
else
    echo "❌ Configuration validation failed!"
    exit 1
fi
```

---

## Daemon Startup Validation

When the Jack daemon starts, it automatically validates configuration files. If validation fails, the daemon will not start and will log helpful error messages.

### Viewing Daemon Errors

```bash
# Check daemon status
systemctl status jack

# View daemon logs
journalctl -u jack -n 50 --no-pager

# Or check log file directly
tail -f /var/log/jack/jack.log
```

### Example Daemon Error

```
[ERROR] Server failed: failed to load interfaces config:
  failed to parse interfaces config at /etc/jack/interfaces.json line 7, column 8:
  invalid character ']' looking for beginning of value

Tip: Run 'jack validate' to check for configuration errors
```

The daemon will suggest running `jack validate` for a clearer view of all errors.

---

## Manual JSON Validation

If you prefer, you can also use standard JSON tools:

### jq

```bash
# Validate JSON syntax
jq empty /etc/jack/interfaces.json

# Pretty-print JSON
jq . /etc/jack/interfaces.json
```

### Python

```bash
python3 -m json.tool /etc/jack/interfaces.json
```

However, `jack validate` is recommended because it:
- Checks all files at once
- Provides line/column error locations
- Validates Jack-specific structure
- Matches the daemon's validation logic

---

## Troubleshooting

### "File not found" Errors

If validation reports files not found:

```bash
# Check config directory
ls -la /etc/jack/

# Check JACK_CONFIG_DIR environment variable
echo $JACK_CONFIG_DIR
```

### Permission Denied

Validation requires read access to `/etc/jack/`:

```bash
# Run with sudo
sudo jack validate

# Or fix permissions
sudo chmod 644 /etc/jack/*.json
```

### False Positives

If `jack validate` reports errors but your JSON appears correct:

1. Check for invisible characters (copy-paste issues)
2. Ensure UTF-8 encoding
3. Use a JSON linter/formatter
4. Compare with [example configurations](../../examples/config/)

---

## See Also

- [Configuration Reference](../configuration.md) - Complete configuration guide
- [CLI Commands](cli-commands.md) - All jack commands
- [Getting Started](../getting-started.md) - Initial setup
- [Example Configurations](../../examples/config/) - Working examples
