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

// TestEnableIPForwarding demonstrates unit testing with mocks.
// This test runs without any actual system calls!
func TestEnableIPForwarding(t *testing.T) {
	// Create mock clients
	mockNetlink := NewMockNetlinkClient()
	mockSysctl := NewMockSysctlClient()

	// Create NetworkManager with mocked dependencies
	nm := NewNetworkManager(mockNetlink, mockSysctl)

	// Call the method
	err := nm.EnableIPForwarding()

	// Verify results
	require.NoError(t, err)

	// Verify sysctl was called with correct parameters
	assert.Equal(t, 1, mockSysctl.SetCalls, "Expected sysctl.Set to be called once")

	// Verify the correct value was set
	val, ok := mockSysctl.Values["net.ipv4.ip_forward"]
	assert.True(t, ok, "Expected net.ipv4.ip_forward to be set")
	assert.Equal(t, "1", val, "Expected net.ipv4.ip_forward=1")
}

// TestApplyPhysicalInterface demonstrates testing complex interface configuration.
func TestApplyPhysicalInterface(t *testing.T) {
	tests := []struct {
		name        string
		iface       types.Interface
		setupMock   func(*MockNetlinkClient)
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful static IP configuration",
			iface: types.Interface{
				Type:     "physical",
				Device:   "eth0",
				Protocol: "static",
				IPAddr:   "192.168.1.10",
				Netmask:  "255.255.255.0",
				MTU:      1500,
				Enabled:  true,
			},
			setupMock: func(m *MockNetlinkClient) {
				// Setup mock link
				attrs := netlink.LinkAttrs{
					Name:  "eth0",
					Index: 1,
					MTU:   1500,
				}
				m.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs}
			},
			expectError: false,
		},
		{
			name: "interface not found",
			iface: types.Interface{
				Type:     "physical",
				Device:   "nonexistent",
				Protocol: "static",
				IPAddr:   "192.168.1.10",
				Netmask:  "255.255.255.0",
				Enabled:  true,
			},
			setupMock:   func(m *MockNetlinkClient) {},
			expectError: true,
			errorMsg:    "failed to find interface",
		},
		{
			name: "disabled interface",
			iface: types.Interface{
				Type:    "physical",
				Device:  "eth0",
				Enabled: false,
			},
			setupMock: func(m *MockNetlinkClient) {
				attrs := netlink.LinkAttrs{Name: "eth0", Index: 1}
				m.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs}
			},
			expectError: false,
		},
		{
			name: "invalid IP address format",
			iface: types.Interface{
				Type:     "physical",
				Device:   "eth0",
				Protocol: "static",
				IPAddr:   "invalid-ip",
				Netmask:  "255.255.255.0",
				Enabled:  true,
			},
			setupMock: func(m *MockNetlinkClient) {
				attrs := netlink.LinkAttrs{Name: "eth0", Index: 1}
				m.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs}
			},
			expectError: true,
			errorMsg:    "invalid",
		},
		{
			name: "netlink AddrAdd failure",
			iface: types.Interface{
				Type:     "physical",
				Device:   "eth0",
				Protocol: "static",
				IPAddr:   "192.168.1.10",
				Netmask:  "255.255.255.0",
				Enabled:  true,
			},
			setupMock: func(m *MockNetlinkClient) {
				attrs := netlink.LinkAttrs{Name: "eth0", Index: 1}
				m.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs}
				// Simulate AddrAdd failure
				m.AddrAddError = fmt.Errorf("mock AddrAdd error")
			},
			expectError: true,
			errorMsg:    "failed to add IP address",
		},
		{
			name: "netlink LinkSetUp failure",
			iface: types.Interface{
				Type:     "physical",
				Device:   "eth0",
				Protocol: "static",
				IPAddr:   "192.168.1.10",
				Netmask:  "255.255.255.0",
				Enabled:  true,
			},
			setupMock: func(m *MockNetlinkClient) {
				attrs := netlink.LinkAttrs{Name: "eth0", Index: 1}
				m.Links["eth0"] = &netlink.Dummy{LinkAttrs: attrs}
				// Simulate LinkSetUp failure
				m.LinkSetUpError = fmt.Errorf("mock LinkSetUp error")
			},
			expectError: true,
			errorMsg:    "failed to bring up interface",
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
			err := nm.ApplyInterfaceConfig("test-iface", tt.iface)

			// Verify
			if tt.expectError {
				require.Error(t, err, "Expected error but got none")
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

