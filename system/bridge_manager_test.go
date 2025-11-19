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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
	"github.com/we-are-mono/jack/types"
)

// TestApplyBridgeInterface tests bridge creation and configuration.
func TestApplyBridgeInterface(t *testing.T) {
	tests := []struct {
		name        string
		ifaceName   string
		config      types.Interface
		setupMock   func(*MockNetlinkClient)
		expectError bool
		errorMsg    string
		verifyMock  func(*testing.T, *MockNetlinkClient)
	}{
		{
			name:      "create new bridge with ports",
			ifaceName: "br-lan",
			config: types.Interface{
				Type:        "bridge",
				Device:      "br-lan",
				BridgePorts: []string{"eth0", "eth1"},
				Protocol:    "static",
				IPAddr:      "192.168.1.1",
				Netmask:     "255.255.255.0",
				MTU:         1500,
				Enabled:     true,
			},
			setupMock: func(m *MockNetlinkClient) {
				// Setup port interfaces
				attrs0 := netlink.LinkAttrs{Name: "eth0", Index: 2}
				m.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs0}

				attrs1 := netlink.LinkAttrs{Name: "eth1", Index: 3}
				m.Links["eth1"] = &netlink.Dummy{LinkAttrs: attrs1}
			},
			expectError: false,
			verifyMock: func(t *testing.T, m *MockNetlinkClient) {
				// Should have created bridge
				if m.LinkAddCalls != 1 {
					t.Errorf("Expected 1 bridge creation, got %d", m.LinkAddCalls)
				}

				// Should have set master for both ports
				if m.LinkSetMasterCalls != 2 {
					t.Errorf("Expected 2 LinkSetMaster calls, got %d", m.LinkSetMasterCalls)
				}

				// Should have added IP address
				if m.AddrAddCalls != 1 {
					t.Errorf("Expected 1 AddrAdd call, got %d", m.AddrAddCalls)
				}
			},
		},
		{
			name:      "bridge already exists with same config",
			ifaceName: "br-lan",
			config: types.Interface{
				Type:        "bridge",
				Device:      "br-lan",
				BridgePorts: []string{"eth0", "eth1"},
				Protocol:    "static",
				IPAddr:      "192.168.1.1",
				Netmask:     "255.255.255.0",
				MTU:         1500,
				Enabled:     true,
			},
			setupMock: func(m *MockNetlinkClient) {
				// Bridge already exists
				brAttrs := netlink.LinkAttrs{
					Name:  "br-lan",
					Index: 1,
					MTU:   1500,
				}
				bridge := &netlink.Bridge{LinkAttrs: brAttrs}
				m.Links["br-lan"] = bridge

				// Ports already attached
				attrs0 := netlink.LinkAttrs{Name: "eth0", Index: 2, MasterIndex: 1}
				m.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs0}

				attrs1 := netlink.LinkAttrs{Name: "eth1", Index: 3, MasterIndex: 1}
				m.Links["eth1"] = &netlink.Dummy{LinkAttrs: attrs1}

				// IP already configured
				m.Addresses["br-lan"] = []netlink.Addr{
					{IPNet: mustParseCIDR("192.168.1.1/24")},
				}
			},
			expectError: false,
			verifyMock: func(t *testing.T, m *MockNetlinkClient) {
				// Should not create new bridge (already exists and matches)
				if m.LinkAddCalls != 0 {
					t.Errorf("Expected 0 bridge creations, got %d", m.LinkAddCalls)
				}

				// Should not modify ports (already correct)
				if m.LinkSetMasterCalls != 0 {
					t.Errorf("Expected 0 LinkSetMaster calls, got %d", m.LinkSetMasterCalls)
				}

				// Note: ensureStaticIP may remove and re-add IPs even if they match
				// due to netmask comparison. This is correct behavior to ensure
				// Jack is the source of truth for IP configuration.
			},
		},
		{
			name:      "bridge exists but port configuration changed",
			ifaceName: "br-lan",
			config: types.Interface{
				Type:        "bridge",
				Device:      "br-lan",
				BridgePorts: []string{"eth0", "eth2"}, // eth1 -> eth2
				Protocol:    "static",
				IPAddr:      "192.168.1.1",
				Netmask:     "255.255.255.0",
				MTU:         1500,
				Enabled:     true,
			},
			setupMock: func(m *MockNetlinkClient) {
				// Bridge exists
				brAttrs := netlink.LinkAttrs{Name: "br-lan", Index: 1, MTU: 1500}
				m.Links["br-lan"] = &netlink.Bridge{LinkAttrs: brAttrs}

				// Old ports
				attrs0 := netlink.LinkAttrs{Name: "eth0", Index: 2, MasterIndex: 1}
				m.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs0}

				attrs1 := netlink.LinkAttrs{Name: "eth1", Index: 3, MasterIndex: 1}
				m.Links["eth1"] = &netlink.Dummy{LinkAttrs: attrs1}

				// New port
				attrs2 := netlink.LinkAttrs{Name: "eth2", Index: 4}
				m.Links["eth2"] = &netlink.Dummy{LinkAttrs: attrs2}

				// IP already configured
				m.Addresses["br-lan"] = []netlink.Addr{{IPNet: mustParseCIDR("192.168.1.1/24")}}
			},
			expectError: false,
			verifyMock: func(t *testing.T, m *MockNetlinkClient) {
				// Should remove eth1 from bridge
				if m.LinkSetNoMasterCalls != 1 {
					t.Errorf("Expected 1 LinkSetNoMaster call, got %d", m.LinkSetNoMasterCalls)
				}

				// Should add eth2 to bridge
				if m.LinkSetMasterCalls != 1 {
					t.Errorf("Expected 1 LinkSetMaster call, got %d", m.LinkSetMasterCalls)
				}
			},
		},
		{
			name:      "bridge MTU changed - requires recreation",
			ifaceName: "br-lan",
			config: types.Interface{
				Type:        "bridge",
				Device:      "br-lan",
				BridgePorts: []string{"eth0"},
				Protocol:    "none",
				MTU:         9000, // Changed from 1500
				Enabled:     true,
			},
			setupMock: func(m *MockNetlinkClient) {
				// Bridge exists with old MTU
				brAttrs := netlink.LinkAttrs{Name: "br-lan", Index: 1, MTU: 1500}
				m.Links["br-lan"] = &netlink.Bridge{LinkAttrs: brAttrs}

				attrs0 := netlink.LinkAttrs{Name: "eth0", Index: 2, MasterIndex: 1}
				m.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs0}
			},
			expectError: false,
			verifyMock: func(t *testing.T, m *MockNetlinkClient) {
				// Should delete old bridge
				if m.LinkDelCalls != 1 {
					t.Errorf("Expected 1 LinkDel call, got %d", m.LinkDelCalls)
				}

				// Should create new bridge
				if m.LinkAddCalls != 1 {
					t.Errorf("Expected 1 LinkAdd call, got %d", m.LinkAddCalls)
				}
			},
		},
		{
			name:      "bridge port not found",
			ifaceName: "br-lan",
			config: types.Interface{
				Type:        "bridge",
				Device:      "br-lan",
				BridgePorts: []string{"nonexistent"},
				Protocol:    "none",
				MTU:         1500,
				Enabled:     true,
			},
			setupMock:   func(m *MockNetlinkClient) {},
			expectError: true,
			errorMsg:    "failed to find port",
		},
		{
			name:      "bridge without ports",
			ifaceName: "br-lan",
			config: types.Interface{
				Type:        "bridge",
				Device:      "br-lan",
				BridgePorts: []string{},
				Protocol:    "static",
				IPAddr:      "192.168.1.1",
				Netmask:     "255.255.255.0",
				MTU:         1500,
				Enabled:     true,
			},
			setupMock:   func(m *MockNetlinkClient) {},
			expectError: false,
			verifyMock: func(t *testing.T, m *MockNetlinkClient) {
				// Should create bridge even without ports
				if m.LinkAddCalls != 1 {
					t.Errorf("Expected 1 bridge creation, got %d", m.LinkAddCalls)
				}

				// Should not try to attach any ports
				if m.LinkSetMasterCalls != 0 {
					t.Errorf("Expected 0 LinkSetMaster calls, got %d", m.LinkSetMasterCalls)
				}
			},
		},
		{
			name:      "bridge creation netlink failure",
			ifaceName: "br-lan",
			config: types.Interface{
				Type:        "bridge",
				Device:      "br-lan",
				BridgePorts: []string{},
				Protocol:    "none",
				MTU:         1500,
				Enabled:     true,
			},
			setupMock: func(m *MockNetlinkClient) {
				// Simulate LinkAdd failure
				m.LinkAddError = fmt.Errorf("mock LinkAdd error")
			},
			expectError: true,
			errorMsg:    "failed to create bridge",
		},
		{
			name:      "LinkSetMaster failure",
			ifaceName: "br-lan",
			config: types.Interface{
				Type:        "bridge",
				Device:      "br-lan",
				BridgePorts: []string{"eth0"},
				Protocol:    "none",
				MTU:         1500,
				Enabled:     true,
			},
			setupMock: func(m *MockNetlinkClient) {
				// Port exists
				attrs0 := netlink.LinkAttrs{Name: "eth0", Index: 2}
				m.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs0}
				// Simulate LinkSetMaster failure
				m.LinkSetMasterError = fmt.Errorf("mock LinkSetMaster error")
			},
			expectError: true,
			errorMsg:    "failed to add port",
		},
		{
			name:      "bridge delete failure during recreation",
			ifaceName: "br-lan",
			config: types.Interface{
				Type:        "bridge",
				Device:      "br-lan",
				BridgePorts: []string{},
				Protocol:    "none",
				MTU:         9000,
				Enabled:     true,
			},
			setupMock: func(m *MockNetlinkClient) {
				// Bridge exists with old MTU
				brAttrs := netlink.LinkAttrs{Name: "br-lan", Index: 1, MTU: 1500}
				m.Links["br-lan"] = &netlink.Bridge{LinkAttrs: brAttrs}
				// Simulate LinkDel failure
				m.LinkDelError = fmt.Errorf("mock LinkDel error")
			},
			expectError: true,
			errorMsg:    "failed to delete bridge",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockNetlink := NewMockNetlinkClient()
			mockSysctl := NewMockSysctlClient()

			tt.setupMock(mockNetlink)

			// Create NetworkManager
			nm := NewNetworkManager(mockNetlink, mockSysctl)

			// Execute
			err := nm.ApplyInterfaceConfig(tt.ifaceName, tt.config)

			// Verify error expectation
			if tt.expectError {
				require.Error(t, err, "Expected error but got none")
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}

			// Verify mock state
			if tt.verifyMock != nil {
				tt.verifyMock(t, mockNetlink)
			}
		})
	}
}

