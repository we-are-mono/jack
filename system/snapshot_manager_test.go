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
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
)

// TestCaptureSystemSnapshot tests capturing the system state.
func TestCaptureSystemSnapshot(t *testing.T) {
	mockNetlink := NewMockNetlinkClient()
	mockFS := NewMockFilesystemClient()
	mockCmd := NewMockCommandRunner()

	// Setup test interfaces
	attrs0 := netlink.LinkAttrs{Name: "eth0", Index: 1, MTU: 1500, Flags: net.FlagUp}
	mockNetlink.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs0}

	attrs1 := netlink.LinkAttrs{Name: "br-lan", Index: 2, MTU: 1500, Flags: net.FlagUp}
	mockNetlink.Links["br-lan"] = &netlink.Bridge{LinkAttrs: attrs1}

	// Setup addresses
	mockNetlink.Addresses["eth0"] = []netlink.Addr{
		{IPNet: mustParseCIDR("192.168.1.10/24")},
	}
	mockNetlink.Addresses["br-lan"] = []netlink.Addr{
		{IPNet: mustParseCIDR("10.0.0.1/24")},
	}

	// Setup routes
	mockNetlink.Routes = []netlink.Route{
		{
			Dst:       nil, // default route
			Gw:        net.ParseIP("192.168.1.1"),
			LinkIndex: 1,
			Priority:  100,
			Table:     254,
		},
	}

	// Setup IP forwarding
	mockFS.Files["/proc/sys/net/ipv4/ip_forward"] = []byte("1")

	// Setup nftables output
	mockCmd.CommandOutputs = make(map[string][]byte)
	mockCmd.CommandOutputs["nft -j list table inet jack"] = []byte(`{"nftables": [{"table": {"family": "inet", "name": "jack"}}]}`)

	sm := NewSnapshotManager(mockNetlink, mockFS, mockCmd)
	snapshot, err := sm.CaptureSystemSnapshot()
	require.NoError(t, err)

	// Verify IP forwarding captured
	assert.True(t, snapshot.IPForwarding, "Expected IP forwarding to be true")

	// Verify interfaces captured
	require.Len(t, snapshot.Interfaces, 2, "Expected 2 interfaces")

	// Verify eth0 captured correctly
	eth0, ok := snapshot.Interfaces["eth0"]
	require.True(t, ok, "eth0 not in snapshot")
	assert.Equal(t, "eth0", eth0.Name)
	assert.Equal(t, 1500, eth0.MTU)
	assert.Equal(t, "up", eth0.State)

	// Note: IPNet.String() normalizes to network address (192.168.1.0/24, not 192.168.1.10/24)
	require.Len(t, eth0.Addresses, 1, "Expected 1 address")
	assert.Equal(t, "192.168.1.0/24", eth0.Addresses[0])

	// Verify routes captured
	require.Len(t, snapshot.Routes, 1, "Expected 1 route")
	assert.Equal(t, "default", snapshot.Routes[0].Destination)

	// Verify nftables captured
	assert.NotEmpty(t, snapshot.NftablesRules, "Expected nftables rules to be captured")
}

// TestCaptureInterfaceState_Bridge tests capturing bridge interface state.
func TestCaptureInterfaceState_Bridge(t *testing.T) {
	mockNetlink := NewMockNetlinkClient()
	mockFS := NewMockFilesystemClient()
	mockCmd := NewMockCommandRunner()

	// Setup bridge
	brAttrs := netlink.LinkAttrs{Name: "br-lan", Index: 1, MTU: 1500, Flags: net.FlagUp}
	bridge := &netlink.Bridge{LinkAttrs: brAttrs}
	mockNetlink.Links["br-lan"] = bridge

	// Setup bridge ports
	attrs0 := netlink.LinkAttrs{Name: "eth0", Index: 2, MasterIndex: 1}
	mockNetlink.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs0}

	attrs1 := netlink.LinkAttrs{Name: "eth1", Index: 3, MasterIndex: 1}
	mockNetlink.Links["eth1"] = &netlink.Dummy{LinkAttrs: attrs1}

	mockNetlink.Addresses["br-lan"] = []netlink.Addr{
		{IPNet: mustParseCIDR("192.168.1.1/24")},
	}

	sm := NewSnapshotManager(mockNetlink, mockFS, mockCmd)
	snapshot, err := sm.captureInterfaceState(bridge)
	require.NoError(t, err)

	assert.Equal(t, "bridge", snapshot.Type)
	require.Len(t, snapshot.Ports, 2, "Expected 2 ports")

	portMap := make(map[string]bool)
	for _, port := range snapshot.Ports {
		portMap[port] = true
	}
	assert.True(t, portMap["eth0"] && portMap["eth1"], "Expected ports eth0 and eth1, got %v", snapshot.Ports)
}

