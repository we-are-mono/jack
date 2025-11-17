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

// GenerateMasqueradeRule generates an nftables masquerade rule for an interface.
func GenerateMasqueradeRule(iface string) string {
	return fmt.Sprintf("oifname %s masquerade", iface)
}

// GenerateForwardingRule generates an nftables forwarding rule for a zone.
func GenerateForwardingRule(srcIface string, comment string) string {
	return fmt.Sprintf("iifname %s counter accept comment \"%s\"", srcIface, comment)
}

// GenerateZoneInputJumpRule generates a rule to jump to a zone's input chain.
func GenerateZoneInputJumpRule(iface, inputChain string) string {
	return fmt.Sprintf("iifname %s jump %s", iface, inputChain)
}

// GenerateZoneForwardJumpRule generates a rule to jump to a zone's forward chain.
func GenerateZoneForwardJumpRule(iface, forwardChain string) string {
	return fmt.Sprintf("iifname %s jump %s", iface, forwardChain)
}

// GenerateDefaultPolicyRule generates a default policy rule for a zone chain.
func GenerateDefaultPolicyRule(policy string) string {
	return fmt.Sprintf("counter %s", strings.ToLower(policy))
}

// ValidateFirewallConfig performs validation on a firewall configuration.
// Returns an error if the configuration is invalid.
func ValidateFirewallConfig(config *FirewallConfig) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	if len(config.Zones) == 0 {
		return fmt.Errorf("no zones defined")
	}

	// Validate zones
	for zoneName, zone := range config.Zones {
		if len(zone.Interfaces) == 0 {
			return fmt.Errorf("zone %s has no interfaces", zoneName)
		}

		// Validate policies
		input := strings.ToUpper(zone.Input)
		if input != "ACCEPT" && input != "DROP" && input != "REJECT" {
			return fmt.Errorf("zone %s has invalid input policy: %s (must be ACCEPT, DROP, or REJECT)", zoneName, zone.Input)
		}

		forward := strings.ToUpper(zone.Forward)
		if forward != "ACCEPT" && forward != "DROP" && forward != "REJECT" {
			return fmt.Errorf("zone %s has invalid forward policy: %s (must be ACCEPT, DROP, or REJECT)", zoneName, zone.Forward)
		}
	}

	// Validate forwardings reference existing zones
	for _, fwd := range config.Forwardings {
		if _, exists := config.Zones[fwd.Src]; !exists {
			return fmt.Errorf("forwarding references non-existent source zone: %s", fwd.Src)
		}
		if _, exists := config.Zones[fwd.Dest]; !exists {
			return fmt.Errorf("forwarding references non-existent dest zone: %s", fwd.Dest)
		}
	}

	// Validate rules reference existing zones
	for _, rule := range config.Rules {
		if rule.Src != "" {
			if _, exists := config.Zones[rule.Src]; !exists {
				return fmt.Errorf("rule %s references non-existent source zone: %s", rule.Name, rule.Src)
			}
		}

		// Validate target
		target := strings.ToUpper(rule.Target)
		if target != "ACCEPT" && target != "DROP" && target != "REJECT" {
			return fmt.Errorf("rule %s has invalid target: %s (must be ACCEPT, DROP, or REJECT)", rule.Name, rule.Target)
		}

		// Validate protocol if specified
		if rule.Proto != "" {
			validProtos := []string{"tcp", "udp", "icmp", "icmpv6"}
			valid := false
			for _, validProto := range validProtos {
				if rule.Proto == validProto {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("rule %s has invalid protocol: %s", rule.Name, rule.Proto)
			}
		}
	}

	// Validate port forwards
	for _, pf := range config.PortForwards {
		if pf.DestIP == "" {
			return fmt.Errorf("port forward %s missing destination IP", pf.Name)
		}

		if pf.SrcDPort == "" {
			return fmt.Errorf("port forward %s missing source port", pf.Name)
		}

		// Validate protocol
		if pf.Proto != "" && pf.Proto != "tcp" && pf.Proto != "udp" {
			return fmt.Errorf("port forward %s has invalid protocol: %s (must be tcp or udp)", pf.Name, pf.Proto)
		}
	}

	return nil
}
