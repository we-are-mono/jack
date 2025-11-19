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

func TestParseKernelLogLine_ValidDrop(t *testing.T) {
	line := "[12345.678901] JACK-FW: ACTION=DROP IN=eth0 OUT= SRC=192.168.1.100 DST=8.8.8.8 PROTO=TCP SPT=54321 DPT=80 LEN=60"

	entry, err := ParseKernelLogLine(line)
	require.NoError(t, err)
	require.NotNil(t, entry)

	assert.Equal(t, "DROP", entry.Action)
	assert.Equal(t, "192.168.1.100", entry.SrcIP)
	assert.Equal(t, "8.8.8.8", entry.DstIP)
	assert.Equal(t, "TCP", entry.Protocol)
	assert.Equal(t, "eth0", entry.InterfaceIn)
	assert.Equal(t, "", entry.InterfaceOut)
	assert.Equal(t, 54321, entry.SrcPort)
	assert.Equal(t, 80, entry.DstPort)
	assert.Equal(t, 60, entry.PacketLength)
	assert.NotEmpty(t, entry.Timestamp)
}

func TestParseKernelLogLine_ValidAccept(t *testing.T) {
	line := "JACK-FW: ACTION=ACCEPT IN=br-lan OUT=eth0 SRC=10.0.0.50 DST=1.1.1.1 PROTO=UDP SPT=12345 DPT=53 LEN=128"

	entry, err := ParseKernelLogLine(line)
	require.NoError(t, err)
	require.NotNil(t, entry)

	assert.Equal(t, "ACCEPT", entry.Action)
	assert.Equal(t, "10.0.0.50", entry.SrcIP)
	assert.Equal(t, "1.1.1.1", entry.DstIP)
	assert.Equal(t, "UDP", entry.Protocol)
	assert.Equal(t, "br-lan", entry.InterfaceIn)
	assert.Equal(t, "eth0", entry.InterfaceOut)
	assert.Equal(t, 12345, entry.SrcPort)
	assert.Equal(t, 53, entry.DstPort)
	assert.Equal(t, 128, entry.PacketLength)
}

func TestParseKernelLogLine_ICMPWithoutPorts(t *testing.T) {
	line := "JACK-FW: ACTION=DROP IN=eth0 OUT= SRC=192.168.1.100 DST=8.8.8.8 PROTO=ICMP LEN=84"

	entry, err := ParseKernelLogLine(line)
	require.NoError(t, err)
	require.NotNil(t, entry)

	assert.Equal(t, "DROP", entry.Action)
	assert.Equal(t, "ICMP", entry.Protocol)
	assert.Equal(t, 0, entry.SrcPort)
	assert.Equal(t, 0, entry.DstPort)
	assert.Equal(t, 84, entry.PacketLength)
}

func TestParseKernelLogLine_MinimalFields(t *testing.T) {
	line := "JACK-FW: ACTION=DROP SRC=192.168.1.100 DST=8.8.8.8"

	entry, err := ParseKernelLogLine(line)
	require.NoError(t, err)
	require.NotNil(t, entry)

	assert.Equal(t, "DROP", entry.Action)
	assert.Equal(t, "192.168.1.100", entry.SrcIP)
	assert.Equal(t, "8.8.8.8", entry.DstIP)
	assert.Equal(t, "", entry.Protocol)
	assert.Equal(t, "", entry.InterfaceIn)
	assert.Equal(t, 0, entry.SrcPort)
}

func TestParseKernelLogLine_MissingAction(t *testing.T) {
	line := "JACK-FW: SRC=192.168.1.100 DST=8.8.8.8 PROTO=TCP"

	entry, err := ParseKernelLogLine(line)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing ACTION field")
	assert.Nil(t, entry)
}

func TestParseKernelLogLine_MissingSrcIP(t *testing.T) {
	line := "JACK-FW: ACTION=DROP DST=8.8.8.8 PROTO=TCP"

	entry, err := ParseKernelLogLine(line)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing SRC field")
	assert.Nil(t, entry)
}

func TestParseKernelLogLine_MissingDstIP(t *testing.T) {
	line := "JACK-FW: ACTION=DROP SRC=192.168.1.100 PROTO=TCP"

	entry, err := ParseKernelLogLine(line)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing DST field")
	assert.Nil(t, entry)
}

func TestParseKernelLogLine_NotFirewallLog(t *testing.T) {
	lines := []string{
		"[12345.678901] Some other kernel message",
		"Regular syslog message",
		"",
		"eth0: link up",
	}

	for _, line := range lines {
		entry, err := ParseKernelLogLine(line)
		assert.NoError(t, err, "Should not error on non-firewall logs")
		assert.Nil(t, entry, "Should return nil for non-firewall logs")
	}
}

func TestParseKernelLogLine_InvalidPortNumbers(t *testing.T) {
	line := "JACK-FW: ACTION=DROP SRC=192.168.1.100 DST=8.8.8.8 PROTO=TCP SPT=invalid DPT=99999999999"

	entry, err := ParseKernelLogLine(line)
	require.NoError(t, err, "Should not fail on invalid port numbers")
	require.NotNil(t, entry)

	// Invalid ports should be set to 0
	assert.Equal(t, 0, entry.SrcPort)
	assert.Equal(t, 0, entry.DstPort)
}