// TestSnapshotManager_GetBridgePorts tests bridge port enumeration.
func TestSnapshotManager_GetBridgePorts(t *testing.T) {
	mockNetlink := NewMockNetlinkClient()
	mockFS := NewMockFilesystemClient()
	mockCmd := NewMockCommandRunner()

	// Setup bridge
	brAttrs := netlink.LinkAttrs{Name: "br-lan", Index: 1}
	mockNetlink.Links["br-lan"] = &netlink.Bridge{LinkAttrs: brAttrs}

	// Setup ports
	attrs0 := netlink.LinkAttrs{Name: "eth0", Index: 2, MasterIndex: 1}
	mockNetlink.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs0}

	attrs1 := netlink.LinkAttrs{Name: "eth1", Index: 3, MasterIndex: 1}
	mockNetlink.Links["eth1"] = &netlink.Dummy{LinkAttrs: attrs1}

	// Non-bridge port
	attrs2 := netlink.LinkAttrs{Name: "eth2", Index: 4, MasterIndex: 0}
	mockNetlink.Links["eth2"] = &netlink.Dummy{LinkAttrs: attrs2}

	sm := NewSnapshotManager(mockNetlink, mockFS, mockCmd)
	ports, err := sm.getBridgePorts("br-lan")
	require.NoError(t, err)

	require.Len(t, ports, 2, "Expected 2 ports")

	portMap := make(map[string]bool)
	for _, port := range ports {
		portMap[port] = true
	}
	assert.True(t, portMap["eth0"] && portMap["eth1"], "Expected eth0 and eth1, got %v", ports)
	assert.False(t, portMap["eth2"], "eth2 should not be in ports")
}

