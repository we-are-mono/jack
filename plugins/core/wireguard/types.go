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

import (
	"fmt"

	"github.com/we-are-mono/jack/validation"
)

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

// Validate checks if the VPNConfig is valid.
func (vc *VPNConfig) Validate() error {
	v := validation.NewCollector()

	for ifaceName, iface := range vc.Interfaces {
		if err := iface.Validate(); err != nil {
			v.CheckMsg(err, fmt.Sprintf("VPN interface %s", ifaceName))
		}
	}

	return v.Error()
}

// Validate checks if the VPNInterface is valid.
func (vi *VPNInterface) Validate() error {
	v := validation.NewCollector()

	if vi.ListenPort > 0 {
		v.CheckMsg(validation.ValidatePort(vi.ListenPort), "invalid listen port")
	}

	if vi.PrivateKey != "" {
		v.CheckMsg(validation.ValidateWireGuardKey(vi.PrivateKey), "invalid private key")
	}

	if vi.Address != "" {
		v.CheckMsg(validation.ValidateIP(vi.Address), "invalid address")
	}

	if vi.Netmask != "" {
		v.CheckMsg(validation.ValidateNetmask(vi.Netmask), "invalid netmask")
	}

	if vi.MTU > 0 {
		v.CheckMsg(validation.ValidateMTU(vi.MTU), "invalid MTU")
	}

	for idx, peer := range vi.Peers {
		if err := peer.Validate(); err != nil {
			v.CheckMsg(err, fmt.Sprintf("peer %d", idx))
		}
	}

	return v.Error()
}

// Validate checks if the WireGuardPeer is valid.
func (wp *WireGuardPeer) Validate() error {
	v := validation.NewCollector()

	v.CheckMsg(validation.ValidateWireGuardKey(wp.PublicKey), "invalid public key")

	if wp.PresharedKey != "" {
		v.CheckMsg(validation.ValidateWireGuardKey(wp.PresharedKey), "invalid preshared key")
	}

	if wp.Endpoint != "" {
		v.CheckMsg(validation.ValidateEndpoint(wp.Endpoint), "invalid endpoint")
	}

	for _, allowedIP := range wp.AllowedIPs {
		v.CheckMsg(validation.ValidateCIDR(allowedIP), fmt.Sprintf("invalid allowed IP %s", allowedIP))
	}

	return v.Error()
}
