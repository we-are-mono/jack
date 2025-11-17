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

package main

import (
	"strings"
	"testing"
)

func TestValidateLEDConfig(t *testing.T) {
	tests := []struct {
		name       string
		config     *LEDConfig
		wantError  bool
		errContain string
	}{
		{
			name: "valid config with single LED",
			config: &LEDConfig{
				LEDs: map[string]LEDSettings{
					"status:green": {
						Brightness: 255,
						Trigger:    "none",
					},
				},
			},
			wantError: false,
		},
		{
			name: "valid config with multiple LEDs",
			config: &LEDConfig{
				LEDs: map[string]LEDSettings{
					"status:green": {Brightness: 255, Trigger: "heartbeat"},
					"status:red":   {Brightness: 0, Trigger: "none"},
					"status:blue":  {Brightness: 128, Trigger: "timer", DelayOn: 500, DelayOff: 500},
				},
			},
			wantError: false,
		},
		{
			name:       "nil config",
			config:     nil,
			wantError:  true,
			errContain: "config is nil",
		},
		{
			name: "no LEDs defined",
			config: &LEDConfig{
				LEDs: map[string]LEDSettings{},
			},
			wantError:  true,
			errContain: "no LEDs defined",
		},
		{
			name: "LED with negative brightness",
			config: &LEDConfig{
				LEDs: map[string]LEDSettings{
					"status:green": {Brightness: -1},
				},
			},
			wantError:  true,
			errContain: "brightness cannot be negative",
		},
		{
			name: "LED with invalid trigger",
			config: &LEDConfig{
				LEDs: map[string]LEDSettings{
					"status:green": {Brightness: 255, Trigger: "invalid-trigger"},
				},
			},
			wantError:  true,
			errContain: "invalid trigger",
		},
		{
			name: "timer trigger with negative delay_on",
			config: &LEDConfig{
				LEDs: map[string]LEDSettings{
					"status:green": {Brightness: 255, Trigger: "timer", DelayOn: -1, DelayOff: 500},
				},
			},
			wantError:  true,
			errContain: "delay_on cannot be negative",
		},
		{
			name: "timer trigger with negative delay_off",
			config: &LEDConfig{
				LEDs: map[string]LEDSettings{
					"status:green": {Brightness: 255, Trigger: "timer", DelayOn: 500, DelayOff: -1},
				},
			},
			wantError:  true,
			errContain: "delay_off cannot be negative",
		},
		{
			name: "pattern trigger without pattern",
			config: &LEDConfig{
				LEDs: map[string]LEDSettings{
					"status:green": {Brightness: 255, Trigger: "pattern"},
				},
			},
			wantError:  true,
			errContain: "pattern trigger requires pattern to be set",
		},
		{
			name: "netdev trigger without device_name",
			config: &LEDConfig{
				LEDs: map[string]LEDSettings{
					"status:green": {Brightness: 255, Trigger: "netdev"},
				},
			},
			wantError:  true,
			errContain: "netdev trigger requires device_name to be set",
		},
		{
			name: "netdev trigger with invalid mode",
			config: &LEDConfig{
				LEDs: map[string]LEDSettings{
					"status:green": {
						Brightness: 255,
						Trigger:    "netdev",
						DeviceName: "br-lan",
						Mode:       "invalid-mode",
					},
				},
			},
			wantError:  true,
			errContain: "invalid netdev mode flag",
		},
		{
			name: "valid netdev trigger",
			config: &LEDConfig{
				LEDs: map[string]LEDSettings{
					"status:green": {
						Brightness: 255,
						Trigger:    "netdev",
						DeviceName: "br-lan",
						Mode:       "link tx rx",
					},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLEDConfig(tt.config)

			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateLEDConfig() expected error, got nil")
				} else if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("ValidateLEDConfig() error = %q, want to contain %q", err.Error(), tt.errContain)
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateLEDConfig() unexpected error: %v", err)
			}
		})
	}
}

func TestValidateLEDSettings(t *testing.T) {
	tests := []struct {
		name       string
		ledName    string
		settings   LEDSettings
		wantError  bool
		errContain string
	}{
		{
			name:      "valid settings with none trigger",
			ledName:   "status:green",
			settings:  LEDSettings{Brightness: 255, Trigger: "none"},
			wantError: false,
		},
		{
			name:      "valid settings with heartbeat trigger",
			ledName:   "status:green",
			settings:  LEDSettings{Brightness: 128, Trigger: "heartbeat"},
			wantError: false,
		},
		{
			name:      "valid settings with timer trigger",
			ledName:   "status:green",
			settings:  LEDSettings{Brightness: 255, Trigger: "timer", DelayOn: 500, DelayOff: 500},
			wantError: false,
		},
		{
			name:       "negative brightness",
			ledName:    "status:green",
			settings:   LEDSettings{Brightness: -10},
			wantError:  true,
			errContain: "brightness cannot be negative",
		},
		{
			name:       "invalid trigger",
			ledName:    "status:green",
			settings:   LEDSettings{Brightness: 255, Trigger: "unknown"},
			wantError:  true,
			errContain: "invalid trigger",
		},
		{
			name:       "timer with negative delay_on",
			ledName:    "status:green",
			settings:   LEDSettings{Brightness: 255, Trigger: "timer", DelayOn: -100, DelayOff: 500},
			wantError:  true,
			errContain: "delay_on cannot be negative",
		},
		{
			name:       "timer with negative delay_off",
			ledName:    "status:green",
			settings:   LEDSettings{Brightness: 255, Trigger: "timer", DelayOn: 500, DelayOff: -100},
			wantError:  true,
			errContain: "delay_off cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLEDSettings(tt.ledName, tt.settings)

			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateLEDSettings() expected error, got nil")
				} else if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("ValidateLEDSettings() error = %q, want to contain %q", err.Error(), tt.errContain)
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateLEDSettings() unexpected error: %v", err)
			}
		})
	}
}

