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

// Package main implements firewall management using nftables.
package main

// FirewallConfig represents the firewall configuration
type FirewallConfig struct {
	Version      string           `json:"version"`
	Enabled      bool             `json:"enabled"`
	Defaults     FirewallDefaults `json:"defaults"`
	Zones        map[string]Zone  `json:"zones"`
	Forwardings  []Forwarding     `json:"forwardings"`
	Rules        []Rule           `json:"rules"`
	PortForwards []PortForward    `json:"port_forwards"`
}

// FirewallDefaults represents default firewall policies
type FirewallDefaults struct {
	Input       string `json:"input"`
	Forward     string `json:"forward"`
	Output      string `json:"output"`
	SynFlood    bool   `json:"syn_flood"`
	DropInvalid bool   `json:"drop_invalid"`
}

// Zone represents a firewall zone
type Zone struct {
	Name       string   `json:"name"`
	Interfaces []string `json:"interfaces"`
	Input      string   `json:"input"`
	Forward    string   `json:"forward"`
	Output     string   `json:"output"`
	Masquerade bool     `json:"masquerade"`
	MTUFix     bool     `json:"mtu_fix"`
	Family     string   `json:"family"`
	Comment    string   `json:"comment"`
}

// Forwarding represents zone-to-zone forwarding
type Forwarding struct {
	Src     string `json:"src"`
	Dest    string `json:"dest"`
	Family  string `json:"family"`
	Comment string `json:"comment"`
}

// Rule represents a firewall rule
type Rule struct {
	Name     string   `json:"name"`
	Src      string   `json:"src"`
	Dest     string   `json:"dest,omitempty"`
	Proto    string   `json:"proto,omitempty"`
	SrcIP    string   `json:"src_ip,omitempty"`
	DestIP   string   `json:"dest_ip,omitempty"`
	SrcPort  string   `json:"src_port,omitempty"`
	DestPort string   `json:"dest_port,omitempty"`
	ICMPType []string `json:"icmp_type,omitempty"`
	Target   string   `json:"target"`
	Family   string   `json:"family"`
	Limit    string   `json:"limit,omitempty"`
}

// PortForward represents a port forwarding rule (DNAT)
type PortForward struct {
	Name     string `json:"name"`
	Src      string `json:"src"`
	Dest     string `json:"dest"`
	Proto    string `json:"proto"`
	SrcDPort string `json:"src_dport"`
	DestIP   string `json:"dest_ip"`
	DestPort string `json:"dest_port"`
	Target   string `json:"target"`
	Enabled  bool   `json:"enabled"`
	Comment  string `json:"comment"`
}
