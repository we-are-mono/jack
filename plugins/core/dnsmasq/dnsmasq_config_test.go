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
	"strings"
	"testing"

	"github.com/we-are-mono/jack/types"
)

func TestValidateDHCPConfig(t *testing.T) {
	tests := []struct {
		name       string
		config     *DHCPConfig
		wantError  bool
		errContain string
	}{
		{
			name: "valid config",
			config: &DHCPConfig{
				Server: DHCPServer{Enabled: true},
				DHCPPools: map[string]DHCPPool{
					"lan": {
						Interface: "br-lan",
						Start:     100,
						Limit:     150,
						LeaseTime: "12h",
						DNS:       []string{"8.8.8.8", "1.1.1.1"},
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
			name: "DHCP enabled but no pools",
			config: &DHCPConfig{
				Server:    DHCPServer{Enabled: true},
				DHCPPools: map[string]DHCPPool{},
			},
			wantError:  true,
			errContain: "no pools defined",
		},
		{
			name: "DHCP disabled with no pools is valid",
			config: &DHCPConfig{
				Server:    DHCPServer{Enabled: false},
				DHCPPools: map[string]DHCPPool{},
			},
			wantError: false,
		},
		{
			name: "pool missing interface",
			config: &DHCPConfig{
				Server: DHCPServer{Enabled: true},
				DHCPPools: map[string]DHCPPool{
					"lan": {
						Start: 100,
						Limit: 50,
					},
				},
			},
			wantError:  true,
			errContain: "interface is required",
		},
		{
			name: "pool start too low",
			config: &DHCPConfig{
				Server: DHCPServer{Enabled: true},
				DHCPPools: map[string]DHCPPool{
					"lan": {
						Interface: "br-lan",
						Start:     -1,
						Limit:     50,
					},
				},
			},
			wantError:  true,
			errContain: "invalid start address",
		},
		{
			name: "pool start too high",
			config: &DHCPConfig{
				Server: DHCPServer{Enabled: true},
				DHCPPools: map[string]DHCPPool{
					"lan": {
						Interface: "br-lan",
						Start:     256,
						Limit:     50,
					},
				},
			},
			wantError:  true,
			errContain: "invalid start address",
		},
		{
			name: "pool limit too low",
			config: &DHCPConfig{
				Server: DHCPServer{Enabled: true},
				DHCPPools: map[string]DHCPPool{
					"lan": {
						Interface: "br-lan",
						Start:     100,
						Limit:     0,
					},
				},
			},
			wantError:  true,
			errContain: "invalid limit",
		},
		{
			name: "pool limit too high",
			config: &DHCPConfig{
				Server: DHCPServer{Enabled: true},
				DHCPPools: map[string]DHCPPool{
					"lan": {
						Interface: "br-lan",
						Start:     100,
						Limit:     256,
					},
				},
			},
			wantError:  true,
			errContain: "invalid limit",
		},
		{
			name: "pool start + limit exceeds 255",
			config: &DHCPConfig{
				Server: DHCPServer{Enabled: true},
				DHCPPools: map[string]DHCPPool{
					"lan": {
						Interface: "br-lan",
						Start:     200,
						Limit:     100,
					},
				},
			},
			wantError:  true,
			errContain: "exceeds 255",
		},
		{
			name: "invalid lease time format",
			config: &DHCPConfig{
				Server: DHCPServer{Enabled: true},
				DHCPPools: map[string]DHCPPool{
					"lan": {
						Interface: "br-lan",
						Start:     100,
						Limit:     50,
						LeaseTime: "invalid",
					},
				},
			},
			wantError:  true,
			errContain: "invalid lease time format",
		},
		{
			name: "valid lease time formats",
			config: &DHCPConfig{
				Server: DHCPServer{Enabled: true},
				DHCPPools: map[string]DHCPPool{
					"lan1": {
						Interface: "br-lan",
						Start:     100,
						Limit:     50,
						LeaseTime: "12h",
					},
					"lan2": {
						Interface: "br-lan2",
						Start:     100,
						Limit:     50,
						LeaseTime: "7d",
					},
					"lan3": {
						Interface: "br-lan3",
						Start:     100,
						Limit:     50,
						LeaseTime: "30m",
					},
				},
			},
			wantError: false,
		},
		{
			name: "invalid DNS server IP",
			config: &DHCPConfig{
				Server: DHCPServer{Enabled: true},
				DHCPPools: map[string]DHCPPool{
					"lan": {
						Interface: "br-lan",
						Start:     100,
						Limit:     50,
						DNS:       []string{"8.8.8.8", "invalid-ip"},
					},
				},
			},
			wantError:  true,
			errContain: "invalid DNS server IP",
		},
		{
			name: "static lease missing MAC",
			config: &DHCPConfig{
				Server: DHCPServer{Enabled: true},
				DHCPPools: map[string]DHCPPool{
					"lan": {Interface: "br-lan", Start: 100, Limit: 50},
				},
				StaticLeases: []StaticLease{
					{IP: "192.168.1.100"},
				},
			},
			wantError:  true,
			errContain: "MAC address is required",
		},
		{
			name: "static lease invalid MAC",
			config: &DHCPConfig{
				Server: DHCPServer{Enabled: true},
				DHCPPools: map[string]DHCPPool{
					"lan": {Interface: "br-lan", Start: 100, Limit: 50},
				},
				StaticLeases: []StaticLease{
					{MAC: "invalid-mac", IP: "192.168.1.100"},
				},
			},
			wantError:  true,
			errContain: "invalid MAC address format",
		},
		{
			name: "static lease missing IP",
			config: &DHCPConfig{
				Server: DHCPServer{Enabled: true},
				DHCPPools: map[string]DHCPPool{
					"lan": {Interface: "br-lan", Start: 100, Limit: 50},
				},
				StaticLeases: []StaticLease{
					{MAC: "aa:bb:cc:dd:ee:ff"},
				},
			},
			wantError:  true,
			errContain: "IP address is required",
		},
		{
			name: "static lease invalid IP",
			config: &DHCPConfig{
				Server: DHCPServer{Enabled: true},
				DHCPPools: map[string]DHCPPool{
					"lan": {Interface: "br-lan", Start: 100, Limit: 50},
				},
				StaticLeases: []StaticLease{
					{MAC: "aa:bb:cc:dd:ee:ff", IP: "invalid-ip"},
				},
			},
			wantError:  true,
			errContain: "invalid IP address",
		},
		{
			name: "valid static lease",
			config: &DHCPConfig{
				Server: DHCPServer{Enabled: true},
				DHCPPools: map[string]DHCPPool{
					"lan": {Interface: "br-lan", Start: 100, Limit: 50},
				},
				StaticLeases: []StaticLease{
					{MAC: "aa:bb:cc:dd:ee:ff", IP: "192.168.1.100", Name: "server1"},
					{MAC: "11-22-33-44-55-66", IP: "192.168.1.101", Name: "server2"},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDHCPConfig(tt.config)

			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateDHCPConfig() expected error, got nil")
				} else if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("ValidateDHCPConfig() error = %q, want to contain %q", err.Error(), tt.errContain)
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateDHCPConfig() unexpected error: %v", err)
			}
		})
	}
}

func TestValidateDNSConfig(t *testing.T) {
	tests := []struct {
		name       string
		config     *DNSConfig
		wantError  bool
		errContain string
	}{
		{
			name: "valid config",
			config: &DNSConfig{
				Server: DNSServer{
					Enabled:   true,
					Port:      53,
					Upstreams: []string{"8.8.8.8", "1.1.1.1"},
				},
				Records: DNSRecords{
					ARecords: []ARecord{
						{Name: "server", IP: "192.168.1.100"},
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
			name: "invalid port negative",
			config: &DNSConfig{
				Server: DNSServer{Port: -1},
			},
			wantError:  true,
			errContain: "invalid DNS port",
		},
		{
			name: "invalid port too high",
			config: &DNSConfig{
				Server: DNSServer{Port: 65536},
			},
			wantError:  true,
			errContain: "invalid DNS port",
		},
		{
			name: "invalid upstream DNS",
			config: &DNSConfig{
				Server: DNSServer{
					Upstreams: []string{"8.8.8.8", "invalid-ip"},
				},
			},
			wantError:  true,
			errContain: "invalid upstream DNS server",
		},
		{
			name: "A record missing name",
			config: &DNSConfig{
				Records: DNSRecords{
					ARecords: []ARecord{
						{IP: "192.168.1.100"},
					},
				},
			},
			wantError:  true,
			errContain: "name is required",
		},
		{
			name: "A record invalid IP",
			config: &DNSConfig{
				Records: DNSRecords{
					ARecords: []ARecord{
						{Name: "server", IP: "invalid-ip"},
					},
				},
			},
			wantError:  true,
			errContain: "invalid IPv4 address",
		},
		{
			name: "AAAA record missing name",
			config: &DNSConfig{
				Records: DNSRecords{
					AAAARecords: []AAAARecord{
						{IP: "2001:db8::1"},
					},
				},
			},
			wantError:  true,
			errContain: "name is required",
		},
		{
			name: "AAAA record invalid IP",
			config: &DNSConfig{
				Records: DNSRecords{
					AAAARecords: []AAAARecord{
						{Name: "server", IP: "192.168.1.1"},
					},
				},
			},
			wantError:  true,
			errContain: "invalid IPv6 address",
		},
		{
			name: "CNAME record missing cname",
			config: &DNSConfig{
				Records: DNSRecords{
					CNAMERecords: []CNAMERecord{
						{Target: "server.local"},
					},
				},
			},
			wantError:  true,
			errContain: "cname is required",
		},
		{
			name: "CNAME record missing target",
			config: &DNSConfig{
				Records: DNSRecords{
					CNAMERecords: []CNAMERecord{
						{CNAME: "alias"},
					},
				},
			},
			wantError:  true,
			errContain: "target is required",
		},
		{
			name: "MX record missing domain",
			config: &DNSConfig{
				Records: DNSRecords{
					MXRecords: []MXRecord{
						{Server: "mail.example.com", Priority: 10},
					},
				},
			},
			wantError:  true,
			errContain: "domain is required",
		},
		{
			name: "MX record missing server",
			config: &DNSConfig{
				Records: DNSRecords{
					MXRecords: []MXRecord{
						{Domain: "example.com", Priority: 10},
					},
				},
			},
			wantError:  true,
			errContain: "server is required",
		},
		{
			name: "MX record negative priority",
			config: &DNSConfig{
				Records: DNSRecords{
					MXRecords: []MXRecord{
						{Domain: "example.com", Server: "mail.example.com", Priority: -1},
					},
				},
			},
			wantError:  true,
			errContain: "priority cannot be negative",
		},
		{
			name: "SRV record missing service",
			config: &DNSConfig{
				Records: DNSRecords{
					SRVRecords: []SRVRecord{
						{Proto: "tcp", Target: "server.local", Port: 80},
					},
				},
			},
			wantError:  true,
			errContain: "service is required",
		},
		{
			name: "SRV record missing proto",
			config: &DNSConfig{
				Records: DNSRecords{
					SRVRecords: []SRVRecord{
						{Service: "http", Target: "server.local", Port: 80},
					},
				},
			},
			wantError:  true,
			errContain: "proto is required",
		},
		{
			name: "SRV record missing target",
			config: &DNSConfig{
				Records: DNSRecords{
					SRVRecords: []SRVRecord{
						{Service: "http", Proto: "tcp", Port: 80},
					},
				},
			},
			wantError:  true,
			errContain: "target is required",
		},
		{
			name: "SRV record invalid port",
			config: &DNSConfig{
				Records: DNSRecords{
					SRVRecords: []SRVRecord{
						{Service: "http", Proto: "tcp", Target: "server.local", Port: 0},
					},
				},
			},
			wantError:  true,
			errContain: "invalid port",
		},
		{
			name: "PTR record missing IP",
			config: &DNSConfig{
				Records: DNSRecords{
					PTRRecords: []PTRRecord{
						{Name: "server.local"},
					},
				},
			},
			wantError:  true,
			errContain: "IP is required",
		},
		{
			name: "PTR record invalid IP",
			config: &DNSConfig{
				Records: DNSRecords{
					PTRRecords: []PTRRecord{
						{IP: "invalid-ip", Name: "server.local"},
					},
				},
			},
			wantError:  true,
			errContain: "invalid IP address",
		},
		{
			name: "PTR record missing name",
			config: &DNSConfig{
				Records: DNSRecords{
					PTRRecords: []PTRRecord{
						{IP: "192.168.1.100"},
					},
				},
			},
			wantError:  true,
			errContain: "name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDNSConfig(tt.config)

			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateDNSConfig() expected error, got nil")
				} else if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("ValidateDNSConfig() error = %q, want to contain %q", err.Error(), tt.errContain)
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateDNSConfig() unexpected error: %v", err)
			}
		})
	}
}

func TestNetworkPortion(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want string
	}{
		{
			name: "standard IP",
			ip:   "192.168.1.1",
			want: "192.168.1",
		},
		{
			name: "class B",
			ip:   "172.16.0.1",
			want: "172.16.0",
		},
		{
			name: "class A",
			ip:   "10.0.0.1",
			want: "10.0.0",
		},
		{
			name: "no dots",
			ip:   "localhost",
			want: "localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NetworkPortion(tt.ip)
			if result != tt.want {
				t.Errorf("NetworkPortion(%q) = %q, want %q", tt.ip, result, tt.want)
			}
		})
	}
}

