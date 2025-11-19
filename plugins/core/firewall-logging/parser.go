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
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// FirewallLogPrefix is the expected prefix in kernel logs for firewall events
	FirewallLogPrefix = "JACK-FW:"
)

// ParseKernelLogLine parses a kernel log line and extracts firewall log information
// Expected format: [timestamp] JACK-FW: ACTION=DROP IN=eth0 OUT= SRC=192.168.1.100 DST=8.8.8.8 PROTO=TCP SPT=54321 DPT=80 LEN=60
// Returns nil if the line is not a firewall log or cannot be parsed
func ParseKernelLogLine(line string) (*FirewallLogEntry, error) {
	// Check if line contains firewall prefix
	if !strings.Contains(line, FirewallLogPrefix) {
		return nil, nil // Not a firewall log, skip silently
	}

	// Extract the log message after the prefix
	prefixIdx := strings.Index(line, FirewallLogPrefix)
	logMsg := line[prefixIdx+len(FirewallLogPrefix):]
	logMsg = strings.TrimSpace(logMsg)

	// Parse key=value pairs
	fields := parseKeyValuePairs(logMsg)

	// Validate required fields
	action, ok := fields["ACTION"]
	if !ok {
		return nil, fmt.Errorf("missing ACTION field")
	}
	srcIP, ok := fields["SRC"]
	if !ok {
		return nil, fmt.Errorf("missing SRC field")
	}
	dstIP, ok := fields["DST"]
	if !ok {
		return nil, fmt.Errorf("missing DST field")
	}

	// Build log entry
	entry := &FirewallLogEntry{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		Action:       action,
		SrcIP:        srcIP,
		DstIP:        dstIP,
		Protocol:     fields["PROTO"],
		InterfaceIn:  fields["IN"],
		InterfaceOut: fields["OUT"],
	}

	// Parse optional integer fields
	if sport, ok := fields["SPT"]; ok {
		if port, err := strconv.Atoi(sport); err == nil && port >= 0 && port <= 65535 {
			entry.SrcPort = port
		}
	}
	if dport, ok := fields["DPT"]; ok {
		if port, err := strconv.Atoi(dport); err == nil && port >= 0 && port <= 65535 {
			entry.DstPort = port
		}
	}
	if length, ok := fields["LEN"]; ok {
		if pktLen, err := strconv.Atoi(length); err == nil && pktLen >= 0 {
			entry.PacketLength = pktLen
		}
	}

	return entry, nil
}

// parseKeyValuePairs extracts key=value pairs from a string
// Example: "ACTION=DROP IN=eth0 SRC=1.2.3.4" -> {"ACTION": "DROP", "IN": "eth0", "SRC": "1.2.3.4"}
func parseKeyValuePairs(s string) map[string]string {
	fields := make(map[string]string)
	parts := strings.Fields(s)

	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			fields[kv[0]] = kv[1]
		}
	}

	return fields
}

// ShouldLogEntry determines if a log entry should be stored based on configuration
func ShouldLogEntry(entry *FirewallLogEntry, config *FirewallLoggingConfig) bool {
	if !config.Enabled {
		return false
	}

	// Check if we should log this action type
	switch entry.Action {
	case "ACCEPT":
		return config.LogAccepts
	case "DROP", "REJECT":
		return config.LogDrops
	default:
		return false
	}
}

// ApplySampling applies sampling rate to determine if entry should be logged
// Returns true if the entry should be logged based on sampling rate
// samplingCounter should be incremented for each call and passed in
func ApplySampling(samplingCounter int, samplingRate int) bool {
	if samplingRate <= 1 {
		return true // Log everything
	}
	return samplingCounter%samplingRate == 0
}
