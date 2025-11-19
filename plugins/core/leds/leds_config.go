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

// Package main implements a self-contained LED control plugin for Jack.
package main

import (
	"fmt"
	"strings"
)

// ParseActiveTrigger parses the trigger file content and returns the active trigger.
// Example input: "none timer [heartbeat] pattern netdev"
// Returns: "heartbeat"
func ParseActiveTrigger(triggerContent string) string {
	// Find the bracketed value
	start := strings.Index(triggerContent, "[")
	end := strings.Index(triggerContent, "]")
	if start != -1 && end != -1 && end > start {
		return triggerContent[start+1 : end]
	}

	// If no brackets found, return the first trigger
	fields := strings.Fields(triggerContent)
	if len(fields) > 0 {
		return fields[0]
	}

	return ""
}

// ParseAvailableTriggers parses the trigger file content and returns all available triggers.
// Example input: "none timer [heartbeat] pattern netdev"
// Returns: []string{"none", "timer", "heartbeat", "pattern", "netdev"}
func ParseAvailableTriggers(triggerContent string) []string {
	// Remove brackets from active trigger
	cleaned := strings.ReplaceAll(triggerContent, "[", "")
	cleaned = strings.ReplaceAll(cleaned, "]", "")

	return strings.Fields(cleaned)
}

// ValidateNetdevMode validates a netdev mode string.
// Valid modes: "link", "tx", "rx", "tx_err", "rx_err", "full_duplex", "half_duplex"
func ValidateNetdevMode(mode string) error {
	if mode == "" {
		return nil // Empty mode is valid (no flags enabled)
	}

	validFlags := map[string]bool{
		"link":        true,
		"tx":          true,
		"rx":          true,
		"tx_err":      true,
		"rx_err":      true,
		"full_duplex": true,
		"half_duplex": true,
	}

	flags := strings.Fields(mode)
	for _, flag := range flags {
		if !validFlags[flag] {
			return fmt.Errorf("invalid netdev mode flag: %s", flag)
		}
	}

	return nil
}
