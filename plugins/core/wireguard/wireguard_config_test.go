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

func TestNetmaskToCIDR(t *testing.T) {
	tests := []struct {
		name    string
		netmask string
		want    string
	}{
		{"class C network", "255.255.255.0", "24"},
		{"class B network", "255.255.0.0", "16"},
		{"class A network", "255.0.0.0", "8"},
		{"/32 host", "255.255.255.255", "32"},
		{"/30 small subnet", "255.255.255.252", "30"},
		{"/29 subnet", "255.255.255.248", "29"},
		{"/28 subnet", "255.255.255.240", "28"},
		{"/27 subnet", "255.255.255.224", "27"},
		{"/26 subnet", "255.255.255.192", "26"},
		{"/25 subnet", "255.255.255.128", "25"},
		{"unknown netmask defaults to /24", "255.255.255.1", "24"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NetmaskToCIDR(tt.netmask)
			assert.Equal(t, tt.want, got, "NetmaskToCIDR(%q)", tt.netmask)
		})
	}
}

func TestStringSlicesEqual(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{
			name: "identical slices",
			a:    []string{"10.0.0.0/8", "192.168.0.0/16"},
			b:    []string{"10.0.0.0/8", "192.168.0.0/16"},
			want: true,
		},
		{
			name: "same elements different order",
			a:    []string{"10.0.0.0/8", "192.168.0.0/16"},
			b:    []string{"192.168.0.0/16", "10.0.0.0/8"},
			want: true,
		},
		{
			name: "different lengths",
			a:    []string{"10.0.0.0/8"},
			b:    []string{"10.0.0.0/8", "192.168.0.0/16"},
			want: false,
		},
		{
			name: "different elements",
			a:    []string{"10.0.0.0/8"},
			b:    []string{"192.168.0.0/16"},
			want: false,
		},
		{
			name: "empty slices",
			a:    []string{},
			b:    []string{},
			want: true,
		},
		{
			name: "one empty one not",
			a:    []string{"10.0.0.0/8"},
			b:    []string{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StringSlicesEqual(tt.a, tt.b)
			assert.Equal(t, tt.want, got, "StringSlicesEqual(%v, %v)", tt.a, tt.b)
		})
	}
}

func TestBuildPeerArgs(t *testing.T) {
	tests := []struct {
		name           string
		deviceName     string
		peer           WireGuardPeer
		wantArgs       []string
		wantNeedsStdin bool
	}{
		{
			name:       "peer with all fields",
			deviceName: "wg0",
			peer: WireGuardPeer{
				PublicKey:           "abcd1234efgh5678ijkl90mnopqr==AAAABBBBCCCC",
				PresharedKey:        "presharedkey123456789",
				Endpoint:            "vpn.example.com:51820",
				AllowedIPs:          []string{"0.0.0.0/0"},
				PersistentKeepalive: 25,
			},
			wantArgs:       []string{"set", "wg0", "peer", "abcd1234efgh5678ijkl90mnopqr==AAAABBBBCCCC", "preshared-key", "/dev/stdin", "endpoint", "vpn.example.com:51820", "allowed-ips", "0.0.0.0/0", "persistent-keepalive", "25"},
			wantNeedsStdin: true,
		},
		{
			name:       "peer without preshared key",
			deviceName: "wg0",
			peer: WireGuardPeer{
				PublicKey:  "abcd1234efgh5678ijkl90mnopqr==AAAABBBBCCCC",
				Endpoint:   "vpn.example.com:51820",
				AllowedIPs: []string{"10.0.0.0/8", "192.168.0.0/16"},
			},
			wantArgs:       []string{"set", "wg0", "peer", "abcd1234efgh5678ijkl90mnopqr==AAAABBBBCCCC", "endpoint", "vpn.example.com:51820", "allowed-ips", "10.0.0.0/8,192.168.0.0/16"},
			wantNeedsStdin: false,
		},
		{
			name:       "peer with minimal config",
			deviceName: "wg-client",
			peer: WireGuardPeer{
				PublicKey:  "xyz789==",
				AllowedIPs: []string{"10.0.0.1/32"},
			},
			wantArgs:       []string{"set", "wg-client", "peer", "xyz789==", "allowed-ips", "10.0.0.1/32"},
			wantNeedsStdin: false,
		},
		{
			name:       "peer with keepalive no endpoint",
			deviceName: "wg0",
			peer: WireGuardPeer{
				PublicKey:           "pubkey123==",
				AllowedIPs:          []string{"192.168.1.0/24"},
				PersistentKeepalive: 30,
			},
			wantArgs:       []string{"set", "wg0", "peer", "pubkey123==", "allowed-ips", "192.168.1.0/24", "persistent-keepalive", "30"},
			wantNeedsStdin: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotArgs, gotNeedsStdin := BuildPeerArgs(tt.deviceName, tt.peer)

			assert.Equal(t, tt.wantNeedsStdin, gotNeedsStdin, "BuildPeerArgs() needsStdin")
			assert.Equal(t, tt.wantArgs, gotArgs, "BuildPeerArgs() args")
		})
	}
}

