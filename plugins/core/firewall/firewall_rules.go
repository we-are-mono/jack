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

// Package main implements firewall rule generation logic for Jack.
package main

import (
	"fmt"
	"strings"
)

// shouldLog determines if a firewall action should be logged based on configuration.
func shouldLog(action string, loggingConfig LoggingConfig) bool {
	if !loggingConfig.Enabled {
		return false
	}

	actionLower := strings.ToLower(action)
	switch actionLower {
	case "accept":
		return loggingConfig.LogAccepts
	case "drop", "reject":
		return loggingConfig.LogDrops
	default:
		return false
	}
}

// getNFLOGGroup returns the NFLOG group number for a given action.
// Group 100 is used for DROP/REJECT, Group 101 is used for ACCEPT.
func getNFLOGGroup(action string) int {
	actionLower := strings.ToLower(action)
	switch actionLower {
	case "accept":
		return 101 // NFLOG group for accepts
	case "drop", "reject":
		return 100 // NFLOG group for drops/rejects
	default:
		return 100 // Default to drops group
	}
}

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
func GeneratePortForwardFilterRule(portFwd PortForward, loggingConfig LoggingConfig) string {
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

	// Add logging if configured
	if shouldLog("accept", loggingConfig) {
		nflogGroup := getNFLOGGroup("accept")
		fwdRuleParts = append(fwdRuleParts, fmt.Sprintf(`log group %d`, nflogGroup))
	}

	fwdRuleParts = append(fwdRuleParts, fmt.Sprintf("counter accept comment \"Allow-PortForward-%s\"", portFwd.Name))
	return strings.Join(fwdRuleParts, " ")
}

// GenerateCustomRule generates an nftables rule from a custom Rule definition.
func GenerateCustomRule(rule Rule, loggingConfig LoggingConfig) string {
	var ruleParts []string

	// Special case: established/related connections
	if rule.Name == "Allow-Established-Related" ||
		strings.HasPrefix(rule.Name, "Allow-Established-Related-") {
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

	// Add logging if configured
	if shouldLog(target, loggingConfig) {
		nflogGroup := getNFLOGGroup(target)
		ruleParts = append(ruleParts, fmt.Sprintf(`log group %d`, nflogGroup))
	}

	ruleParts = append(ruleParts, fmt.Sprintf("counter %s comment \"%s\"", target, rule.Name))

	return strings.Join(ruleParts, " ")
}

// ValidateFirewallConfig performs validation on a firewall configuration.
// Returns an error if the configuration is invalid.
func ValidateFirewallConfig(config *FirewallConfig) error {
	return config.Validate()
}