// TestSnapshotRoutesMatch tests route matching logic.
func TestSnapshotRoutesMatch(t *testing.T) {
	mockNetlink := NewMockNetlinkClient()
	mockFS := NewMockFilesystemClient()
	mockCmd := NewMockCommandRunner()

	// Add test interfaces to mock for LinkByIndex lookups
	attrs0 := netlink.LinkAttrs{Name: "eth0", Index: 1}
	mockNetlink.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs0}

	attrs1 := netlink.LinkAttrs{Name: "eth1", Index: 2}
	mockNetlink.Links["eth1"] = &netlink.Dummy{LinkAttrs: attrs1}

	sm := NewSnapshotManager(mockNetlink, mockFS, mockCmd)

	tests := []struct {
		name     string
		route    netlink.Route
		snapshot RouteSnapshot
		expected bool
	}{
		{
			name: "exact match - default route",
			route: netlink.Route{
				Dst:       nil,
				Gw:        net.ParseIP("192.168.1.1"),
				LinkIndex: 1,
				Priority:  100,
				Table:     254,
			},
			snapshot: RouteSnapshot{
				Destination: "default",
				Gateway:     "192.168.1.1",
				Device:      "eth0",
				Metric:      100,
				Table:       254,
			},
			expected: true,
		},
		{
			name: "exact match - specific destination",
			route: netlink.Route{
				Dst:       mustParseCIDR("10.0.0.0/8"),
				Gw:        net.ParseIP("192.168.1.254"),
				LinkIndex: 1,
				Priority:  200,
				Table:     254,
			},
			snapshot: RouteSnapshot{
				Destination: "10.0.0.0/8",
				Gateway:     "192.168.1.254",
				Device:      "eth0",
				Metric:      200,
				Table:       254,
			},
			expected: true,
		},
		{
			name: "different destination",
			route: netlink.Route{
				Dst:       mustParseCIDR("192.168.1.0/24"),
				Gw:        net.ParseIP("192.168.1.1"),
				LinkIndex: 1,
				Priority:  100,
				Table:     254,
			},
			snapshot: RouteSnapshot{
				Destination: "10.0.0.0/8",
				Gateway:     "192.168.1.1",
				Device:      "eth0",
				Metric:      100,
				Table:       254,
			},
			expected: false,
		},
		{
			name: "different gateway",
			route: netlink.Route{
				Dst:       nil,
				Gw:        net.ParseIP("192.168.1.1"),
				LinkIndex: 1,
				Priority:  100,
				Table:     254,
			},
			snapshot: RouteSnapshot{
				Destination: "default",
				Gateway:     "192.168.1.254",
				Device:      "eth0",
				Metric:      100,
				Table:       254,
			},
			expected: false,
		},
		{
			name: "different metric",
			route: netlink.Route{
				Dst:       nil,
				Gw:        net.ParseIP("192.168.1.1"),
				LinkIndex: 1,
				Priority:  100,
				Table:     254,
			},
			snapshot: RouteSnapshot{
				Destination: "default",
				Gateway:     "192.168.1.1",
				Device:      "eth0",
				Metric:      200,
				Table:       254,
			},
			expected: false,
		},
		{
			name: "route without gateway",
			route: netlink.Route{
				Dst:       mustParseCIDR("192.168.1.0/24"),
				Gw:        nil,
				LinkIndex: 1,
				Priority:  0,
				Table:     254,
			},
			snapshot: RouteSnapshot{
				Destination: "192.168.1.0/24",
				Gateway:     "",
				Device:      "eth0",
				Metric:      0,
				Table:       254,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sm.snapshotRoutesMatch(tt.route, tt.snapshot)
			if result != tt.expected {
				t.Errorf("snapshotRoutesMatch() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestRollbackIPForwarding tests IP forwarding rollback.
func TestRollbackIPForwarding(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		want    string
	}{
		{"enable IP forwarding", true, "1"},
		{"disable IP forwarding", false, "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNetlink := NewMockNetlinkClient()
			mockFS := NewMockFilesystemClient()
			mockCmd := NewMockCommandRunner()

			sm := NewSnapshotManager(mockNetlink, mockFS, mockCmd)
			err := sm.rollbackIPForwarding(tt.enabled)

			if err != nil {
				t.Fatalf("rollbackIPForwarding failed: %v", err)
			}

			value, ok := mockFS.Files["/proc/sys/net/ipv4/ip_forward"]
			if !ok {
				t.Fatal("IP forwarding file not written")
			}

			if string(value) != tt.want {
				t.Errorf("Expected %s, got %s", tt.want, string(value))
			}
		})
	}
}

// TestRestoreSnapshot tests restoring system state.
func TestRestoreSnapshot(t *testing.T) {
	mockNetlink := NewMockNetlinkClient()
	mockFS := NewMockFilesystemClient()
	mockCmd := NewMockCommandRunner()

	// Setup current state (after apply)
	attrs0 := netlink.LinkAttrs{Name: "eth0", Index: 1, MTU: 9000, Flags: net.FlagUp}
	mockNetlink.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs0}

	mockNetlink.Addresses["eth0"] = []netlink.Addr{
		{IPNet: mustParseCIDR("192.168.1.20/24")},
	}

	mockFS.Files["/proc/sys/net/ipv4/ip_forward"] = []byte("1")

	// Snapshot state (before apply)
	snapshot := &SystemSnapshot{
		IPForwarding: false,
		Interfaces: map[string]InterfaceSnapshot{
			"eth0": {
				Name:      "eth0",
				Type:      "physical",
				Existed:   true,
				MTU:       1500,
				State:     "up",
				Addresses: []string{"192.168.1.10/24"},
			},
		},
		Routes: []RouteSnapshot{},
	}

	sm := NewSnapshotManager(mockNetlink, mockFS, mockCmd)
	err := sm.RestoreSnapshot(snapshot, []string{"all"})

	if err != nil {
		t.Fatalf("RestoreSnapshot failed: %v", err)
	}

	// Verify IP forwarding restored
	value := mockFS.Files["/proc/sys/net/ipv4/ip_forward"]
	if string(value) != "0" {
		t.Errorf("Expected IP forwarding disabled, got %s", string(value))
	}

	// Verify MTU restored
	if mockNetlink.LinkSetMTUCalls != 1 {
		t.Errorf("Expected 1 LinkSetMTU call, got %d", mockNetlink.LinkSetMTUCalls)
	}

	// Verify old IP removed
	if mockNetlink.AddrDelCalls != 1 {
		t.Errorf("Expected 1 AddrDel call, got %d", mockNetlink.AddrDelCalls)
	}

	// Verify new IP added
	if mockNetlink.AddrAddCalls != 1 {
		t.Errorf("Expected 1 AddrAdd call, got %d", mockNetlink.AddrAddCalls)
	}
}

// TestRestoreSnapshot_Scopes tests different restore scopes.
func TestRestoreSnapshot_Scopes(t *testing.T) {
	tests := []struct {
		name              string
		scope             []string
		expectIPForward   bool
		expectInterfaces  bool
		expectRoutes      bool
	}{
		{"all scope", []string{"all"}, true, true, true},
		{"ipforward only", []string{"ipforward"}, true, false, false},
		{"interfaces only", []string{"interfaces"}, false, true, false},
		{"routes only", []string{"routes"}, false, false, true},
		{"multiple scopes", []string{"ipforward", "routes"}, true, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNetlink := NewMockNetlinkClient()
			mockFS := NewMockFilesystemClient()
			mockCmd := NewMockCommandRunner()

			// Setup current state
			attrs0 := netlink.LinkAttrs{Name: "eth0", Index: 1, MTU: 1500}
			mockNetlink.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs0}
			mockNetlink.Addresses["eth0"] = []netlink.Addr{}
			mockFS.Files["/proc/sys/net/ipv4/ip_forward"] = []byte("1")

			snapshot := &SystemSnapshot{
				IPForwarding: false,
				Interfaces: map[string]InterfaceSnapshot{
					"eth0": {
						Name:      "eth0",
						Existed:   true,
						MTU:       1500,
						State:     "up",
						Addresses: []string{},
					},
				},
				Routes: []RouteSnapshot{},
			}

			sm := NewSnapshotManager(mockNetlink, mockFS, mockCmd)

			// Reset call counters before restore
			mockFS.WriteFileCalls = 0
			mockNetlink.LinkListCalls = 0

			err := sm.RestoreSnapshot(snapshot, tt.scope)

			if err != nil {
				t.Fatalf("RestoreSnapshot failed: %v", err)
			}

			// Check IP forwarding - use WriteFileCalls to detect if rollbackIPForwarding was called
			ipForwardCalled := mockFS.WriteFileCalls > 0
			if ipForwardCalled != tt.expectIPForward {
				t.Errorf("IP forward called = %v, expected %v (WriteFileCalls=%d)", ipForwardCalled, tt.expectIPForward, mockFS.WriteFileCalls)
			}

			// Check interfaces
			interfacesCalled := mockNetlink.LinkListCalls > 0
			if interfacesCalled != tt.expectInterfaces {
				t.Errorf("Interfaces called = %v, expected %v", interfacesCalled, tt.expectInterfaces)
			}

			// Note: Routes would be similar but we're testing scope logic here
		})
	}
}
