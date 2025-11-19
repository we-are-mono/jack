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

package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInterfaceMarshaling tests Interface JSON marshaling
func TestInterfaceMarshaling(t *testing.T) {
	iface := Interface{
		Type:     "physical",
		Device:   "eth0",
		Protocol: "static",
		IPAddr:   "10.0.0.1",
		Netmask:  "255.255.255.0",
		Gateway:  "10.0.0.254",
		Enabled:  true,
	}

	// Marshal to JSON
	data, err := json.Marshal(iface)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Unmarshal back
	var decoded Interface
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, iface.Type, decoded.Type)
	assert.Equal(t, iface.Device, decoded.Device)
	assert.Equal(t, iface.Protocol, decoded.Protocol)
}

// TestBridgeInterfaceMarshaling tests bridge interface with ports
func TestBridgeInterfaceMarshaling(t *testing.T) {
	iface := Interface{
		Type:        "bridge",
		Device:      "br-lan",
		Protocol:    "static",
		BridgePorts: []string{"lan1", "lan2", "lan3"},
		Enabled:     true,
	}

	data, err := json.Marshal(iface)
	require.NoError(t, err)

	var decoded Interface
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "bridge", decoded.Type)
	assert.Len(t, decoded.BridgePorts, 3)
}

// TestVLANInterfaceMarshaling tests VLAN interface
func TestVLANInterfaceMarshaling(t *testing.T) {
	iface := Interface{
		Type:     "vlan",
		Device:   "eth0.100",
		Protocol: "static",
		VLANId:   100,
		Enabled:  true,
	}

	data, err := json.Marshal(iface)
	require.NoError(t, err)

	var decoded Interface
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "vlan", decoded.Type)
	assert.Equal(t, 100, decoded.VLANId)
}

// TestWireGuardInterfaceMarshaling tests WireGuard interface with peers
func TestWireGuardInterfaceMarshaling(t *testing.T) {
	iface := Interface{
		Type:         "wireguard",
		Device:       "wg0",
		Protocol:     "static",
		WGPrivateKey: "privatekey123",
		WGListenPort: 51820,
		WGPeers: []WireGuardPeer{
			{
				PublicKey:  "peerkey123",
				Endpoint:   "vpn.example.com:51820",
				AllowedIPs: []string{"10.0.0.0/8"},
			},
		},
		Enabled: true,
	}

	data, err := json.Marshal(iface)
	require.NoError(t, err)

	var decoded Interface
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "wireguard", decoded.Type)
	assert.Len(t, decoded.WGPeers, 1)
	assert.Equal(t, "peerkey123", decoded.WGPeers[0].PublicKey)
}

// TestRouteMarshaling tests Route JSON marshaling
func TestRouteMarshaling(t *testing.T) {
	route := Route{
		Name:        "vpn-route",
		Destination: "10.10.0.0/16",
		Gateway:     "10.0.0.254",
		Interface:   "wg0",
		Metric:      100,
		Table:       1,
		Enabled:     true,
	}

	data, err := json.Marshal(route)
	require.NoError(t, err)

	var decoded Route
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, route.Name, decoded.Name)
	assert.Equal(t, route.Destination, decoded.Destination)
	assert.Equal(t, route.Gateway, decoded.Gateway)
	assert.Equal(t, route.Interface, decoded.Interface)
	assert.Equal(t, route.Metric, decoded.Metric)
	assert.Equal(t, route.Table, decoded.Table)
}

// TestInterfacesConfigMarshaling tests InterfacesConfig
func TestInterfacesConfigMarshaling(t *testing.T) {
	config := InterfacesConfig{
		Interfaces: map[string]Interface{
			"wan": {
				Type:     "physical",
				Device:   "eth0",
				Protocol: "static",
				Enabled:  true,
			},
			"lan": {
				Type:        "bridge",
				Device:      "br-lan",
				Protocol:    "static",
				BridgePorts: []string{"lan1", "lan2"},
				Enabled:     true,
			},
		},
		Version: "1.0.0",
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	var decoded InterfacesConfig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Len(t, decoded.Interfaces, 2)
	assert.Equal(t, "physical", decoded.Interfaces["wan"].Type)
	assert.Equal(t, "bridge", decoded.Interfaces["lan"].Type)
}

// TestRoutesConfigMarshaling tests RoutesConfig
func TestRoutesConfigMarshaling(t *testing.T) {
	config := RoutesConfig{
		Routes: map[string]Route{
			"default": {
				Name:        "default",
				Destination: "0.0.0.0/0",
				Gateway:     "10.0.0.1",
				Enabled:     true,
			},
			"vpn": {
				Name:        "vpn",
				Destination: "10.10.0.0/16",
				Interface:   "wg0",
				Enabled:     true,
			},
		},
		Version: "1.0.0",
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)

	var decoded RoutesConfig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Len(t, decoded.Routes, 2)
	assert.Equal(t, "0.0.0.0/0", decoded.Routes["default"].Destination)
}

// TestInterfaceOmitEmpty tests that empty fields are omitted
func TestInterfaceOmitEmpty(t *testing.T) {
	iface := Interface{
		Type:     "physical",
		Device:   "eth0",
		Protocol: "static",
		Enabled:  false,
	}

	data, err := json.Marshal(iface)
	require.NoError(t, err)

	var jsonMap map[string]interface{}
	err = json.Unmarshal(data, &jsonMap)
	require.NoError(t, err)

	// Required fields
	assert.Contains(t, jsonMap, "type")
	assert.Contains(t, jsonMap, "protocol")

	// Empty fields should be omitted
	assert.NotContains(t, jsonMap, "bridge_ports")
	assert.NotContains(t, jsonMap, "vlan_id")
}

// TestIPv6ConfigMarshaling tests IPv6 configuration
func TestIPv6ConfigMarshaling(t *testing.T) {
	ipv6 := IPv6Config{
		Protocol:  "static",
		IP6Addr:   "2001:db8::1/64",
		IP6GW:     "2001:db8::ffff",
		IP6Prefix: "2001:db8::/48",
		DNS6:      []string{"2001:4860:4860::8888"},
		Enabled:   true,
	}

	data, err := json.Marshal(ipv6)
	require.NoError(t, err)

	var decoded IPv6Config
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, ipv6.Protocol, decoded.Protocol)
	assert.Equal(t, ipv6.IP6Addr, decoded.IP6Addr)
	assert.True(t, decoded.Enabled)
}

