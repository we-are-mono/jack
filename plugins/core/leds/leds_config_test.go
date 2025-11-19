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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
				require.Error(t, err, "ValidateNetdevMode() expected error")
				if tt.errContain != "" {
					assert.Contains(t, err.Error(), tt.errContain)
				}
			} else {
				require.NoError(t, err)
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
			assert.Equal(t, tt.want, result, "ParseActiveTrigger(%q)", tt.content)
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
			assert.Equal(t, tt.want, result, "ParseAvailableTriggers(%q)", tt.content)
		})
	}
}