func TestIsValidLeaseTime(t *testing.T) {
	tests := []struct {
		name      string
		leaseTime string
		want      bool
	}{
		{"1 hour", "1h", true},
		{"12 hours", "12h", true},
		{"24 hours", "24h", true},
		{"7 days", "7d", true},
		{"30 minutes", "30m", true},
		{"1 week", "1w", true},
		{"invalid no unit", "12", false},
		{"invalid format", "12hours", false},
		{"invalid unit", "12x", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidLeaseTime(tt.leaseTime)
			if result != tt.want {
				t.Errorf("isValidLeaseTime(%q) = %v, want %v", tt.leaseTime, result, tt.want)
			}
		})
	}
}

func TestIsValidMAC(t *testing.T) {
	tests := []struct {
		name string
		mac  string
		want bool
	}{
		{"colon format lowercase", "aa:bb:cc:dd:ee:ff", true},
		{"colon format uppercase", "AA:BB:CC:DD:EE:FF", true},
		{"colon format mixed", "aA:bB:cC:dD:eE:fF", true},
		{"hyphen format", "aa-bb-cc-dd-ee-ff", true},
		{"hyphen format uppercase", "AA-BB-CC-DD-EE-FF", true},
		{"invalid too short", "aa:bb:cc:dd:ee", false},
		{"invalid too long", "aa:bb:cc:dd:ee:ff:11", false},
		{"invalid no separator", "aabbccddeeff", false},
		{"invalid mixed separators", "aa:bb-cc:dd:ee:ff", false},
		{"invalid characters", "gg:hh:ii:jj:kk:ll", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidMAC(tt.mac)
			if result != tt.want {
				t.Errorf("isValidMAC(%q) = %v, want %v", tt.mac, result, tt.want)
			}
		})
	}
}

