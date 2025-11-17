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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type NftablesProvider struct {
	tableName     string
	configHashPath string
}

func NewNftables() (*NftablesProvider, error) {
	// Check if nft is available
	if _, err := exec.LookPath("nft"); err != nil {
		return nil, fmt.Errorf("nftables not found: %w", err)
	}

	return &NftablesProvider{
		tableName:      "jack",
		configHashPath: "/var/lib/jack/nftables-config.hash",
	}, nil
}

func (n *NftablesProvider) ApplyConfig(config *FirewallConfig) error {
	fmt.Println("  Applying firewall configuration...")

	// Calculate hash of new config
	newHash, err := n.calculateConfigHash(config)
	if err != nil {
		return fmt.Errorf("failed to calculate config hash: %w", err)
	}

	// Read last applied hash
	lastHash, err := n.readConfigHash()
	if err == nil && lastHash == newHash {
		// Hash matches, but verify the table actually exists before skipping
		if n.tableExists() {
			fmt.Println("    [OK] Firewall configuration unchanged, skipping rebuild")
			return nil
		}
		fmt.Println("    [INFO] Table doesn't exist, rebuilding despite matching hash...")
	}

	if err == nil {
		fmt.Println("    [INFO] Firewall configuration changed, rebuilding...")
	} else {
		fmt.Println("    [INFO] No previous firewall configuration, building...")
	}

	// Flush existing rules
	if err := n.Flush(); err != nil {
		return fmt.Errorf("failed to flush rules: %w", err)
	}

	// Create our table
	if err := n.createTable(); err != nil {
		return err
	}

	// Add global connection tracking rules to base chains
	if err := n.addGlobalRules(); err != nil {
		return fmt.Errorf("failed to add global rules: %w", err)
	}

	// Create chains for each zone (without default policies)
	for zoneName, zone := range config.Zones {
		if err := n.createZoneChains(zoneName, zone); err != nil {
			return fmt.Errorf("failed to create zone %s: %w", zoneName, err)
		}
	}

	// Apply custom rules FIRST (before default policies)
	for _, rule := range config.Rules {
		if err := n.applyRule(rule); err != nil {
			return fmt.Errorf("failed to apply rule %s: %w", rule.Name, err)
		}
	}

	// Apply forwarding rules
	for _, fwd := range config.Forwardings {
		if err := n.applyForwarding(fwd, config.Zones); err != nil {
			return fmt.Errorf("failed to apply forwarding: %w", err)
		}
	}

	// Apply NAT/masquerade for zones
	for zoneName, zone := range config.Zones {
		if zone.Masquerade {
			if err := n.applyMasquerade(zoneName, zone); err != nil {
				return fmt.Errorf("failed to apply masquerade for %s: %w", zoneName, err)
			}
		}
	}

	// Apply port forwarding rules (DNAT)
	for _, portFwd := range config.PortForwards {
		if portFwd.Enabled {
			if err := n.applyPortForward(portFwd); err != nil {
				return fmt.Errorf("failed to apply port forward %s: %w", portFwd.Name, err)
			}
		}
	}

	// Apply default policies LAST (so allow rules are checked first)
	if err := n.applyDefaultPolicies(config); err != nil {
		return fmt.Errorf("failed to apply default policies: %w", err)
	}

	// Save hash of successfully applied config
	if err := n.saveConfigHash(newHash); err != nil {
		// Log warning but don't fail - config was applied successfully
		fmt.Printf("    [WARN] Failed to save config hash: %v\n", err)
	}

	fmt.Println("    [OK] Firewall configured")
	return nil
}

func (n *NftablesProvider) createTable() error {
	// Create inet table (handles both IPv4 and IPv6)
	cmd := exec.Command("nft", "add", "table", "inet", n.tableName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}
	return nil
}