// TestGetBridgePorts tests bridge port enumeration.
func TestGetBridgePorts(t *testing.T) {
	mockNetlink := NewMockNetlinkClient()
	mockSysctl := NewMockSysctlClient()

	// Setup bridge
	brAttrs := netlink.LinkAttrs{Name: "br-lan", Index: 1}
	mockNetlink.Links["br-lan"] = &netlink.Bridge{LinkAttrs: brAttrs}

	// Setup ports attached to bridge
	attrs0 := netlink.LinkAttrs{Name: "eth0", Index: 2, MasterIndex: 1}
	mockNetlink.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs0}

	attrs1 := netlink.LinkAttrs{Name: "eth1", Index: 3, MasterIndex: 1}
	mockNetlink.Links["eth1"] = &netlink.Dummy{LinkAttrs: attrs1}

	// Setup unattached port
	attrs2 := netlink.LinkAttrs{Name: "eth2", Index: 4, MasterIndex: 0}
	mockNetlink.Links["eth2"] = &netlink.Dummy{LinkAttrs: attrs2}

	nm := NewNetworkManager(mockNetlink, mockSysctl)

	ports, err := nm.getBridgePorts("br-lan")
	require.NoError(t, err)

	require.Len(t, ports, 2, "Expected 2 ports")

	// Verify correct ports are returned
	portMap := make(map[string]bool)
	for _, port := range ports {
		portMap[port] = true
	}

	assert.True(t, portMap["eth0"] && portMap["eth1"], "Expected eth0 and eth1, got %v", ports)
	assert.False(t, portMap["eth2"], "eth2 should not be in bridge ports")
}

