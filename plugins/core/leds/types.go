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

// LEDConfig is the top-level configuration structure for the LED plugin
type LEDConfig struct {
	LEDs map[string]LEDSettings `json:"leds"` // Map of LED name to settings
}

// LEDSettings contains the configuration for a single LED
type LEDSettings struct {
	// Brightness level (0 to max_brightness, typically 255)
	Brightness int `json:"brightness"`

	// Trigger type: "none", "timer", "heartbeat", "pattern", "netdev", "mmc0", "default-on"
	Trigger string `json:"trigger,omitempty"`

	// Timer trigger parameters (when trigger = "timer")
	DelayOn  int `json:"delay_on,omitempty"`  // Milliseconds on
	DelayOff int `json:"delay_off,omitempty"` // Milliseconds off

	// Pattern trigger parameters (when trigger = "pattern")
	Pattern string `json:"pattern,omitempty"` // Format: "brightness1 ms1 brightness2 ms2 ..."
	Repeat  int    `json:"repeat,omitempty"`  // Repeat count, -1 for infinite

	// Netdev trigger parameters (when trigger = "netdev")
	DeviceName string `json:"device_name,omitempty"` // Network device name (e.g., "br-lan", "wg-proton")
	Mode       string `json:"mode,omitempty"`        // Mode string (e.g., "link tx rx")
}

// LEDState represents the current state of an LED
type LEDState struct {
	Name           string   `json:"name"`
	Brightness     int      `json:"brightness"`
	MaxBrightness  int      `json:"max_brightness"`
	CurrentTrigger string   `json:"current_trigger"`
	AvailTriggers  []string `json:"available_triggers,omitempty"`
}

// LEDStatus is the status response for the plugin
type LEDStatus struct {
	LEDs []LEDState `json:"leds"`
}
