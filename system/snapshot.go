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

package system

import (
	"time"
)

// SystemSnapshot captures the complete state of the system before apply operations.
// This allows rollback to a known good state if apply fails.
type SystemSnapshot struct {
	Timestamp    time.Time                  `json:"timestamp"`
	CheckpointID string                     `json:"checkpoint_id"`
	IPForwarding bool                       `json:"ip_forwarding"`
	Interfaces   map[string]InterfaceSnapshot `json:"interfaces"`
	Routes       []RouteSnapshot            `json:"routes"`
	NftablesRules string                    `json:"nftables_rules,omitempty"`
}

// InterfaceSnapshot captures the state of a single network interface.
type InterfaceSnapshot struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"` // "physical", "bridge", "vlan", "wireguard"
	Existed      bool     `json:"existed"`
	MTU          int      `json:"mtu"`
	State        string   `json:"state"` // "up" or "down"
	Addresses    []string `json:"addresses"`
	Gateway      string   `json:"gateway,omitempty"`
	Metric       int      `json:"metric,omitempty"`

	// Bridge-specific fields
	Ports        []string `json:"ports,omitempty"`

	// VLAN-specific fields
	VLANId       int      `json:"vlan_id,omitempty"`
	ParentDevice string   `json:"parent_device,omitempty"`
}

// RouteSnapshot captures the state of a single route.
type RouteSnapshot struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway,omitempty"`
	Device      string `json:"device,omitempty"`
	Metric      int    `json:"metric"`
	Table       int    `json:"table"`
	Scope       int    `json:"scope"`
}

// Default snapshot manager for backwards compatibility.
var defaultSnapshotManager = NewDefaultSnapshotManager()

// CaptureSystemSnapshot captures the current state of the system.
// For testing, create a SnapshotManager with injected dependencies.
func CaptureSystemSnapshot() (*SystemSnapshot, error) {
	return defaultSnapshotManager.CaptureSystemSnapshot()
}
