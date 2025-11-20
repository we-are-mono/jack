// Copyright (C) 2025 Mono Technologies Inc.
//
// This program is free software; you can redistribute it and/or
// modify it under the terms of the GNU General Public License
// as published by the Free Software Foundation; version 2.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBoolToStatus tests boolean to status string conversion
func TestBoolToStatus(t *testing.T) {
	tests := []struct {
		name     string
		input    bool
		expected string
	}{
		{
			name:     "true returns Active",
			input:    true,
			expected: "Active",
		},
		{
			name:     "false returns Inactive",
			input:    false,
			expected: "Inactive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := boolToStatus(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestBoolToYesNo tests boolean to yes/no conversion
func TestBoolToYesNo(t *testing.T) {
	tests := []struct {
		name     string
		input    bool
		expected string
	}{
		{
			name:     "true returns Yes",
			input:    true,
			expected: "Yes",
		},
		{
			name:     "false returns No",
			input:    false,
			expected: "No",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := boolToYesNo(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFormatBytes tests byte formatting
func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{
			name:     "bytes",
			input:    512,
			expected: "512 B",
		},
		{
			name:     "kilobytes",
			input:    1536, // 1.5 KiB
			expected: "1.5 KiB",
		},
		{
			name:     "megabytes",
			input:    1572864, // 1.5 MiB
			expected: "1.5 MiB",
		},
		{
			name:     "gigabytes",
			input:    1610612736, // 1.5 GiB
			expected: "1.5 GiB",
		},
		{
			name:     "exact kilobyte",
			input:    1024,
			expected: "1.0 KiB",
		},
		{
			name:     "exact megabyte",
			input:    1048576,
			expected: "1.0 MiB",
		},
		{
			name:     "zero bytes",
			input:    0,
			expected: "0 B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytes(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestToTitleCase tests title case conversion
func TestToTitleCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "lowercase words",
			input:    "hello world",
			expected: "Hello World",
		},
		{
			name:     "uppercase words",
			input:    "HELLO WORLD",
			expected: "Hello World",
		},
		{
			name:     "mixed case",
			input:    "hElLo WoRlD",
			expected: "Hello World",
		},
		{
			name:     "single word",
			input:    "test",
			expected: "Test",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "with underscores removed",
			input:    "lease count",
			expected: "Lease Count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toTitleCase(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// captureOutput captures stdout during function execution
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// TestPrintInterface tests interface printing
func TestPrintInterface(t *testing.T) {
	tests := []struct {
		name     string
		iface    map[string]interface{}
		contains []string
	}{
		{
			name: "interface up with IP",
			iface: map[string]interface{}{
				"name":  "eth0",
				"type":  "physical",
				"state": "up",
				"ipaddr": []interface{}{
					"192.168.1.1/24",
				},
				"mtu":        float64(1500),
				"rx_packets": float64(1000),
				"tx_packets": float64(2000),
				"rx_bytes":   float64(1048576), // 1 MiB
				"tx_bytes":   float64(2097152), // 2 MiB
			},
			contains: []string{
				"[UP] eth0 (physical)",
				"State:      up",
				"IP Address: 192.168.1.1/24",
				"MTU:        1500",
				"RX:         1000 packets (1.0 MiB)",
				"TX:         2000 packets (2.0 MiB)",
			},
		},
		{
			name: "interface down without IP",
			iface: map[string]interface{}{
				"name":  "eth1",
				"type":  "bridge",
				"state": "down",
			},
			contains: []string{
				"[DOWN] eth1 (bridge)",
				"State:      down",
				"IP Address: (none)",
			},
		},
		{
			name: "interface with errors",
			iface: map[string]interface{}{
				"name":       "eth2",
				"type":       "vlan",
				"state":      "up",
				"rx_packets": float64(500),
				"tx_packets": float64(600),
				"rx_errors":  float64(10),
				"tx_errors":  float64(5),
			},
			contains: []string{
				"[UP] eth2 (vlan)",
				"RX:         500 packets",
				"[10 errors]",
				"TX:         600 packets",
				"[5 errors]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				printInterface(tt.iface)
			})

			for _, expected := range tt.contains {
				assert.Contains(t, output, expected)
			}
		})
	}
}

// TestPrintPluginStatus tests compact plugin status printing
func TestPrintPluginStatus(t *testing.T) {
	tests := []struct {
		name     string
		namespace string
		status   interface{}
		contains []string
	}{
		{
			name:      "enabled plugin with provider",
			namespace: "firewall",
			status: map[string]interface{}{
				"enabled":  true,
				"provider": "nftables",
			},
			contains: []string{
				"[UP]",
				"firewall",
				"(nftables)",
			},
		},
		{
			name:      "disabled plugin",
			namespace: "vpn",
			status: map[string]interface{}{
				"enabled": false,
			},
			contains: []string{
				"[DOWN]",
				"vpn",
			},
		},
		{
			name:      "dhcp plugin with leases",
			namespace: "dhcp",
			status: map[string]interface{}{
				"enabled":     true,
				"lease_count": float64(15),
			},
			contains: []string{
				"[UP]",
				"dhcp",
				"15 leases",
			},
		},
		{
			name:      "firewall plugin with rules",
			namespace: "firewall",
			status: map[string]interface{}{
				"running":    true,
				"rule_count": float64(42),
			},
			contains: []string{
				"[UP]",
				"firewall",
				"42 rules",
			},
		},
		{
			name:      "invalid status type",
			namespace: "test",
			status:    "not a map",
			contains:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				printPluginStatus(tt.namespace, tt.status)
			})

			for _, expected := range tt.contains {
				assert.Contains(t, output, expected)
			}
		})
	}
}

// TestPrintPluginStatusVerbose tests verbose plugin status printing
func TestPrintPluginStatusVerbose(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		status    interface{}
		contains  []string
		notContains []string
	}{
		{
			name:      "plugin with various field types",
			namespace: "firewall",
			status: map[string]interface{}{
				"enabled":    true,
				"rule_count": float64(25),
				"provider":   "nftables",
			},
			contains: []string{
				"firewall",
				"Enabled:",
				"Active",
				"Rule Count:",
				"25",
				"Provider:",
				"nftables",
			},
		},
		{
			name:      "monitoring plugin skips complex fields",
			namespace: "monitoring",
			status: map[string]interface{}{
				"enabled":          true,
				"system_metrics":   map[string]interface{}{"cpu": 50},
				"interface_metrics": []interface{}{},
			},
			contains: []string{
				"monitoring",
				"Enabled:",
				"Active",
			},
			notContains: []string{
				"System Metrics",
				"Interface Metrics",
			},
		},
		{
			name:      "invalid status type",
			namespace: "test",
			status:    "not a map",
			contains:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				printPluginStatusVerbose(tt.namespace, tt.status)
			})

			for _, expected := range tt.contains {
				assert.Contains(t, output, expected)
			}

			for _, notExpected := range tt.notContains {
				assert.NotContains(t, output, notExpected)
			}
		})
	}
}

