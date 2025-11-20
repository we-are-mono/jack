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
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

const (
	// IP protocol numbers
	IPProtoTCP  = 6
	IPProtoUDP  = 17
	IPProtoICMP = 1
)

// ParseIPPacket parses a binary IP packet from NFLOG payload
// Returns a FirewallLogEntry with extracted packet information
func ParseIPPacket(payload []byte, action string) (*FirewallLogEntry, error) {
	// Minimum IPv4 header size is 20 bytes
	if len(payload) < 20 {
		return nil, fmt.Errorf("packet too short: %d bytes", len(payload))
	}

	// Check IP version (first 4 bits of first byte)
	ipVersion := payload[0] >> 4
	if ipVersion != 4 {
		// Only support IPv4 for now
		return nil, fmt.Errorf("unsupported IP version: %d", ipVersion)
	}

	// Parse IPv4 header
	ihl := int(payload[0] & 0x0F) * 4 // Internet Header Length in bytes
	if len(payload) < ihl {
		return nil, fmt.Errorf("packet shorter than IHL: %d < %d", len(payload), ihl)
	}

	totalLength := int(binary.BigEndian.Uint16(payload[2:4]))
	protocol := payload[9]
	srcIP := net.IP(payload[12:16]).String()
	dstIP := net.IP(payload[16:20]).String()

	// Create log entry
	entry := &FirewallLogEntry{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		Action:       action,
		SrcIP:        srcIP,
		DstIP:        dstIP,
		PacketLength: totalLength,
	}

	// Parse protocol-specific information
	switch protocol {
	case IPProtoTCP:
		entry.Protocol = "TCP"
		if len(payload) >= ihl+4 {
			entry.SrcPort = int(binary.BigEndian.Uint16(payload[ihl : ihl+2]))
			entry.DstPort = int(binary.BigEndian.Uint16(payload[ihl+2 : ihl+4]))
		}
	case IPProtoUDP:
		entry.Protocol = "UDP"
		if len(payload) >= ihl+4 {
			entry.SrcPort = int(binary.BigEndian.Uint16(payload[ihl : ihl+2]))
			entry.DstPort = int(binary.BigEndian.Uint16(payload[ihl+2 : ihl+4]))
		}
	case IPProtoICMP:
		entry.Protocol = "ICMP"
	default:
		entry.Protocol = fmt.Sprintf("PROTO-%d", protocol)
	}

	return entry, nil
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
