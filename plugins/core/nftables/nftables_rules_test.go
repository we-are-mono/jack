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

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePortForwardDNATRule(t *testing.T) {
	tests := []struct {
		name        string
		portFwd     PortForward
		wantContain []string
	}{
		{
			name: "simple TCP port forward",
			portFwd: PortForward{
				Name:     "web",
				Proto:    "tcp",
				SrcDPort: "80",
				DestIP:   "192.168.1.100",
				DestPort: "8080",
				Comment:  "Web server",
			},
			wantContain: []string{"meta l4proto tcp", "th dport 80", "dnat to 192.168.1.100:8080", "comment \"Web server\""},
		},
		{
			name: "port forward same source and dest port",
			portFwd: PortForward{
				Name:     "ssh",
				Proto:    "tcp",
				SrcDPort: "22",
				DestIP:   "192.168.1.10",
				Comment:  "SSH",
			},
			wantContain: []string{"meta l4proto tcp", "th dport 22", "dnat to 192.168.1.10", "comment \"SSH\""},
		},
		{
			name: "UDP port forward",
			portFwd: PortForward{
				Name:     "dns",
				Proto:    "udp",
				SrcDPort: "53",
				DestIP:   "10.0.0.53",
				Comment:  "DNS server",
			},
			wantContain: []string{"meta l4proto udp", "th dport 53", "dnat to 10.0.0.53", "comment \"DNS server\""},
		},
		{
			name: "no protocol specified",
			portFwd: PortForward{
				Name:     "any-proto",
				SrcDPort: "8080",
				DestIP:   "192.168.1.1",
				Comment:  "Test",
			},
			wantContain: []string{"th dport 8080", "dnat to 192.168.1.1", "comment \"Test\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GeneratePortForwardDNATRule(tt.portFwd)

			for _, want := range tt.wantContain {
				assert.Contains(t, result, want, "GeneratePortForwardDNATRule()")
			}

			// Verify rule has counter
			assert.Contains(t, result, "counter", "GeneratePortForwardDNATRule() should contain counter")
		})
	}
}

func TestGeneratePortForwardFilterRule(t *testing.T) {
	tests := []struct {
		name        string
		portFwd     PortForward
		wantContain []string
	}{
		{
			name: "filter rule with dest port",
			portFwd: PortForward{
				Name:     "web",
				Proto:    "tcp",
				SrcDPort: "80",
				DestIP:   "192.168.1.100",
				DestPort: "8080",
			},
			wantContain: []string{"ip daddr 192.168.1.100", "meta l4proto tcp", "th dport 8080", "counter accept", "Allow-PortForward-web"},
		},
		{
			name: "filter rule without dest port uses src port",
			portFwd: PortForward{
				Name:     "ssh",
				Proto:    "tcp",
				SrcDPort: "22",
				DestIP:   "192.168.1.10",
			},
			wantContain: []string{"ip daddr 192.168.1.10", "meta l4proto tcp", "th dport 22", "counter accept", "Allow-PortForward-ssh"},
		},
		{
			name: "filter rule no protocol",
			portFwd: PortForward{
				Name:     "any",
				SrcDPort: "9000",
				DestIP:   "10.0.0.1",
			},
			wantContain: []string{"ip daddr 10.0.0.1", "th dport 9000", "counter accept"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GeneratePortForwardFilterRule(tt.portFwd)

			for _, want := range tt.wantContain {
				assert.Contains(t, result, want, "GeneratePortForwardFilterRule()")
			}
		})
	}
}

func TestGenerateCustomRule(t *testing.T) {
	tests := []struct {
		name        string
		rule        Rule
		wantContain []string
	}{
		{
			name: "allow SSH",
			rule: Rule{
				Name:     "Allow-SSH",
				Proto:    "tcp",
				DestPort: "22",
				Target:   "ACCEPT",
			},
			wantContain: []string{"meta l4proto tcp", "th dport 22", "counter accept", "comment \"Allow-SSH\""},
		},
		{
			name: "drop ICMP",
			rule: Rule{
				Name:   "Drop-ICMP",
				Proto:  "icmp",
				Target: "DROP",
			},
			wantContain: []string{"meta l4proto icmp", "counter drop", "comment \"Drop-ICMP\""},
		},
		{
			name: "allow established/related",
			rule: Rule{
				Name:   "Allow-Established-Related",
				Target: "ACCEPT",
			},
			wantContain: []string{"ct state established,related", "counter accept"},
		},
		{
			name: "allow DNS UDP",
			rule: Rule{
				Name:     "Allow-DNS",
				Proto:    "udp",
				DestPort: "53",
				Target:   "ACCEPT",
			},
			wantContain: []string{"meta l4proto udp", "th dport 53", "counter accept"},
		},
		{
			name: "rule with source port",
			rule: Rule{
				Name:    "Allow-High-Ports",
				Proto:   "tcp",
				SrcPort: "1024-65535",
				Target:  "ACCEPT",
			},
			wantContain: []string{"meta l4proto tcp", "th sport 1024-65535", "counter accept"},
		},
		{
			name: "reject rule",
			rule: Rule{
				Name:   "Reject-Invalid",
				Target: "REJECT",
			},
			wantContain: []string{"counter reject"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateCustomRule(tt.rule)

			for _, want := range tt.wantContain {
				assert.Contains(t, result, want, "GenerateCustomRule()")
			}
		})
	}
}