// TestPrintCompactStatus tests compact status display
func TestPrintCompactStatus(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		contains []string
	}{
		{
			name: "running daemon with interfaces",
			data: map[string]interface{}{
				"daemon": map[string]interface{}{
					"running": true,
					"pid":     float64(1234),
				},
				"system": map[string]interface{}{
					"hostname": "router",
					"uptime":   "2 days",
				},
				"pending":       false,
				"ip_forwarding": true,
				"interfaces": []interface{}{
					map[string]interface{}{
						"state": "up",
					},
					map[string]interface{}{
						"state": "up",
					},
					map[string]interface{}{
						"state": "down",
					},
				},
			},
			contains: []string{
				"Jack Network Configuration Daemon",
				"[OK] Daemon:     Running (PID: 1234)",
				"Hostname:   router",
				"Uptime:     2 days",
				"[OK] Configuration:   No pending changes",
				"[OK] IP Forwarding: Enabled",
				"Interfaces: 2 up, 1 down",
			},
		},
		{
			name: "daemon down with pending changes",
			data: map[string]interface{}{
				"daemon": map[string]interface{}{
					"running": false,
				},
				"pending":       true,
				"ip_forwarding": false,
			},
			contains: []string{
				"[DOWN] Daemon:   Not running",
				"[WARN] Configuration: Pending changes",
				"[INFO] IP Forwarding: Disabled",
			},
		},
		{
			name: "with plugins",
			data: map[string]interface{}{
				"daemon": map[string]interface{}{
					"running": true,
					"pid":     float64(5678),
				},
				"plugins": map[string]interface{}{
					"firewall": map[string]interface{}{
						"enabled": true,
					},
					"dhcp": map[string]interface{}{
						"enabled": false,
					},
				},
			},
			contains: []string{
				"Plugins:",
				"firewall",
				"dhcp",
			},
		},
		{
			name: "invalid data type",
			data: "not a map",
			contains: []string{
				"Unable to parse status data",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				printCompactStatus(tt.data)
			})

			for _, expected := range tt.contains {
				assert.Contains(t, output, expected)
			}
		})
	}
}