func (n *NftablesProvider) createZoneChains(zoneName string, zone Zone) error {
	// Create input chain for this zone
	inputChain := fmt.Sprintf("input_%s", zoneName)
	if err := n.runNft("add", "chain", "inet", n.tableName, inputChain); err != nil {
		return err
	}

	// Create forward chain for this zone
	forwardChain := fmt.Sprintf("forward_%s", zoneName)
	if err := n.runNft("add", "chain", "inet", n.tableName, forwardChain); err != nil {
		return err
	}

	// Create output chain for this zone
	outputChain := fmt.Sprintf("output_%s", zoneName)
	if err := n.runNft("add", "chain", "inet", n.tableName, outputChain); err != nil {
		return err
	}

	// Set up interface jumps to zone chains
	for _, iface := range zone.Interfaces {
		// Input: packets coming TO this device on this interface
		rule := fmt.Sprintf("iifname %s jump %s", iface, inputChain)
		if err := n.addRule("filter", "input", rule); err != nil {
			return err
		}

		// Forward: packets passing THROUGH this device
		rule = fmt.Sprintf("iifname %s jump %s", iface, forwardChain)
		if err := n.addRule("filter", "forward", rule); err != nil {
			return err
		}
	}

	// DON'T add default policy here - it will be added after custom rules

	fmt.Printf("    [OK] Created zone: %s\n", zoneName)
	return nil
}

func (n *NftablesProvider) applyDefaultPolicies(config *FirewallConfig) error {
	// Apply default policies for each zone (these go at the END of chains)
	for zoneName, zone := range config.Zones {
		inputChain := fmt.Sprintf("input_%s", zoneName)
		forwardChain := fmt.Sprintf("forward_%s", zoneName)

		inputPolicy := strings.ToLower(zone.Input)
		forwardPolicy := strings.ToLower(zone.Forward)

		// Add default policy as LAST rule in each chain
		if err := n.addRule("filter", inputChain, fmt.Sprintf("counter %s", inputPolicy)); err != nil {
			return err
		}

		if err := n.addRule("filter", forwardChain, fmt.Sprintf("counter %s", forwardPolicy)); err != nil {
			return err
		}
	}

	return nil
}

func (n *NftablesProvider) applyForwarding(fwd Forwarding, zones map[string]Zone) error {
	srcZone := zones[fwd.Src]

	srcChain := fmt.Sprintf("forward_%s", fwd.Src)

	// Allow forwarding from src zone to dest zone
	for _, srcIface := range srcZone.Interfaces {
		rule := fmt.Sprintf("iifname %s counter accept comment \"%s\"", srcIface, fwd.Comment)
		if err := n.addRule("filter", srcChain, rule); err != nil {
			return err
		}
	}

	fmt.Printf("    [OK] Forwarding: %s → %s\n", fwd.Src, fwd.Dest)
	return nil
}

func (n *NftablesProvider) applyMasquerade(zoneName string, zone Zone) error {
	// Create NAT postrouting chain if it doesn't exist
	if err := n.runNft("add", "chain", "inet", n.tableName, "postrouting",
		"{", "type", "nat", "hook", "postrouting", "priority", "100", ";", "}"); err != nil {
		// Chain might already exist, that's ok
	}

	// Add masquerade rule for each interface in this zone
	for _, iface := range zone.Interfaces {
		rule := fmt.Sprintf("oifname %s masquerade", iface)
		if err := n.addRule("nat", "postrouting", rule); err != nil {
			return err
		}
	}

	fmt.Printf("    [OK] NAT masquerade enabled for zone: %s\n", zoneName)
	return nil
}

func (n *NftablesProvider) applyPortForward(portFwd PortForward) error {
	// Create NAT prerouting chain if it doesn't exist
	if err := n.runNft("add", "chain", "inet", n.tableName, "prerouting",
		"{", "type", "nat", "hook", "prerouting", "priority", "-100", ";", "}"); err != nil {
		// Chain might already exist, that's ok
	}

	// Build the DNAT rule using pure function
	ruleStr := GeneratePortForwardDNATRule(portFwd)
	if err := n.addRule("nat", "prerouting", ruleStr); err != nil {
		return err
	}

	// Build the filter rule using pure function
	fwdRuleStr := GeneratePortForwardFilterRule(portFwd)
	if err := n.addRule("filter", "forward", fwdRuleStr); err != nil {
		return err
	}

	fmt.Printf("    [OK] Port forward: %s:%s → %s:%s (%s)\n",
		portFwd.Src, portFwd.SrcDPort, portFwd.DestIP,
		getDestPort(portFwd), portFwd.Name)

	return nil
}

// Helper to get the destination port for display
func getDestPort(portFwd PortForward) string {
	if portFwd.DestPort != "" {
		return portFwd.DestPort
	}
	return portFwd.SrcDPort
}

func (n *NftablesProvider) applyRule(rule Rule) error {
	srcChain := fmt.Sprintf("input_%s", rule.Src)

	// Build the rule using pure function
	ruleStr := GenerateCustomRule(rule)
	if err := n.addRule("filter", srcChain, ruleStr); err != nil {
		return err
	}

	fmt.Printf("    [OK] Rule applied: %s\n", rule.Name)
	return nil
}

