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

// Package system provides low-level system integration for network, firewall, DHCP, and routing.
package system

import (
	"github.com/we-are-mono/jack/types"
)

// Default network manager for backwards compatibility.
// This allows existing code to work without changes while enabling
// dependency injection for testing through NetworkManager.
var defaultNetworkManager = NewDefaultNetworkManager()

// EnableIPForwarding enables kernel IP forwarding.
// For testing, use NetworkManager.EnableIPForwarding with injected dependencies.
func EnableIPForwarding() error {
	return defaultNetworkManager.EnableIPForwarding()
}

// ApplyInterfaceConfig applies configuration to a network interface.
// This function uses the default NetworkManager for backwards compatibility.
// For testing, create a NetworkManager with injected dependencies.
func ApplyInterfaceConfig(name string, iface types.Interface) error {
	return defaultNetworkManager.ApplyInterfaceConfig(name, iface)
}
