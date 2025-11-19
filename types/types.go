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

// Package types defines the core data structures for Jack's configuration.
// It includes types for network interfaces, routes, firewall rules, DHCP/DNS settings,
// and plugin provider configurations.
package types

import (
	"fmt"

	"github.com/we-are-mono/jack/validation"
)

// InterfacesConfig represents /etc/netrunner/interfaces.json
type InterfacesConfig struct {
	Interfaces map[string]Interface `json:"interfaces"`
	Version    string               `json:"version"`
}

// Interface represents a network interface configuration
type Interface struct {
	IPv6               *IPv6Config     `json:"ipv6,omitempty"`
	Gateway            string          `json:"gateway,omitempty"`
	Comment            string          `json:"comment,omitempty"`
	MAC                string          `json:"mac,omitempty"`
	Type               string          `json:"type"`
	Device             string          `json:"device,omitempty"`
	DeviceName         string          `json:"device_name,omitempty"`
	Protocol           string          `json:"protocol"`
	IPAddr             string          `json:"ipaddr,omitempty"`
	WGPrivateKey       string          `json:"wg_private_key,omitempty"`
	Netmask            string          `json:"netmask,omitempty"`
	BridgePorts        []string        `json:"bridge_ports,omitempty"`
	DNS                []string        `json:"dns,omitempty"`
	WGPeers            []WireGuardPeer `json:"wg_peers,omitempty"`
	Metric             int             `json:"metric,omitempty"`
	MTU                int             `json:"mtu,omitempty"`
	BridgeForwardDelay int             `json:"bridge_forward_delay,omitempty"`
	VLANId             int             `json:"vlan_id,omitempty"`
	WGListenPort       int             `json:"wg_listen_port,omitempty"`
	Enabled            bool            `json:"enabled"`
	BridgeSTP          bool            `json:"bridge_stp,omitempty"`
}

// IPv6Config holds IPv6-specific settings
type IPv6Config struct {
	Protocol  string   `json:"protocol"`
	IP6Addr   string   `json:"ip6addr,omitempty"`
	IP6GW     string   `json:"ip6gw,omitempty"`
	IP6Prefix string   `json:"ip6prefix,omitempty"`
	DNS6      []string `json:"dns6,omitempty"`
	Enabled   bool     `json:"enabled"`
}

// WireGuardPeer represents a WireGuard peer configuration
type WireGuardPeer struct {
	PublicKey           string   `json:"public_key"`
	PresharedKey        string   `json:"preshared_key,omitempty"`
	Endpoint            string   `json:"endpoint,omitempty"`
	Comment             string   `json:"comment,omitempty"`
	AllowedIPs          []string `json:"allowed_ips"`
	PersistentKeepalive int      `json:"persistent_keepalive,omitempty"`
}

// RoutesConfig represents /etc/jack/routes.json
type RoutesConfig struct {
	Routes  map[string]Route `json:"routes"`
	Version string           `json:"version"`
}

// Route represents a static route configuration
type Route struct {
	Name        string `json:"name"`
	Destination string `json:"destination"` // CIDR notation (e.g., "10.0.0.0/8", "default" for 0.0.0.0/0)
	Gateway     string `json:"gateway,omitempty"`
	Interface   string `json:"interface,omitempty"` // Optional: interface name
	Comment     string `json:"comment,omitempty"`
	Metric      int    `json:"metric,omitempty"` // Route priority (lower is preferred)
	Table       int    `json:"table,omitempty"`  // Routing table ID (default: main)
	Enabled     bool   `json:"enabled"`
}

// Validate checks if the Route configuration is valid.
func (r *Route) Validate() error {
	v := validation.NewCollector().WithContext(fmt.Sprintf("route %s", r.Name))

	v.Check(validation.ValidateMetric(r.Metric))
	v.Check(validation.ValidateTableID(r.Table))
	v.CheckMsg(validation.ValidateCIDR(r.Destination), "invalid destination")

	if r.Gateway != "" {
		v.CheckMsg(validation.ValidateIP(r.Gateway), "invalid gateway")
	}

	return v.Error()
}

// Validate checks if the Interface configuration is valid.
func (i *Interface) Validate() error {
	v := validation.NewCollector().WithContext("interface")

	if i.MTU > 0 {
		v.Check(validation.ValidateMTU(i.MTU))
	}

	if i.Protocol == "static" && i.IPAddr != "" {
		v.CheckMsg(validation.ValidateIP(i.IPAddr), "invalid IP address")
	}

	if i.Netmask != "" {
		v.Check(validation.ValidateNetmask(i.Netmask))
	}

	if i.MAC != "" {
		v.Check(validation.ValidateMAC(i.MAC))
	}

	if i.Type == "vlan" && i.VLANId > 0 {
		v.Check(validation.ValidateVLANID(i.VLANId))
	}

	if i.Gateway != "" {
		v.CheckMsg(validation.ValidateIP(i.Gateway), "invalid gateway")
	}

	for _, dns := range i.DNS {
		v.CheckMsg(validation.ValidateIP(dns), fmt.Sprintf("invalid DNS server %s", dns))
	}

	if i.WGListenPort > 0 {
		v.CheckMsg(validation.ValidatePort(i.WGListenPort), "invalid WireGuard listen port")
	}

	if i.WGPrivateKey != "" {
		v.CheckMsg(validation.ValidateWireGuardKey(i.WGPrivateKey), "invalid WireGuard private key")
	}

	for idx, peer := range i.WGPeers {
		if err := peer.Validate(); err != nil {
			v.CheckMsg(err, fmt.Sprintf("peer %d", idx))
		}
	}

	if i.IPv6 != nil {
		v.Check(i.IPv6.Validate())
	}

	return v.Error()
}

// Validate checks if the IPv6Config is valid.
func (ipv6 *IPv6Config) Validate() error {
	v := validation.NewCollector().WithContext("IPv6")

	if ipv6.IP6Addr != "" {
		v.CheckMsg(validation.ValidateIP(ipv6.IP6Addr), "invalid address")
	}

	if ipv6.IP6GW != "" {
		v.CheckMsg(validation.ValidateIP(ipv6.IP6GW), "invalid gateway")
	}

	for _, dns := range ipv6.DNS6 {
		v.CheckMsg(validation.ValidateIP(dns), fmt.Sprintf("invalid DNS server %s", dns))
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