func (n *NftablesProvider) addRule(chainType, chainName, rule string) error {
	// Ensure base chains exist
	if chainName == "input" || chainName == "forward" || chainName == "output" {
		n.ensureBaseChain(chainType, chainName)
	}

	return n.runNft("add", "rule", "inet", n.tableName, chainName, rule)
}

func (n *NftablesProvider) ensureBaseChain(chainType, chainName string) error {
	var hookType, priority string

	switch chainName {
	case "input":
		hookType = "input"
		priority = "0"
	case "forward":
		hookType = "forward"
		priority = "0"
	case "output":
		hookType = "output"
		priority = "0"
	}

	if chainType == "nat" {
		priority = "100"
	}

	// Try to create the base chain (ignore error if exists)
	chainDef := fmt.Sprintf("{ type %s hook %s priority %s; }", chainType, hookType, priority)
	cmd := exec.Command("nft", "add", "chain", "inet", n.tableName, chainName, chainDef)
	cmd.Run() // Ignore error if chain already exists

	return nil
}

func (n *NftablesProvider) Flush() error {
	// Delete our table (this removes all our rules)
	cmd := exec.Command("nft", "delete", "table", "inet", n.tableName)
	cmd.Run() // Ignore error if table doesn't exist

	fmt.Println("    [OK] Flushed firewall rules")
	return nil
}

func (n *NftablesProvider) Validate(config *FirewallConfig) error {
	// Use comprehensive validation from pure function
	return ValidateFirewallConfig(config)
}

func (n *NftablesProvider) Enable() error {
	// nftables doesn't have a global enable/disable
	// Rules are active as soon as they're added
	return nil
}

func (n *NftablesProvider) Disable() error {
	return n.Flush()
}

// Status represents the firewall status
type Status struct {
	Enabled   bool   `json:"enabled"`
	Backend   string `json:"backend"`
	RuleCount int    `json:"rule_count"`
}

// tableExists checks if the nftables table exists on the system
func (n *NftablesProvider) tableExists() bool {
	cmd := exec.Command("nft", "list", "table", "inet", n.tableName)
	err := cmd.Run()
	return err == nil
}

func (n *NftablesProvider) Status() (*Status, error) {
	// Check if our table exists and get rule count
	cmd := exec.Command("nft", "list", "table", "inet", n.tableName)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return &Status{
			Enabled:   false,
			Backend:   "nftables",
			RuleCount: 0,
		}, nil
	}

	// Count rules (count lines that start with whitespace and don't contain "chain", "table", or "}")
	ruleCount := 0
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Count lines that look like rules (start with whitespace, not chain/table definitions)
		if len(line) > 0 && (line[0] == '\t' || line[0] == ' ') &&
			!strings.Contains(trimmed, "chain ") &&
			!strings.Contains(trimmed, "table ") &&
			trimmed != "{" && trimmed != "}" && trimmed != "" {
			ruleCount++
		}
	}

	return &Status{
		Enabled:   true,
		Backend:   "nftables",
		RuleCount: ruleCount,
	}, nil
}

func (n *NftablesProvider) runNft(args ...string) error {
	cmd := exec.Command("nft", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("nft command failed: %s: %w", string(output), err)
	}
	return nil
}

func (n *NftablesProvider) addGlobalRules() error {
	// Allow established/related connections in forward chain (for return traffic)
	if err := n.addRule("filter", "forward", "ct state established,related counter accept"); err != nil {
		return err
	}

	return nil
}

// calculateConfigHash generates a SHA256 hash of the config for comparison
func (n *NftablesProvider) calculateConfigHash(config *FirewallConfig) (string, error) {
	// Serialize config to JSON for consistent hashing
	jsonData, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	// Calculate SHA256 hash
	hash := sha256.Sum256(jsonData)
	return fmt.Sprintf("%x", hash), nil
}

// readConfigHash reads the stored config hash from disk
func (n *NftablesProvider) readConfigHash() (string, error) {
	data, err := os.ReadFile(n.configHashPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// saveConfigHash saves the config hash to disk
func (n *NftablesProvider) saveConfigHash(hash string) error {
	// Ensure directory exists
	dir := "/var/lib/jack"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write hash to file
	if err := os.WriteFile(n.configHashPath, []byte(hash), 0644); err != nil {
		return fmt.Errorf("failed to write hash: %w", err)
	}

	return nil
}