func TestValidateVPNConfig(t *testing.T) {
	tests := []struct {
		name       string
		config     *VPNConfig
		wantError  bool
		errContain string
	}{
		{
			name: "valid config",
			config: &VPNConfig{
				Interfaces: map[string]VPNInterface{
					"wg-proton": {
						DeviceName: "wg-proton",
						PrivateKey: "privatekey123456789abcdefghijklmnop==",
						Address:    "10.2.0.2",
						Netmask:    "255.255.255.0",
						Peers: []WireGuardPeer{
							{
								PublicKey:  "abcd1234efgh5678ijkl90mnopqrABCDEFGHIJKL==",
								Endpoint:   "vpn.example.com:51820",
								AllowedIPs: []string{"0.0.0.0/0"},
							},
						},
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
			name: "no interfaces",
			config: &VPNConfig{
				Interfaces: map[string]VPNInterface{},
			},
			wantError:  true,
			errContain: "no interfaces defined",
		},
		{
			name: "missing device name",
			config: &VPNConfig{
				Interfaces: map[string]VPNInterface{
					"wg0": {
						PrivateKey: "key",
						Address:    "10.0.0.1",
						Netmask:    "255.255.255.0",
					},
				},
			},
			wantError:  true,
			errContain: "device_name is required",
		},
		{
			name: "missing private key",
			config: &VPNConfig{
				Interfaces: map[string]VPNInterface{
					"wg0": {
						DeviceName: "wg0",
						Address:    "10.0.0.1",
						Netmask:    "255.255.255.0",
					},
				},
			},
			wantError:  true,
			errContain: "private_key is required",
		},
		{
			name: "missing address",
			config: &VPNConfig{
				Interfaces: map[string]VPNInterface{
					"wg0": {
						DeviceName: "wg0",
						PrivateKey: "key",
						Netmask:    "255.255.255.0",
					},
				},
			},
			wantError:  true,
			errContain: "address is required",
		},
		{
			name: "missing netmask",
			config: &VPNConfig{
				Interfaces: map[string]VPNInterface{
					"wg0": {
						DeviceName: "wg0",
						PrivateKey: "key",
						Address:    "10.0.0.1",
					},
				},
			},
			wantError:  true,
			errContain: "netmask is required",
		},
		{
			name: "invalid listen port negative",
			config: &VPNConfig{
				Interfaces: map[string]VPNInterface{
					"wg0": {
						DeviceName: "wg0",
						PrivateKey: "key",
						Address:    "10.0.0.1",
						Netmask:    "255.255.255.0",
						ListenPort: -1,
					},
				},
			},
			wantError:  true,
			errContain: "invalid listen port",
		},
		{
			name: "invalid listen port too high",
			config: &VPNConfig{
				Interfaces: map[string]VPNInterface{
					"wg0": {
						DeviceName: "wg0",
						PrivateKey: "key",
						Address:    "10.0.0.1",
						Netmask:    "255.255.255.0",
						ListenPort: 70000,
					},
				},
			},
			wantError:  true,
			errContain: "invalid listen port",
		},
		{
			name: "invalid MTU",
			config: &VPNConfig{
				Interfaces: map[string]VPNInterface{
					"wg0": {
						DeviceName: "wg0",
						PrivateKey: "key",
						Address:    "10.0.0.1",
						Netmask:    "255.255.255.0",
						MTU:        10000,
					},
				},
			},
			wantError:  true,
			errContain: "invalid MTU",
		},
		{
			name: "invalid IP address",
			config: &VPNConfig{
				Interfaces: map[string]VPNInterface{
					"wg0": {
						DeviceName: "wg0",
						PrivateKey: "key",
						Address:    "999.999.999.999",
						Netmask:    "255.255.255.0",
					},
				},
			},
			wantError:  true,
			errContain: "invalid IP address",
		},
		{
			name: "invalid CIDR",
			config: &VPNConfig{
				Interfaces: map[string]VPNInterface{
					"wg0": {
						DeviceName: "wg0",
						PrivateKey: "key",
						Address:    "10.0.0.1/99",
						Netmask:    "255.255.255.0",
					},
				},
			},
			wantError:  true,
			errContain: "invalid CIDR",
		},
		{
			name: "peer missing public key",
			config: &VPNConfig{
				Interfaces: map[string]VPNInterface{
					"wg0": {
						DeviceName: "wg0",
						PrivateKey: "key",
						Address:    "10.0.0.1",
						Netmask:    "255.255.255.0",
						Peers: []WireGuardPeer{
							{AllowedIPs: []string{"0.0.0.0/0"}},
						},
					},
				},
			},
			wantError:  true,
			errContain: "public_key is required",
		},
		{
			name: "peer invalid public key format",
			config: &VPNConfig{
				Interfaces: map[string]VPNInterface{
					"wg0": {
						DeviceName: "wg0",
						PrivateKey: "key",
						Address:    "10.0.0.1",
						Netmask:    "255.255.255.0",
						Peers: []WireGuardPeer{
							{
								PublicKey:  "short",
								AllowedIPs: []string{"0.0.0.0/0"},
							},
						},
					},
				},
			},
			wantError:  true,
			errContain: "invalid public_key format",
		},
		{
			name: "peer missing allowed IPs",
			config: &VPNConfig{
				Interfaces: map[string]VPNInterface{
					"wg0": {
						DeviceName: "wg0",
						PrivateKey: "key",
						Address:    "10.0.0.1",
						Netmask:    "255.255.255.0",
						Peers: []WireGuardPeer{
							{PublicKey: "abcd1234efgh5678ijkl90mnopqrABCDEFGHIJKL=="},
						},
					},
				},
			},
			wantError:  true,
			errContain: "allowed_ips is required",
		},
		{
			name: "peer invalid allowed IP CIDR",
			config: &VPNConfig{
				Interfaces: map[string]VPNInterface{
					"wg0": {
						DeviceName: "wg0",
						PrivateKey: "key",
						Address:    "10.0.0.1",
						Netmask:    "255.255.255.0",
						Peers: []WireGuardPeer{
							{
								PublicKey:  "abcd1234efgh5678ijkl90mnopqrABCDEFGHIJKL==",
								AllowedIPs: []string{"not-a-cidr"},
							},
						},
					},
				},
			},
			wantError:  true,
			errContain: "invalid CIDR",
		},
		{
			name: "peer invalid endpoint format",
			config: &VPNConfig{
				Interfaces: map[string]VPNInterface{
					"wg0": {
						DeviceName: "wg0",
						PrivateKey: "key",
						Address:    "10.0.0.1",
						Netmask:    "255.255.255.0",
						Peers: []WireGuardPeer{
							{
								PublicKey:  "abcd1234efgh5678ijkl90mnopqrABCDEFGHIJKL==",
								AllowedIPs: []string{"0.0.0.0/0"},
								Endpoint:   "invalid-endpoint",
							},
						},
					},
				},
			},
			wantError:  true,
			errContain: "invalid endpoint format",
		},
		{
			name: "peer invalid persistent keepalive",
			config: &VPNConfig{
				Interfaces: map[string]VPNInterface{
					"wg0": {
						DeviceName: "wg0",
						PrivateKey: "key",
						Address:    "10.0.0.1",
						Netmask:    "255.255.255.0",
						Peers: []WireGuardPeer{
							{
								PublicKey:           "abcd1234efgh5678ijkl90mnopqrABCDEFGHIJKL==",
								AllowedIPs:          []string{"0.0.0.0/0"},
								PersistentKeepalive: 70000,
							},
						},
					},
				},
			},
			wantError:  true,
			errContain: "invalid persistent_keepalive",
		},
		{
			name: "listen port out of range negative",
			config: &VPNConfig{
				Interfaces: map[string]VPNInterface{
					"wg0": {
						DeviceName: "wg0",
						PrivateKey: "key",
						Address:    "10.0.0.1",
						Netmask:    "255.255.255.0",
						ListenPort: -1,
					},
				},
			},
			wantError:  true,
			errContain: "invalid listen port",
		},
		{
			name: "listen port exceeds maximum",
			config: &VPNConfig{
				Interfaces: map[string]VPNInterface{
					"wg0": {
						DeviceName: "wg0",
						PrivateKey: "key",
						Address:    "10.0.0.1",
						Netmask:    "255.255.255.0",
						ListenPort: 99999,
					},
				},
			},
			wantError:  true,
			errContain: "invalid listen port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVPNConfig(tt.config)

			if tt.wantError {
				require.Error(t, err, "ValidateVPNConfig() expected error")
				if tt.errContain != "" {
					assert.Contains(t, err.Error(), tt.errContain)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestFormatCIDR(t *testing.T) {
	tests := []struct {
		name    string
		ipAddr  string
		netmask string
		want    string
	}{
		{
			name:    "IP without CIDR gets formatted",
			ipAddr:  "10.0.0.1",
			netmask: "255.255.255.0",
			want:    "10.0.0.1/24",
		},
		{
			name:    "IP with CIDR is returned as-is",
			ipAddr:  "10.0.0.1/32",
			netmask: "255.255.255.255",
			want:    "10.0.0.1/32",
		},
		{
			name:    "different netmask",
			ipAddr:  "192.168.1.1",
			netmask: "255.255.0.0",
			want:    "192.168.1.1/16",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatCIDR(tt.ipAddr, tt.netmask)
			assert.Equal(t, tt.want, got, "FormatCIDR(%q, %q)", tt.ipAddr, tt.netmask)
		})
	}
}
