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

package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name      string
		port      int
		wantError bool
	}{
		{"valid port 80", 80, false},
		{"valid port 1", 1, false},
		{"valid port 65535", 65535, false},
		{"valid port 8080", 8080, false},
		{"port too low", 0, true},
		{"port negative", -1, true},
		{"port too high", 65536, true},
		{"port way too high", 100000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePort(tt.port)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePortString(t *testing.T) {
	tests := []struct {
		name      string
		portStr   string
		wantError bool
	}{
		{"single port", "80", false},
		{"single high port", "8080", false},
		{"valid range", "8080-8090", false},
		{"valid wide range", "1024-65535", false},
		{"empty string", "", true},
		{"invalid number", "abc", true},
		{"invalid range start", "abc-8090", true},
		{"invalid range end", "8080-xyz", true},
		{"start equals end", "8080-8080", true},
		{"start greater than end", "8090-8080", true},
		{"port 0", "0", true},
		{"negative port", "-1", true},
		{"port too high", "65536", true},
		{"range with invalid start", "0-100", true},
		{"range with invalid end", "100-65536", true},
		{"multiple hyphens", "80-90-100", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePortString(tt.portStr)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateIP(t *testing.T) {
	tests := []struct {
		name      string
		ip        string
		wantError bool
	}{
		{"valid IPv4", "192.168.1.1", false},
		{"valid IPv4 zero", "0.0.0.0", false},
		{"valid IPv6", "2001:db8::1", false},
		{"valid IPv6 loopback", "::1", false},
		{"empty string", "", true},
		{"invalid format", "not.an.ip", true},
		{"invalid IPv4", "256.1.1.1", true},
		{"invalid IPv4 segments", "192.168", true},
		{"invalid IPv6", "gggg::1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIP(tt.ip)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCIDR(t *testing.T) {
	tests := []struct {
		name      string
		cidr      string
		wantError bool
	}{
		{"valid CIDR /24", "192.168.1.0/24", false},
		{"valid CIDR /32", "10.0.0.1/32", false},
		{"valid CIDR /0", "0.0.0.0/0", false},
		{"special default", "default", false},
		{"valid IPv6 CIDR", "2001:db8::/32", false},
		{"empty string", "", true},
		{"missing prefix", "192.168.1.0", true},
		{"invalid prefix", "192.168.1.0/33", true},
		{"invalid IP", "256.1.1.1/24", true},
		{"just slash", "/24", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCIDR(tt.cidr)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePolicy(t *testing.T) {
	tests := []struct {
		name      string
		policy    string
		wantError bool
	}{
		{"ACCEPT uppercase", "ACCEPT", false},
		{"DROP uppercase", "DROP", false},
		{"REJECT uppercase", "REJECT", false},
		{"accept lowercase", "accept", false},
		{"drop lowercase", "drop", false},
		{"reject lowercase", "reject", false},
		{"Accept mixed case", "Accept", false},
		{"empty string", "", true},
		{"invalid policy", "ALLOW", true},
		{"invalid policy DENY", "DENY", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePolicy(tt.policy)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateProtocol(t *testing.T) {
	tcpUdp := []string{"tcp", "udp"}
	allProtos := []string{"tcp", "udp", "icmp", "icmpv6"}

	tests := []struct {
		name      string
		proto     string
		allowed   []string
		wantError bool
	}{
		{"tcp in tcp/udp", "tcp", tcpUdp, false},
		{"udp in tcp/udp", "udp", tcpUdp, false},
		{"icmp in all", "icmp", allProtos, false},
		{"empty proto", "", tcpUdp, false}, // Empty is often optional
		{"tcp in all", "tcp", allProtos, false},
		{"icmp in tcp/udp only", "icmp", tcpUdp, true},
		{"invalid proto", "sctp", tcpUdp, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProtocol(tt.proto, tt.allowed)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateDomain(t *testing.T) {
	tests := []struct {
		name      string
		domain    string
		wantError bool
	}{
		{"simple domain", "example.com", false},
		{"subdomain", "www.example.com", false},
		{"deep subdomain", "a.b.c.example.com", false},
		{"wildcard", "*.example.com", false},
		{"empty domain", "", false}, // Often optional
		{"single label", "localhost", false},
		{"hyphen in label", "my-site.example.com", false},
		{"number in label", "site123.example.com", false},
		{"too long domain", "a" + string(make([]byte, 250)) + ".com", true},
		{"label too long", string(make([]byte, 64)) + ".com", true},
		{"starts with hyphen", "-invalid.com", true},
		{"ends with hyphen", "invalid-.com", true},
		{"double dot", "example..com", true},
		{"ends with dot", "example.com.", true},
		{"special chars", "exam@ple.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDomain(tt.domain)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMAC(t *testing.T) {
	tests := []struct {
		name      string
		mac       string
		wantError bool
	}{
		{"colon format", "00:11:22:33:44:55", false},
		{"hyphen format", "00-11-22-33-44-55", false},
		{"dot format", "0011.2233.4455", false},
		{"empty mac", "", false}, // Often optional
		{"uppercase", "AA:BB:CC:DD:EE:FF", false},
		{"mixed case", "Aa:Bb:Cc:Dd:Ee:Ff", false},
		{"compact format", "001122334455", true}, // Not supported by net.ParseMAC
		{"invalid length", "00:11:22:33:44", true},
		{"invalid chars", "GG:11:22:33:44:55", true},
		{"wrong format", "00:11:22:33:44:55:66", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMAC(tt.mac)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateMTU(t *testing.T) {
	tests := []struct {
		name      string
		mtu       int
		wantError bool
	}{
		{"standard ethernet", 1500, false},
		{"minimum", 68, false},
		{"maximum", 65535, false},
		{"jumbo frame", 9000, false},
		{"too low", 67, true},
		{"zero", 0, true},
		{"negative", -1, true},
		{"too high", 65536, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMTU(tt.mtu)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateVLANID(t *testing.T) {
	tests := []struct {
		name      string
		vlanID    int
		wantError bool
	}{
		{"valid VLAN 1", 1, false},
		{"valid VLAN 100", 100, false},
		{"valid VLAN 4094", 4094, false},
		{"reserved 0", 0, true},
		{"reserved 4095", 4095, true},
		{"negative", -1, true},
		{"too high", 5000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVLANID(tt.vlanID)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateWireGuardKey(t *testing.T) {
	// Valid 32-byte key encoded as base64 (44 chars with padding)
	validKey := "YCkuRtIvpBglmQx4JQkLgGHqj8yNk9B5N1jzZqhJvAc="

	tests := []struct {
		name      string
		key       string
		wantError bool
	}{
		{"valid key", validKey, false},
		{"another valid key", "QJpJ3Nc4rlVfHTUxNHO2N2qHJlBKDTMBD7OqRJPDJUs=", false},
		{"empty key", "", true},
		{"not base64", "not-a-valid-key!!!", true},
		{"wrong length short", "dGVzdA==", true}, // "test" in base64, only 4 bytes
		{"wrong length long", "dGhpcyBpcyBhIHZlcnkgbG9uZyBrZXkgdGhhdCBpcyB0b28gbG9uZw==", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWireGuardKey(tt.key)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateEndpoint(t *testing.T) {
	tests := []struct {
		name      string
		endpoint  string
		wantError bool
	}{
		{"IP with port", "192.168.1.1:51820", false},
		{"hostname with port", "vpn.example.com:51820", false},
		{"IPv6 with port", "[2001:db8::1]:51820", false},
		{"empty endpoint", "", false}, // Often optional
		{"missing port", "192.168.1.1", true},
		{"invalid port", "192.168.1.1:abc", true},
		{"port too high", "192.168.1.1:65536", true},
		{"port zero", "192.168.1.1:0", true},
		{"invalid hostname", "invalid_host:51820", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEndpoint(tt.endpoint)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateNetmask(t *testing.T) {
	tests := []struct {
		name      string
		netmask   string
		wantError bool
	}{
		{"dotted /24", "255.255.255.0", false},
		{"dotted /16", "255.255.0.0", false},
		{"dotted /8", "255.0.0.0", false},
		{"dotted /32", "255.255.255.255", false},
		{"CIDR /24", "/24", false},
		{"CIDR /16", "/16", false},
		{"CIDR /32", "/32", false},
		{"CIDR /0", "/0", false},
		{"empty", "", true},
		{"invalid CIDR", "/33", true},
		{"invalid CIDR negative", "/-1", true},
		{"invalid dotted", "255.255.255.256", true},
		{"non-contiguous mask", "255.255.0.255", true},
		{"IPv6 address", "ffff:ffff:ffff::", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNetmask(tt.netmask)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateTableID(t *testing.T) {
	tests := []struct {
		name      string
		tableID   int
		wantError bool
	}{
		{"main table", 254, false},
		{"zero", 0, false},
		{"maximum", 4294967295, false},
		{"custom table", 100, false},
		{"negative", -1, true},
		{"exceeds maximum", 4294967296, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTableID(tt.tableID)
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateMetric(t *testing.T) {
	tests := []struct {
		name      string
		metric    int
		wantError bool
	}{
		{"zero", 0, false},
		{"positive", 100, false},
		{"large", 1000, false},
		{"negative", -1, true},
		{"very negative", -100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMetric(tt.metric)
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
