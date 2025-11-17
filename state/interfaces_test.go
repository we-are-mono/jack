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

package state

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"
)

// TestIsLoopback tests loopback interface detection
func TestIsLoopback(t *testing.T) {
	tests := []struct {
		name     string
		linkName string
		flags    net.Flags
		expected bool
	}{
		{
			name:     "lo interface",
			linkName: "lo",
			flags:    net.FlagLoopback,
			expected: true,
		},
		{
			name:     "loopback flag set",
			linkName: "test",
			flags:    net.FlagLoopback,
			expected: true,
		},
		{
			name:     "not loopback",
			linkName: "eth0",
			flags:    net.FlagUp,
			expected: false,
		},
		{
			name:     "loopback flag not set but name is lo",
			linkName: "lo",
			flags:    0,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a dummy device link for testing
			link := &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name:  tt.linkName,
					Flags: tt.flags,
				},
			}

			result := isLoopback(link)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsBridge tests bridge interface detection
func TestIsBridge(t *testing.T) {
	tests := []struct {
		name     string
		link     netlink.Link
		expected bool
	}{
		{
			name: "bridge interface",
			link: &netlink.Bridge{
				LinkAttrs: netlink.LinkAttrs{
					Name: "br-lan",
				},
			},
			expected: true,
		},
		{
			name: "dummy interface",
			link: &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: "dummy0",
				},
			},
			expected: false,
		},
		{
			name: "veth interface",
			link: &netlink.Veth{
				LinkAttrs: netlink.LinkAttrs{
					Name: "veth0",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBridge(tt.link)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsVirtual tests virtual interface detection
func TestIsVirtual(t *testing.T) {
	tests := []struct {
		name     string
		link     netlink.Link
		expected bool
	}{
		{
			name: "veth interface by type",
			link: &netlink.Veth{
				LinkAttrs: netlink.LinkAttrs{
					Name: "veth0",
				},
			},
			expected: true,
		},
		{
			name: "tuntap interface by type",
			link: &netlink.Tuntap{
				LinkAttrs: netlink.LinkAttrs{
					Name: "tun0",
				},
			},
			expected: true,
		},
		{
			name: "wireguard interface by type",
			link: &netlink.Wireguard{
				LinkAttrs: netlink.LinkAttrs{
					Name: "wg0",
				},
			},
			expected: true,
		},
		{
			name: "docker interface by name",
			link: &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: "docker0",
				},
			},
			expected: true,
		},
		{
			name: "veth by name prefix",
			link: &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: "veth123abc",
				},
			},
			expected: true,
		},
		{
			name: "tun by name prefix",
			link: &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: "tun0",
				},
			},
			expected: true,
		},
		{
			name: "tap by name prefix",
			link: &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: "tap0",
				},
			},
			expected: true,
		},
		{
			name: "wireguard by name prefix wg",
			link: &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: "wg0",
				},
			},
			expected: true,
		},
		{
			name: "wireguard by name prefix wg-",
			link: &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: "wg-proton",
				},
			},
			expected: true,
		},
		{
			name: "sit tunnel",
			link: &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: "sit0",
				},
			},
			expected: true,
		},
		{
			name: "teql interface",
			link: &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: "teql0",
				},
			},
			expected: true,
		},
		{
			name: "ip6tnl tunnel",
			link: &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: "ip6tnl0",
				},
			},
			expected: true,
		},
		{
			name: "gre tunnel",
			link: &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: "gre0",
				},
			},
			expected: true,
		},
		{
			name: "vlan interface",
			link: &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: "vlan10",
				},
			},
			expected: true,
		},
		{
			name: "macvlan interface",
			link: &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: "macvlan0",
				},
			},
			expected: true,
		},
		{
			name: "vxlan interface",
			link: &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: "vxlan100",
				},
			},
			expected: true,
		},
		{
			name: "physical interface eth0",
			link: &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: "eth0",
				},
			},
			expected: false,
		},
		{
			name: "physical interface ens18",
			link: &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: "ens18",
				},
			},
			expected: false,
		},
		{
			name: "physical interface enp0s3",
			link: &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: "enp0s3",
				},
			},
			expected: false,
		},
		{
			name: "physical interface lan1",
			link: &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: "lan1",
				},
			},
			expected: false,
		},
		{
			name: "bridge (not virtual)",
			link: &netlink.Bridge{
				LinkAttrs: netlink.LinkAttrs{
					Name: "br-lan",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isVirtual(tt.link)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsVirtual_EdgeCases tests edge cases for virtual interface detection
func TestIsVirtual_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		interfaceName string
		expected      bool
	}{
		{"empty name", "", false},
		{"single char", "v", false},
		{"two chars matching prefix", "wg", true},
		{"prefix but longer name", "wireless0", false},
		{"exact match veth", "veth", true},
		{"prefix wg- with name", "wg-mullvad", true},
		{"almost matching vlan", "vla", false}, // Doesn't match "vlan" prefix
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link := &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: tt.interfaceName,
				},
			}

			result := isVirtual(link)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Benchmark tests

func BenchmarkIsLoopback(b *testing.B) {
	link := &netlink.Dummy{
		LinkAttrs: netlink.LinkAttrs{
			Name:  "eth0",
			Flags: net.FlagUp,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isLoopback(link)
	}
}

func BenchmarkIsBridge(b *testing.B) {
	link := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: "br-lan",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = isBridge(link)
	}
}

func BenchmarkIsVirtual(b *testing.B) {
	tests := []struct {
		name string
		link netlink.Link
	}{
		{
			name: "physical",
			link: &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: "eth0",
				},
			},
		},
		{
			name: "virtual veth",
			link: &netlink.Veth{
				LinkAttrs: netlink.LinkAttrs{
					Name: "veth0",
				},
			},
		},
		{
			name: "virtual by name",
			link: &netlink.Dummy{
				LinkAttrs: netlink.LinkAttrs{
					Name: "docker0",
				},
			},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = isVirtual(tt.link)
			}
		})
	}
}
