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

// Package wireguard implements VPN management using WireGuard.
package main

// VPNConfig represents the VPN configuration.
type VPNConfig struct {
	Version    string                  `json:"version"`
	Interfaces map[string]VPNInterface `json:"interfaces"`
}

// VPNInterface represents a VPN tunnel interface (e.g., WireGuard).
type VPNInterface struct {
	Type         string          `json:"type"` // wireguard, openvpn, ipsec
	Enabled      bool            `json:"enabled"`
	DeviceName   string          `json:"device_name"`   // wg0, tun0, etc.
	PrivateKey   string          `json:"private_key"`   // WireGuard private key
	ListenPort   int             `json:"listen_port"`   // WireGuard listen port
	Address      string          `json:"address"`       // VPN interface IP address
	Netmask      string          `json:"netmask"`       // VPN interface netmask
	MTU          int             `json:"mtu,omitempty"` // Interface MTU
	Peers        []WireGuardPeer `json:"peers"`         // WireGuard peers
	FirewallZone string          `json:"firewall_zone,omitempty"`
	Comment      string          `json:"comment,omitempty"`
}

// WireGuardPeer represents a WireGuard peer configuration
type WireGuardPeer struct {
	PublicKey           string   `json:"public_key"`
	PresharedKey        string   `json:"preshared_key,omitempty"`
	Endpoint            string   `json:"endpoint,omitempty"`
	AllowedIPs          []string `json:"allowed_ips"`
	PersistentKeepalive int      `json:"persistent_keepalive,omitempty"`
	Comment             string   `json:"comment,omitempty"`
}
