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

// FirewallLoggingConfig defines the configuration structure for firewall logging
type FirewallLoggingConfig struct {
	Enabled          bool `json:"enabled"`
	LogAccepts       bool `json:"log_accepts"`        // Log ACCEPT packets
	LogDrops         bool `json:"log_drops"`          // Log DROP packets
	SamplingRate     int  `json:"sampling_rate"`      // Log 1 out of N packets (1 = all)
	RateLimitPerSec  int  `json:"rate_limit_per_sec"` // Maximum logs per second
	MaxLogEntries    int  `json:"max_log_entries"`    // Maximum entries in database (0 = unlimited)
	RetentionDays    int  `json:"retention_days"`     // Days to keep logs (0 = forever)
	DatabasePath     string `json:"-"` // Read from sqlite3 config, not from this config
}

// FirewallLogEntry represents a single firewall log entry
type FirewallLogEntry struct {
	ID           int64  `json:"id"`
	Timestamp    string `json:"timestamp"`
	Action       string `json:"action"`        // ACCEPT or DROP
	SrcIP        string `json:"src_ip"`
	DstIP        string `json:"dst_ip"`
	Protocol     string `json:"protocol"`      // TCP, UDP, ICMP, etc.
	SrcPort      int    `json:"src_port"`
	DstPort      int    `json:"dst_port"`
	InterfaceIn  string `json:"interface_in"`
	InterfaceOut string `json:"interface_out"`
	PacketLength int    `json:"packet_length"`
	CreatedAt    string `json:"created_at"`
}

// FirewallLogQuery represents query parameters for filtering logs
type FirewallLogQuery struct {
	Action       string `json:"action"`        // Filter by action (ACCEPT/DROP)
	SrcIP        string `json:"src_ip"`        // Filter by source IP
	DstIP        string `json:"dst_ip"`        // Filter by destination IP
	Protocol     string `json:"protocol"`      // Filter by protocol
	InterfaceIn  string `json:"interface_in"`  // Filter by input interface
	InterfaceOut string `json:"interface_out"` // Filter by output interface
	Limit        int    `json:"limit"`         // Maximum number of results (0 = no limit)
	Since        string `json:"since"`         // Only show logs after this timestamp (RFC3339)
}

// FirewallLogStats represents statistics about logged firewall events
type FirewallLogStats struct {
	TotalLogs       int64 `json:"total_logs"`
	AcceptLogs      int64 `json:"accept_logs"`
	DropLogs        int64 `json:"drop_logs"`
	DatabasePath    string `json:"database_path"`
	DatabaseSizeMB  float64 `json:"database_size_mb"`
	OldestLogTime   string `json:"oldest_log_time"`
	NewestLogTime   string `json:"newest_log_time"`
}
