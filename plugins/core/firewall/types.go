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

import (
	"fmt"

	"github.com/we-are-mono/jack/validation"
)

// FirewallConfig represents the firewall configuration
type FirewallConfig struct {
	Version      string           `json:"version"`
	Enabled      bool             `json:"enabled"`
	Defaults     FirewallDefaults `json:"defaults"`
	Zones        map[string]Zone  `json:"zones"`
	Forwardings  []Forwarding     `json:"forwardings"`
	Rules        []Rule           `json:"rules"`
	PortForwards []PortForward    `json:"port_forwards"`
	Logging      LoggingConfig    `json:"logging"`
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

// LoggingConfig represents firewall logging configuration
type LoggingConfig struct {
	Enabled          bool `json:"enabled"`
	LogAccepts       bool `json:"log_accepts"`        // Log ACCEPT packets
	LogDrops         bool `json:"log_drops"`          // Log DROP packets
	SamplingRate     int  `json:"sampling_rate"`      // Log 1 out of N packets (1 = all)
	RateLimitPerSec  int  `json:"rate_limit_per_sec"` // Maximum logs per second
	MaxLogEntries    int  `json:"max_log_entries"`    // Maximum entries in database (0 = unlimited)
	RetentionDays    int  `json:"retention_days"`     // Days to keep logs (0 = forever)
}

// FirewallLoggingConfig is an alias for LoggingConfig for compatibility
type FirewallLoggingConfig = LoggingConfig

// FirewallLogEntry represents a single firewall log entry
type FirewallLogEntry struct {
	ID           int64  `json:"id"`
	Timestamp    string `json:"timestamp"`
	Action       string `json:"action"`        // ACCEPT or DROP
	SrcIP        string `json:"src_ip"`
	DstIP        string `json:"dst_ip"`
	Protocol     string `json:"protocol"`      // TCP, UDP, ICMP, etc.
	SrcPort      int    `json:"src_port"`
	DstPort      int    `json:"dst_port"`
	InterfaceIn  string `json:"interface_in"`
	InterfaceOut string `json:"interface_out"`
	PacketLength int    `json:"packet_length"`
	CreatedAt    string `json:"created_at"`
}

// FirewallLogQuery represents query parameters for filtering logs
type FirewallLogQuery struct {
	Action       string `json:"action"`        // Filter by action (ACCEPT/DROP)
	SrcIP        string `json:"src_ip"`        // Filter by source IP
	DstIP        string `json:"dst_ip"`        // Filter by destination IP
	Protocol     string `json:"protocol"`      // Filter by protocol
	InterfaceIn  string `json:"interface_in"`  // Filter by input interface
	InterfaceOut string `json:"interface_out"` // Filter by output interface
	Limit        int    `json:"limit"`         // Maximum number of results (0 = no limit)
	Since        string `json:"since"`         // Only show logs after this timestamp (RFC3339)
}

// Validate checks if the FirewallConfig is valid.
func (fc *FirewallConfig) Validate() error {
	v := validation.NewCollector()

	if fc == nil {
		v.Check(fmt.Errorf("config is nil"))
		return v.Error()
	}

	if len(fc.Zones) == 0 {
		v.Check(fmt.Errorf("no zones defined"))
	}

	// Track interfaces to detect duplicates across zones
	interfaceToZone := make(map[string]string)

	// Validate each zone
	for zoneName, zone := range fc.Zones {
		if zoneName == "" {
			v.Check(fmt.Errorf("zone name cannot be empty"))
			continue
		}

		// Check for duplicate interfaces across zones
		for _, iface := range zone.Interfaces {
			if existingZone, exists := interfaceToZone[iface]; exists {
				v.Check(fmt.Errorf("interface %s appears in multiple zones: %s and %s", iface, existingZone, zoneName))
			}
			interfaceToZone[iface] = zoneName
		}

		// Validate zone configuration
		if err := zone.Validate(); err != nil {
			v.CheckMsg(err, fmt.Sprintf("zone %s", zoneName))
		}
	}

	// Validate forwardings reference existing zones
	for idx, fwd := range fc.Forwardings {
		if err := fwd.Validate(fc.Zones); err != nil {
			v.CheckMsg(err, fmt.Sprintf("forwarding %d", idx))
		}
	}

	// Validate rules
	for _, rule := range fc.Rules {
		if err := rule.Validate(fc.Zones); err != nil {
			v.CheckMsg(err, fmt.Sprintf("rule %s", rule.Name))
		}
	}

	// Validate port forwards
	for _, pf := range fc.PortForwards {
		if err := pf.Validate(); err != nil {
			v.CheckMsg(err, fmt.Sprintf("port forward %s", pf.Name))
		}
	}

	return v.Error()
}

// Validate checks if the Zone configuration is valid.
func (z *Zone) Validate() error {
	v := validation.NewCollector()

	if len(z.Interfaces) == 0 {
		v.Check(fmt.Errorf("zone has no interfaces"))
	}

	v.CheckMsg(validation.ValidatePolicy(z.Input), "invalid input policy")
	v.CheckMsg(validation.ValidatePolicy(z.Forward), "invalid forward policy")

	return v.Error()
}

// Validate checks if the Forwarding is valid and references existing zones.
func (f *Forwarding) Validate(zones map[string]Zone) error {
	v := validation.NewCollector()

	if f.Src == "" {
		v.Check(fmt.Errorf("source zone not specified"))
	}

	if f.Dest == "" {
		v.Check(fmt.Errorf("destination zone not specified"))
	}

	if _, exists := zones[f.Src]; !exists && f.Src != "" {
		v.Check(fmt.Errorf("references non-existent source zone: %s", f.Src))
	}

	if _, exists := zones[f.Dest]; !exists && f.Dest != "" {
		v.Check(fmt.Errorf("references non-existent dest zone: %s", f.Dest))
	}

	return v.Error()
}

// Validate checks if the Rule is valid.
func (r *Rule) Validate(zones map[string]Zone) error {
	v := validation.NewCollector()

	if r.Src != "" {
		if _, exists := zones[r.Src]; !exists {
			v.Check(fmt.Errorf("references non-existent source zone: %s", r.Src))
		}
	}

	v.CheckMsg(validation.ValidatePolicy(r.Target), "invalid target")

	if r.Proto != "" {
		validProtos := []string{"tcp", "udp", "icmp", "icmpv6"}
		v.Check(validation.ValidateProtocol(r.Proto, validProtos))
	}

	if r.SrcPort != "" {
		v.CheckMsg(validation.ValidatePortString(r.SrcPort), "invalid source port")
	}

	if r.DestPort != "" {
		v.CheckMsg(validation.ValidatePortString(r.DestPort), "invalid destination port")
	}

	if r.SrcIP != "" {
		v.CheckMsg(validation.ValidateIP(r.SrcIP), "invalid source IP")
	}

	if r.DestIP != "" {
		v.CheckMsg(validation.ValidateIP(r.DestIP), "invalid destination IP")
	}

	return v.Error()
}

// Validate checks if the PortForward is valid.
func (pf *PortForward) Validate() error {
	v := validation.NewCollector()

	if pf.DestIP == "" {
		v.Check(fmt.Errorf("missing destination IP"))
	} else {
		v.CheckMsg(validation.ValidateIP(pf.DestIP), "invalid destination IP")
	}

	if pf.SrcDPort == "" {
		v.Check(fmt.Errorf("missing source port"))
	} else {
		v.CheckMsg(validation.ValidatePortString(pf.SrcDPort), "invalid source port")
	}

	if pf.DestPort != "" {
		v.CheckMsg(validation.ValidatePortString(pf.DestPort), "invalid destination port")
	}

	if pf.Proto != "" {
		validProtos := []string{"tcp", "udp"}
		v.CheckMsg(validation.ValidateProtocol(pf.Proto, validProtos), "invalid protocol (must be tcp or udp)")
	}

	return v.Error()
}
