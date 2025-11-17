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