func TestGetInterfaceIPsFromConfig(t *testing.T) {
	config := types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"br-lan": {
				Type:       "bridge",
				IPAddr:     "192.168.1.1",
				DeviceName: "br-lan",
			},
			"eth0": {
				Type:   "physical",
				Device: "eth0",
				IPAddr: "10.0.0.1",
			},
			"eth1": {
				Type:   "physical",
				Device: "eth1",
				// No IP address
			},
		},
	}

	result := GetInterfaceIPsFromConfig(config)

	// Check expected IPs are present
	if result["br-lan"] != "192.168.1.1" {
		t.Errorf("Expected br-lan IP to be 192.168.1.1, got %s", result["br-lan"])
	}

	if result["eth0"] != "10.0.0.1" {
		t.Errorf("Expected eth0 IP to be 10.0.0.1, got %s", result["eth0"])
	}

	// Check interface without IP is not in the map
	if _, exists := result["eth1"]; exists {
		t.Errorf("Expected eth1 to not be in result map")
	}
}

func TestGetInterfaceDevicesFromConfig(t *testing.T) {
	config := types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"br-lan": {
				Type:       "bridge",
				DeviceName: "br-lan",
				IPAddr:     "192.168.1.1",
			},
			"eth0": {
				Type:   "physical",
				Device: "eth0",
				IPAddr: "10.0.0.1",
			},
			"vlan10": {
				Type:   "vlan",
				Device: "eth0.10",
				// VLAN interfaces should not be included
			},
		},
	}

	result := GetInterfaceDevicesFromConfig(config)

	// Check bridge device is present
	if result["br-lan"] != "br-lan" {
		t.Errorf("Expected br-lan device to be br-lan, got %s", result["br-lan"])
	}

	// Check physical device is present
	if result["eth0"] != "eth0" {
		t.Errorf("Expected eth0 device to be eth0, got %s", result["eth0"])
	}

	// Check VLAN interface is not in the map
	if _, exists := result["vlan10"]; exists {
		t.Errorf("Expected vlan10 to not be in result map")
	}
}
