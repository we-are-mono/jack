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

//go:build integration
// +build integration

package integration

import (
	"testing"

	"github.com/we-are-mono/jack/types"
)

// TestStructureValidation validates that test fixtures and type structures are correct
// This test runs without root privileges to validate configuration structures
func TestStructureValidation(t *testing.T) {
	t.Run("InterfaceConfigStructure", func(t *testing.T) {
		// Validate that interface config structures are properly defined
		config := &types.InterfacesConfig{
			Interfaces: map[string]types.Interface{
				"eth0": {
					Type:     "physical",
					Device:   "eth0",
					Enabled:  true,
					Protocol: "static",
					IPAddr:   "192.168.1.1",
					Netmask:  "255.255.255.0",
					MTU:      1500,
				},
				"br-lan": {
					Type:        "bridge",
					Device:      "br-lan",
					BridgePorts: []string{"eth0", "eth1"},
					Enabled:     true,
					Protocol:    "static",
					IPAddr:      "192.168.1.1",
					Netmask:     "255.255.255.0",
				},
				"eth0.100": {
					Type:       "vlan",
					Device:     "eth0",
					DeviceName: "eth0.100",
					VLANId:     100,
					Enabled:    true,
					Protocol:   "static",
					IPAddr:     "192.168.100.1",
					Netmask:    "255.255.255.0",
				},
			},
		}

		if len(config.Interfaces) != 3 {
			t.Errorf("Expected 3 interfaces, got %d", len(config.Interfaces))
		}

		// Validate physical interface
		if iface, ok := config.Interfaces["eth0"]; ok {
			if iface.Type != "physical" {
				t.Errorf("Expected physical type, got %s", iface.Type)
			}
			if iface.MTU != 1500 {
				t.Errorf("Expected MTU 1500, got %d", iface.MTU)
			}
		} else {
			t.Error("eth0 interface not found")
		}

		// Validate bridge interface
		if iface, ok := config.Interfaces["br-lan"]; ok {
			if iface.Type != "bridge" {
				t.Errorf("Expected bridge type, got %s", iface.Type)
			}
			if len(iface.BridgePorts) != 2 {
				t.Errorf("Expected 2 bridge ports, got %d", len(iface.BridgePorts))
			}
		} else {
			t.Error("br-lan interface not found")
		}

		// Validate VLAN interface
		if iface, ok := config.Interfaces["eth0.100"]; ok {
			if iface.Type != "vlan" {
				t.Errorf("Expected vlan type, got %s", iface.Type)
			}
			if iface.VLANId != 100 {
				t.Errorf("Expected VLAN ID 100, got %d", iface.VLANId)
			}
			if iface.Device != "eth0" {
				t.Errorf("Expected parent device eth0, got %s", iface.Device)
			}
			if iface.DeviceName != "eth0.100" {
				t.Errorf("Expected device name eth0.100, got %s", iface.DeviceName)
			}
		} else {
			t.Error("eth0.100 interface not found")
		}
	})

	t.Run("RouteConfigStructure", func(t *testing.T) {
		// Validate route config structures
		config := &types.RoutesConfig{
			Routes: map[string]types.Route{
				"default": {
					Destination: "0.0.0.0/0",
					Gateway:     "192.168.1.1",
					Interface:   "eth0",
					Metric:      100,
					Enabled:     true,
				},
				"static": {
					Destination: "10.0.0.0/8",
					Gateway:     "192.168.1.254",
					Interface:   "eth0",
					Metric:      200,
					Enabled:     true,
				},
			},
		}

		if len(config.Routes) != 2 {
			t.Errorf("Expected 2 routes, got %d", len(config.Routes))
		}

		// Validate default route
		if route, ok := config.Routes["default"]; ok {
			if route.Destination != "0.0.0.0/0" {
				t.Errorf("Expected default destination, got %s", route.Destination)
			}
			if route.Metric != 100 {
				t.Errorf("Expected metric 100, got %d", route.Metric)
			}
		} else {
			t.Error("default route not found")
		}

		// Validate static route
		if route, ok := config.Routes["static"]; ok {
			if route.Destination != "10.0.0.0/8" {
				t.Errorf("Expected 10.0.0.0/8 destination, got %s", route.Destination)
			}
			if route.Gateway != "192.168.1.254" {
				t.Errorf("Expected gateway 192.168.1.254, got %s", route.Gateway)
			}
		} else {
			t.Error("static route not found")
		}
	})
}
