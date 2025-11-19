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

// Package nftables implements the nftables plugin for Jack.
package main

import (
	"fmt"
	"strings"
)

// GeneratePortForwardDNATRule generates an nftables DNAT rule for port forwarding.
func GeneratePortForwardDNATRule(portFwd PortForward) string {
	var ruleParts []string

	// Protocol
	if portFwd.Proto != "" {
		ruleParts = append(ruleParts, fmt.Sprintf("meta l4proto %s", portFwd.Proto))
	}

	// Source port (external port)
	if portFwd.SrcDPort != "" {
		ruleParts = append(ruleParts, fmt.Sprintf("th dport %s", portFwd.SrcDPort))
	}

	// DNAT to internal IP:port
	dnatTarget := portFwd.DestIP
	if portFwd.DestPort != "" && portFwd.DestPort != portFwd.SrcDPort {
		dnatTarget = fmt.Sprintf("%s:%s", portFwd.DestIP, portFwd.DestPort)
	}

	ruleParts = append(ruleParts, fmt.Sprintf("counter dnat to %s comment \"%s\"", dnatTarget, portFwd.Comment))

	return strings.Join(ruleParts, " ")
}

// GeneratePortForwardFilterRule generates an nftables filter rule to allow forwarded traffic.
func GeneratePortForwardFilterRule(portFwd PortForward) string {
	var fwdRuleParts []string
	fwdRuleParts = append(fwdRuleParts, fmt.Sprintf("ip daddr %s", portFwd.DestIP))

	if portFwd.Proto != "" {
		fwdRuleParts = append(fwdRuleParts, fmt.Sprintf("meta l4proto %s", portFwd.Proto))
	}

	if portFwd.DestPort != "" {
		fwdRuleParts = append(fwdRuleParts, fmt.Sprintf("th dport %s", portFwd.DestPort))
	} else if portFwd.SrcDPort != "" {
		fwdRuleParts = append(fwdRuleParts, fmt.Sprintf("th dport %s", portFwd.SrcDPort))
	}

	fwdRuleParts = append(fwdRuleParts, fmt.Sprintf("counter accept comment \"Allow-PortForward-%s\"", portFwd.Name))
	return strings.Join(fwdRuleParts, " ")
}

// GenerateCustomRule generates an nftables rule from a custom Rule definition.
func GenerateCustomRule(rule Rule) string {
	var ruleParts []string

	// Special case: established/related connections
	if rule.Name == "Allow-Established-Related" {
		ruleParts = append(ruleParts, "ct state established,related")
	} else {
		if rule.Proto != "" {
			if rule.Proto == "icmp" {
				ruleParts = append(ruleParts, "meta l4proto icmp")
			} else {
				ruleParts = append(ruleParts, fmt.Sprintf("meta l4proto %s", rule.Proto))
			}
		}

		if rule.DestPort != "" {
			ruleParts = append(ruleParts, fmt.Sprintf("th dport %s", rule.DestPort))
		}

		if rule.SrcPort != "" {
			ruleParts = append(ruleParts, fmt.Sprintf("th sport %s", rule.SrcPort))
		}
	}

	target := strings.ToLower(rule.Target)
	ruleParts = append(ruleParts, fmt.Sprintf("counter %s comment \"%s\"", target, rule.Name))

	return strings.Join(ruleParts, " ")
}

// ValidateFirewallConfig performs validation on a firewall configuration.
// Returns an error if the configuration is invalid.
func ValidateFirewallConfig(config *FirewallConfig) error {
	return config.Validate()
}
