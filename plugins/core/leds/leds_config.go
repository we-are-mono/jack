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

// ValidateLEDConfig validates an LED configuration.
func ValidateLEDConfig(config *LEDConfig) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	if len(config.LEDs) == 0 {
		return fmt.Errorf("no LEDs defined")
	}

	// Validate each LED configuration
	for ledName, settings := range config.LEDs {
		if err := ValidateLEDSettings(ledName, settings); err != nil {
			return err
		}
	}

	return nil
}

// ValidateLEDSettings validates settings for a single LED.
func ValidateLEDSettings(ledName string, settings LEDSettings) error {
	// Brightness validation (typically 0-255, but we'll be lenient)
	if settings.Brightness < 0 {
		return fmt.Errorf("LED %s: brightness cannot be negative (%d)", ledName, settings.Brightness)
	}

	// Validate trigger if specified
	if settings.Trigger != "" {
		validTriggers := []string{
			"none", "timer", "heartbeat", "pattern", "netdev",
			"mmc0", "mmc1", "default-on", "panic", "disk-activity",
		}

		valid := false
		for _, vt := range validTriggers {
			if settings.Trigger == vt {
				valid = true
				break
			}
		}

		if !valid {
			return fmt.Errorf("LED %s: invalid trigger %q (valid: %s)",
				ledName, settings.Trigger, strings.Join(validTriggers, ", "))
		}
	}

	// Validate timer trigger parameters
	if settings.Trigger == "timer" {
		if settings.DelayOn < 0 {
			return fmt.Errorf("LED %s: delay_on cannot be negative", ledName)
		}
		if settings.DelayOff < 0 {
			return fmt.Errorf("LED %s: delay_off cannot be negative", ledName)
		}
	}

	// Validate pattern trigger parameters
	if settings.Trigger == "pattern" {
		if settings.Pattern == "" {
			return fmt.Errorf("LED %s: pattern trigger requires pattern to be set", ledName)
		}
		if err := ValidatePattern(settings.Pattern); err != nil {
			return fmt.Errorf("LED %s: %w", ledName, err)
		}
	}

	// Validate netdev trigger parameters
	if settings.Trigger == "netdev" {
		if settings.DeviceName == "" {
			return fmt.Errorf("LED %s: netdev trigger requires device_name to be set", ledName)
		}
		if settings.Mode != "" {
			if err := ValidateNetdevMode(settings.Mode); err != nil {
				return fmt.Errorf("LED %s: %w", ledName, err)
			}
		}
	}

	return nil
}

// ValidatePattern validates a pattern string for the pattern trigger.
// Pattern format: "brightness1 duration1 brightness2 duration2 ..."
func ValidatePattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("pattern cannot be empty")
	}

	fields := strings.Fields(pattern)
	if len(fields)%2 != 0 {
		return fmt.Errorf("pattern must have an even number of values (brightness duration pairs)")
	}

	if len(fields) == 0 {
		return fmt.Errorf("pattern must contain at least one brightness/duration pair")
	}

	return nil
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