func TestValidateFirewallConfig(t *testing.T) {
	tests := []struct {
		name       string
		config     *FirewallConfig
		wantError  bool
		errContain string
	}{
		{
			name: "valid basic config",
			config: &FirewallConfig{
				Zones: map[string]Zone{
					"lan": {
						Interfaces: []string{"br-lan"},
						Input:      "ACCEPT",
						Forward:    "ACCEPT",
					},
					"wan": {
						Interfaces: []string{"eth0"},
						Input:      "DROP",
						Forward:    "DROP",
					},
				},
			},
			wantError: false,
		},
		{
			name:       "nil config",
			config:     nil,
			wantError:  true,
			errContain: "config is nil",
		},
		{
			name: "no zones",
			config: &FirewallConfig{
				Zones: map[string]Zone{},
			},
			wantError:  true,
			errContain: "no zones defined",
		},
		{
			name: "zone with no interfaces",
			config: &FirewallConfig{
				Zones: map[string]Zone{
					"lan": {
						Interfaces: []string{},
						Input:      "ACCEPT",
						Forward:    "ACCEPT",
					},
				},
			},
			wantError:  true,
			errContain: "has no interfaces",
		},
		{
			name: "invalid input policy",
			config: &FirewallConfig{
				Zones: map[string]Zone{
					"lan": {
						Interfaces: []string{"br-lan"},
						Input:      "INVALID",
						Forward:    "ACCEPT",
					},
				},
			},
			wantError:  true,
			errContain: "invalid input policy",
		},
		{
			name: "invalid forward policy",
			config: &FirewallConfig{
				Zones: map[string]Zone{
					"lan": {
						Interfaces: []string{"br-lan"},
						Input:      "ACCEPT",
						Forward:    "INVALID",
					},
				},
			},
			wantError:  true,
			errContain: "invalid forward policy",
		},
		{
			name: "forwarding references non-existent source zone",
			config: &FirewallConfig{
				Zones: map[string]Zone{
					"lan": {Interfaces: []string{"br-lan"}, Input: "ACCEPT", Forward: "ACCEPT"},
				},
				Forwardings: []Forwarding{
					{Src: "nonexistent", Dest: "lan"},
				},
			},
			wantError:  true,
			errContain: "non-existent source zone",
		},
		{
			name: "forwarding references non-existent dest zone",
			config: &FirewallConfig{
				Zones: map[string]Zone{
					"lan": {Interfaces: []string{"br-lan"}, Input: "ACCEPT", Forward: "ACCEPT"},
				},
				Forwardings: []Forwarding{
					{Src: "lan", Dest: "nonexistent"},
				},
			},
			wantError:  true,
			errContain: "non-existent dest zone",
		},
		{
			name: "rule references non-existent zone",
			config: &FirewallConfig{
				Zones: map[string]Zone{
					"lan": {Interfaces: []string{"br-lan"}, Input: "ACCEPT", Forward: "ACCEPT"},
				},
				Rules: []Rule{
					{Name: "test", Src: "nonexistent", Target: "ACCEPT"},
				},
			},
			wantError:  true,
			errContain: "non-existent source zone",
		},
		{
			name: "rule with invalid target",
			config: &FirewallConfig{
				Zones: map[string]Zone{
					"lan": {Interfaces: []string{"br-lan"}, Input: "ACCEPT", Forward: "ACCEPT"},
				},
				Rules: []Rule{
					{Name: "test", Src: "lan", Target: "INVALID"},
				},
			},
			wantError:  true,
			errContain: "invalid target",
		},
		{
			name: "rule with invalid protocol",
			config: &FirewallConfig{
				Zones: map[string]Zone{
					"lan": {Interfaces: []string{"br-lan"}, Input: "ACCEPT", Forward: "ACCEPT"},
				},
				Rules: []Rule{
					{Name: "test", Src: "lan", Target: "ACCEPT", Proto: "invalid"},
				},
			},
			wantError:  true,
			errContain: "invalid protocol",
		},
		{
			name: "port forward missing dest IP",
			config: &FirewallConfig{
				Zones: map[string]Zone{
					"lan": {Interfaces: []string{"br-lan"}, Input: "ACCEPT", Forward: "ACCEPT"},
				},
				PortForwards: []PortForward{
					{Name: "test", SrcDPort: "80"},
				},
			},
			wantError:  true,
			errContain: "missing destination IP",
		},
		{
			name: "port forward missing source port",
			config: &FirewallConfig{
				Zones: map[string]Zone{
					"lan": {Interfaces: []string{"br-lan"}, Input: "ACCEPT", Forward: "ACCEPT"},
				},
				PortForwards: []PortForward{
					{Name: "test", DestIP: "192.168.1.1"},
				},
			},
			wantError:  true,
			errContain: "missing source port",
		},
		{
			name: "port forward with invalid protocol",
			config: &FirewallConfig{
				Zones: map[string]Zone{
					"lan": {Interfaces: []string{"br-lan"}, Input: "ACCEPT", Forward: "ACCEPT"},
				},
				PortForwards: []PortForward{
					{Name: "test", DestIP: "192.168.1.1", SrcDPort: "80", Proto: "icmp"},
				},
			},
			wantError:  true,
			errContain: "invalid protocol",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFirewallConfig(tt.config)

			if tt.wantError {
				require.Error(t, err, "ValidateFirewallConfig() expected error")
				if tt.errContain != "" {
					assert.Contains(t, err.Error(), tt.errContain)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