func TestValidatePattern(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		wantError  bool
		errContain string
	}{
		{
			name:      "valid pattern with one pair",
			pattern:   "255 1000",
			wantError: false,
		},
		{
			name:      "valid pattern with multiple pairs",
			pattern:   "255 500 0 500 128 1000",
			wantError: false,
		},
		{
			name:       "empty pattern",
			pattern:    "",
			wantError:  true,
			errContain: "pattern cannot be empty",
		},
		{
			name:       "odd number of values",
			pattern:    "255 500 0",
			wantError:  true,
			errContain: "even number of values",
		},
		{
			name:       "single value",
			pattern:    "255",
			wantError:  true,
			errContain: "even number of values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePattern(tt.pattern)

			if tt.wantError {
				if err == nil {
					t.Errorf("ValidatePattern() expected error, got nil")
				} else if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("ValidatePattern() error = %q, want to contain %q", err.Error(), tt.errContain)
				}
				return
			}

			if err != nil {
				t.Errorf("ValidatePattern() unexpected error: %v", err)
			}
		})
	}
}

func TestValidateNetdevMode(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		wantError  bool
		errContain string
	}{
		{
			name:      "valid single flag",
			mode:      "link",
			wantError: false,
		},
		{
			name:      "valid multiple flags",
			mode:      "link tx rx",
			wantError: false,
		},
		{
			name:      "valid all flags",
			mode:      "link tx rx tx_err rx_err full_duplex half_duplex",
			wantError: false,
		},
		{
			name:      "empty mode",
			mode:      "",
			wantError: false,
		},
		{
			name:       "invalid flag",
			mode:       "invalid",
			wantError:  true,
			errContain: "invalid netdev mode flag",
		},
		{
			name:       "mixed valid and invalid flags",
			mode:       "link tx invalid",
			wantError:  true,
			errContain: "invalid netdev mode flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNetdevMode(tt.mode)

			if tt.wantError {
				if err == nil {
					t.Errorf("ValidateNetdevMode() expected error, got nil")
				} else if tt.errContain != "" && !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("ValidateNetdevMode() error = %q, want to contain %q", err.Error(), tt.errContain)
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateNetdevMode() unexpected error: %v", err)
			}
		})
	}
}

func TestParseActiveTrigger(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "active trigger in brackets",
			content: "none timer [heartbeat] pattern netdev",
			want:    "heartbeat",
		},
		{
			name:    "none trigger active",
			content: "[none] timer heartbeat pattern",
			want:    "none",
		},
		{
			name:    "netdev trigger active",
			content: "none timer heartbeat [netdev]",
			want:    "netdev",
		},
		{
			name:    "no brackets - return first",
			content: "none timer heartbeat",
			want:    "none",
		},
		{
			name:    "empty content",
			content: "",
			want:    "",
		},
		{
			name:    "single trigger in brackets",
			content: "[default-on]",
			want:    "default-on",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseActiveTrigger(tt.content)
			if result != tt.want {
				t.Errorf("ParseActiveTrigger(%q) = %q, want %q", tt.content, result, tt.want)
			}
		})
	}
}

func TestParseAvailableTriggers(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "standard triggers with active",
			content: "none timer [heartbeat] pattern netdev",
			want:    []string{"none", "timer", "heartbeat", "pattern", "netdev"},
		},
		{
			name:    "all triggers",
			content: "[none] timer heartbeat pattern netdev mmc0 default-on",
			want:    []string{"none", "timer", "heartbeat", "pattern", "netdev", "mmc0", "default-on"},
		},
		{
			name:    "single trigger",
			content: "[none]",
			want:    []string{"none"},
		},
		{
			name:    "empty content",
			content: "",
			want:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseAvailableTriggers(tt.content)

			if len(result) != len(tt.want) {
				t.Errorf("ParseAvailableTriggers(%q) returned %d triggers, want %d",
					tt.content, len(result), len(tt.want))
				return
			}

			for i, trigger := range result {
				if trigger != tt.want[i] {
					t.Errorf("ParseAvailableTriggers(%q)[%d] = %q, want %q",
						tt.content, i, trigger, tt.want[i])
				}
			}
		})
	}
}