// TestPrintVerboseStatus tests verbose status display
func TestPrintVerboseStatus(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		contains []string
	}{
		{
			name: "complete system status",
			data: map[string]interface{}{
				"daemon": map[string]interface{}{
					"running":     true,
					"pid":         float64(1234),
					"uptime":      "1h 30m",
					"config_path": "/etc/jack",
				},
				"system": map[string]interface{}{
					"hostname":       "router",
					"kernel_version": "5.15.0",
					"uptime":         "2 days",
				},
				"pending":       true,
				"ip_forwarding": true,
				"interfaces": []interface{}{
					map[string]interface{}{
						"name":  "eth0",
						"type":  "physical",
						"state": "up",
					},
				},
				"plugins": map[string]interface{}{
					"firewall": map[string]interface{}{
						"enabled": true,
					},
				},
			},
			contains: []string{
				"Jack Network Configuration Daemon - Detailed Status",
				"DAEMON",
				"Status:      Active",
				"PID:         1234",
				"Uptime:      1h 30m",
				"Config Path: /etc/jack",
				"SYSTEM",
				"Hostname:       router",
				"Kernel:         5.15.0",
				"System Uptime:  2 days",
				"CONFIGURATION",
				"Pending Changes: Yes",
				"IP FORWARDING",
				"Status: Active",
				"PLUGINS",
				"firewall",
				"INTERFACES",
			},
		},
		{
			name: "daemon inactive",
			data: map[string]interface{}{
				"daemon": map[string]interface{}{
					"running":     false,
					"config_path": "/etc/jack",
				},
				"pending":       false,
				"ip_forwarding": false,
			},
			contains: []string{
				"Status:      Inactive",
				"Pending Changes: No",
				"Status: Inactive",
			},
		},
		{
			name: "invalid data type",
			data: "not a map",
			contains: []string{
				"Unable to parse status data",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureOutput(func() {
				printVerboseStatus(tt.data)
			})

			for _, expected := range tt.contains {
				assert.Contains(t, output, expected)
			}
		})
	}
}

// BenchmarkFormatBytes benchmarks byte formatting
func BenchmarkFormatBytes(b *testing.B) {
	sizes := []int64{
		512,
		1536,
		1572864,
		1610612736,
	}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = formatBytes(size)
			}
		})
	}
}

// BenchmarkToTitleCase benchmarks title case conversion
func BenchmarkToTitleCase(b *testing.B) {
	inputs := []string{
		"hello world",
		"the quick brown fox",
		"a very long string with many words to convert",
	}

	for _, input := range inputs {
		b.Run(strings.ReplaceAll(input, " ", "_"), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = toTitleCase(input)
			}
		})
	}
}