// TestWireGuardPeerMarshaling tests WireGuard peer configuration
func TestWireGuardPeerMarshaling(t *testing.T) {
	peer := WireGuardPeer{
		PublicKey:           "pubkey123",
		PresharedKey:        "presharedkey456",
		Endpoint:            "vpn.example.com:51820",
		AllowedIPs:          []string{"10.0.0.0/8", "192.168.0.0/16"},
		PersistentKeepalive: 25,
		Comment:             "Test peer",
	}

	data, err := json.Marshal(peer)
	require.NoError(t, err)

	var decoded WireGuardPeer
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, peer.PublicKey, decoded.PublicKey)
	assert.Equal(t, peer.Endpoint, decoded.Endpoint)
	assert.Len(t, decoded.AllowedIPs, 2)
	assert.Equal(t, 25, decoded.PersistentKeepalive)
}

// ============================================================================
// Validation Tests
// ============================================================================

// TestRoute_Validate tests Route validation
func TestRoute_Validate(t *testing.T) {
	tests := []struct {
		name       string
		route      Route
		wantErr    bool
		errContain string
	}{
		{
			name: "valid route",
			route: Route{
				Name:        "default",
				Destination: "0.0.0.0/0",
				Gateway:     "192.168.1.1",
				Enabled:     true,
			},
			wantErr: false,
		},
		{
			name: "valid route without gateway",
			route: Route{
				Name:        "direct",
				Destination: "192.168.1.0/24",
				Interface:   "eth0",
				Enabled:     true,
			},
			wantErr: false,
		},
		{
			name: "invalid destination",
			route: Route{
				Name:        "bad",
				Destination: "not-a-cidr",
				Enabled:     true,
			},
			wantErr:    true,
			errContain: "invalid destination",
		},
		{
			name: "invalid gateway",
			route: Route{
				Name:        "bad",
				Destination: "10.0.0.0/8",
				Gateway:     "999.999.999.999",
				Enabled:     true,
			},
			wantErr:    true,
			errContain: "invalid gateway",
		},
		{
			name: "negative metric",
			route: Route{
				Name:        "bad",
				Destination: "10.0.0.0/8",
				Metric:      -1,
				Enabled:     true,
			},
			wantErr:    true,
			errContain: "metric",
		},
		{
			name: "invalid table ID",
			route: Route{
				Name:        "bad",
				Destination: "10.0.0.0/8",
				Table:       -1,
				Enabled:     true,
			},
			wantErr:    true,
			errContain: "table",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.route.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContain)
				assert.Contains(t, err.Error(), "route "+tt.route.Name)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestInterface_Validate tests Interface validation
func TestInterface_Validate(t *testing.T) {
	tests := []struct {
		name       string
		iface      Interface
		wantErr    bool
		errContain string
	}{
		{
			name: "valid DHCP interface",
			iface: Interface{
				Type:     "physical",
				Protocol: "dhcp",
				Enabled:  true,
			},
			wantErr: false,
		},
		{
			name: "valid static interface",
			iface: Interface{
				Type:     "physical",
				Protocol: "static",
				IPAddr:   "192.168.1.10",
				Netmask:  "255.255.255.0",
				Gateway:  "192.168.1.1",
				DNS:      []string{"8.8.8.8"},
				Enabled:  true,
			},
			wantErr: false,
		},
		{
			name: "invalid MTU",
			iface: Interface{
				Type:     "physical",
				Protocol: "dhcp",
				MTU:      67,
				Enabled:  true,
			},
			wantErr:    true,
			errContain: "MTU",
		},
		{
			name: "invalid IP address",
			iface: Interface{
				Type:     "physical",
				Protocol: "static",
				IPAddr:   "999.999.999.999",
				Enabled:  true,
			},
			wantErr:    true,
			errContain: "invalid IP address",
		},
		{
			name: "invalid netmask",
			iface: Interface{
				Type:     "physical",
				Protocol: "static",
				IPAddr:   "192.168.1.10",
				Netmask:  "255.255.999.0",
				Enabled:  true,
			},
			wantErr:    true,
			errContain: "netmask",
		},
		{
			name: "invalid MAC",
			iface: Interface{
				Type:     "physical",
				Protocol: "dhcp",
				MAC:      "00:11:22:33:44:ZZ",
				Enabled:  true,
			},
			wantErr:    true,
			errContain: "MAC",
		},
		{
			name: "invalid VLAN ID",
			iface: Interface{
				Type:     "vlan",
				Protocol: "dhcp",
				VLANId:   5000,
				Enabled:  true,
			},
			wantErr:    true,
			errContain: "VLAN",
		},
		{
			name: "invalid DNS",
			iface: Interface{
				Type:     "physical",
				Protocol: "dhcp",
				DNS:      []string{"8.8.8.8", "bad-dns"},
				Enabled:  true,
			},
			wantErr:    true,
			errContain: "invalid DNS server",
		},
		{
			name: "invalid WireGuard port",
			iface: Interface{
				Type:         "wireguard",
				Protocol:     "static",
				WGListenPort: 70000,
				Enabled:      true,
			},
			wantErr:    true,
			errContain: "invalid WireGuard listen port",
		},
		{
			name: "invalid WireGuard key",
			iface: Interface{
				Type:         "wireguard",
				Protocol:     "static",
				WGPrivateKey: "not-a-key",
				Enabled:      true,
			},
			wantErr:    true,
			errContain: "invalid WireGuard private key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.iface.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContain)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestIPv6Config_Validate tests IPv6 configuration validation
func TestIPv6Config_Validate(t *testing.T) {
	tests := []struct {
		name       string
		config     IPv6Config
		wantErr    bool
		errContain string
	}{
		{
			name: "valid DHCP",
			config: IPv6Config{
				Protocol: "dhcp",
				Enabled:  true,
			},
			wantErr: false,
		},
		{
			name: "valid static",
			config: IPv6Config{
				Protocol: "static",
				IP6Addr:  "2001:db8::1",
				IP6GW:    "2001:db8::ff",
				DNS6:     []string{"2001:4860:4860::8888"},
				Enabled:  true,
			},
			wantErr: false,
		},
		{
			name: "invalid IP6Addr",
			config: IPv6Config{
				Protocol: "static",
				IP6Addr:  "not-ipv6",
				Enabled:  true,
			},
			wantErr:    true,
			errContain: "invalid address",
		},
		{
			name: "invalid IP6GW",
			config: IPv6Config{
				Protocol: "static",
				IP6Addr:  "2001:db8::1",
				IP6GW:    "bad-gateway",
				Enabled:  true,
			},
			wantErr:    true,
			errContain: "invalid gateway",
		},
		{
			name: "invalid DNS6",
			config: IPv6Config{
				Protocol: "static",
				DNS6:     []string{"bad-dns"},
				Enabled:  true,
			},
			wantErr:    true,
			errContain: "invalid DNS server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContain)
				assert.Contains(t, err.Error(), "IPv6")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestWireGuardPeer_Validate tests WireGuard peer validation
func TestWireGuardPeer_Validate(t *testing.T) {
	tests := []struct {
		name       string
		peer       WireGuardPeer
		wantErr    bool
		errContain string
	}{
		{
			name: "valid peer",
			peer: WireGuardPeer{
				PublicKey:  "xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=",
				AllowedIPs: []string{"10.0.0.2/32"},
			},
			wantErr: false,
		},
		{
			name: "valid peer with endpoint",
			peer: WireGuardPeer{
				PublicKey:  "xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=",
				Endpoint:   "vpn.example.com:51820",
				AllowedIPs: []string{"0.0.0.0/0"},
			},
			wantErr: false,
		},
		{
			name: "invalid public key",
			peer: WireGuardPeer{
				PublicKey:  "bad-key",
				AllowedIPs: []string{"10.0.0.2/32"},
			},
			wantErr:    true,
			errContain: "invalid public key",
		},
		{
			name: "invalid preshared key",
			peer: WireGuardPeer{
				PublicKey:    "xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=",
				PresharedKey: "bad-psk",
				AllowedIPs:   []string{"10.0.0.2/32"},
			},
			wantErr:    true,
			errContain: "invalid preshared key",
		},
		{
			name: "invalid endpoint",
			peer: WireGuardPeer{
				PublicKey:  "xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=",
				Endpoint:   "bad-endpoint",
				AllowedIPs: []string{"10.0.0.2/32"},
			},
			wantErr:    true,
			errContain: "invalid endpoint",
		},
		{
			name: "invalid allowed IP",
			peer: WireGuardPeer{
				PublicKey:  "xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg=",
				AllowedIPs: []string{"bad-cidr"},
			},
			wantErr:    true,
			errContain: "invalid allowed IP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.peer.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContain)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