func TestParseKernelLogLine_IPv6Addresses(t *testing.T) {
	line := "JACK-FW: ACTION=DROP SRC=2001:db8::1 DST=2001:db8::2 PROTO=TCP SPT=443 DPT=8080"

	entry, err := ParseKernelLogLine(line)
	require.NoError(t, err)
	require.NotNil(t, entry)

	assert.Equal(t, "2001:db8::1", entry.SrcIP)
	assert.Equal(t, "2001:db8::2", entry.DstIP)
}

func TestParseKeyValuePairs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:  "simple pairs",
			input: "ACTION=DROP IN=eth0 SRC=1.2.3.4",
			expected: map[string]string{
				"ACTION": "DROP",
				"IN":     "eth0",
				"SRC":    "1.2.3.4",
			},
		},
		{
			name:  "empty values",
			input: "IN= OUT=eth0 ACTION=DROP",
			expected: map[string]string{
				"IN":     "",
				"OUT":    "eth0",
				"ACTION": "DROP",
			},
		},
		{
			name:     "empty string",
			input:    "",
			expected: map[string]string{},
		},
		{
			name:  "values with equals",
			input: "KEY=value=with=equals",
			expected: map[string]string{
				"KEY": "value=with=equals",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseKeyValuePairs(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShouldLogEntry_Disabled(t *testing.T) {
	config := &FirewallLoggingConfig{
		Enabled:    false,
		LogAccepts: true,
		LogDrops:   true,
	}

	entry := &FirewallLogEntry{Action: "DROP"}
	assert.False(t, ShouldLogEntry(entry, config))

	entry = &FirewallLogEntry{Action: "ACCEPT"}
	assert.False(t, ShouldLogEntry(entry, config))
}

func TestShouldLogEntry_LogDropsOnly(t *testing.T) {
	config := &FirewallLoggingConfig{
		Enabled:    true,
		LogAccepts: false,
		LogDrops:   true,
	}

	dropEntry := &FirewallLogEntry{Action: "DROP"}
	assert.True(t, ShouldLogEntry(dropEntry, config))

	rejectEntry := &FirewallLogEntry{Action: "REJECT"}
	assert.True(t, ShouldLogEntry(rejectEntry, config))

	acceptEntry := &FirewallLogEntry{Action: "ACCEPT"}
	assert.False(t, ShouldLogEntry(acceptEntry, config))
}

func TestShouldLogEntry_LogAcceptsOnly(t *testing.T) {
	config := &FirewallLoggingConfig{
		Enabled:    true,
		LogAccepts: true,
		LogDrops:   false,
	}

	acceptEntry := &FirewallLogEntry{Action: "ACCEPT"}
	assert.True(t, ShouldLogEntry(acceptEntry, config))

	dropEntry := &FirewallLogEntry{Action: "DROP"}
	assert.False(t, ShouldLogEntry(dropEntry, config))

	rejectEntry := &FirewallLogEntry{Action: "REJECT"}
	assert.False(t, ShouldLogEntry(rejectEntry, config))
}

func TestShouldLogEntry_LogBoth(t *testing.T) {
	config := &FirewallLoggingConfig{
		Enabled:    true,
		LogAccepts: true,
		LogDrops:   true,
	}

	acceptEntry := &FirewallLogEntry{Action: "ACCEPT"}
	assert.True(t, ShouldLogEntry(acceptEntry, config))

	dropEntry := &FirewallLogEntry{Action: "DROP"}
	assert.True(t, ShouldLogEntry(dropEntry, config))
}

func TestShouldLogEntry_UnknownAction(t *testing.T) {
	config := &FirewallLoggingConfig{
		Enabled:    true,
		LogAccepts: true,
		LogDrops:   true,
	}

	unknownEntry := &FirewallLogEntry{Action: "UNKNOWN"}
	assert.False(t, ShouldLogEntry(unknownEntry, config))
}

func TestApplySampling_NoSampling(t *testing.T) {
	// Sampling rate of 1 means log everything
	for i := 0; i < 100; i++ {
		assert.True(t, ApplySampling(i, 1))
	}
}

func TestApplySampling_EveryOther(t *testing.T) {
	// Sampling rate of 2 means log every other entry
	assert.True(t, ApplySampling(0, 2))
	assert.False(t, ApplySampling(1, 2))
	assert.True(t, ApplySampling(2, 2))
	assert.False(t, ApplySampling(3, 2))
	assert.True(t, ApplySampling(4, 2))
}

func TestApplySampling_EveryTenth(t *testing.T) {
	// Sampling rate of 10 means log every 10th entry
	for i := 0; i < 100; i++ {
		expected := (i % 10) == 0
		assert.Equal(t, expected, ApplySampling(i, 10))
	}
}

func TestApplySampling_ZeroRate(t *testing.T) {
	// Sampling rate of 0 or negative should log everything
	for i := 0; i < 10; i++ {
		assert.True(t, ApplySampling(i, 0))
		assert.True(t, ApplySampling(i, -1))
	}
}
