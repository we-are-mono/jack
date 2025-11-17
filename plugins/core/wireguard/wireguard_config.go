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

// Package main implements the WireGuard VPN plugin for Jack.
package main

import (
	"fmt"
	"net"
	"strings"
)

// NetmaskToCIDR converts a netmask to CIDR prefix length.
func NetmaskToCIDR(netmask string) string {
	// Common netmask to CIDR conversions
	masks := map[string]string{
		"255.255.255.255": "32",
		"255.255.255.254": "31",
		"255.255.255.252": "30",
		"255.255.255.248": "29",
		"255.255.255.240": "28",
		"255.255.255.224": "27",
		"255.255.255.192": "26",
		"255.255.255.128": "25",
		"255.255.255.0":   "24",
		"255.255.254.0":   "23",
		"255.255.252.0":   "22",
		"255.255.248.0":   "21",
		"255.255.240.0":   "20",
		"255.255.224.0":   "19",
		"255.255.192.0":   "18",
		"255.255.128.0":   "17",
		"255.255.0.0":     "16",
		"255.254.0.0":     "15",
		"255.252.0.0":     "14",
		"255.248.0.0":     "13",
		"255.240.0.0":     "12",
		"255.224.0.0":     "11",
		"255.192.0.0":     "10",
		"255.128.0.0":     "9",
		"255.0.0.0":       "8",
	}

	if cidr, ok := masks[netmask]; ok {
		return cidr
	}

	return "24" // Default fallback
}

// StringSlicesEqual checks if two string slices contain the same elements (order-independent).
func StringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	aMap := make(map[string]bool)
	for _, item := range a {
		aMap[item] = true
	}

	for _, item := range b {
		if !aMap[item] {
			return false
		}
	}

	return true
}

// BuildPeerArgs generates the wg command arguments for configuring a peer.
// Returns args and whether preshared key needs to be passed via stdin.
func BuildPeerArgs(deviceName string, peer WireGuardPeer) ([]string, bool) {
	args := []string{"set", deviceName, "peer", peer.PublicKey}

	needsStdin := false
	if peer.PresharedKey != "" {
		args = append(args, "preshared-key", "/dev/stdin")
		needsStdin = true
	}

	if peer.Endpoint != "" {
		args = append(args, "endpoint", peer.Endpoint)
	}

	if len(peer.AllowedIPs) > 0 {
		args = append(args, "allowed-ips", strings.Join(peer.AllowedIPs, ","))
	}

	if peer.PersistentKeepalive > 0 {
		args = append(args, "persistent-keepalive", fmt.Sprintf("%d", peer.PersistentKeepalive))
	}

	return args, needsStdin
}

// ValidateVPNConfig validates a WireGuard VPN configuration.
func ValidateVPNConfig(config *VPNConfig) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	if len(config.Interfaces) == 0 {
		return fmt.Errorf("no interfaces defined")
	}

	for name, iface := range config.Interfaces {
		if iface.DeviceName == "" {
			return fmt.Errorf("interface %s: device_name is required", name)
		}

		if iface.PrivateKey == "" {
			return fmt.Errorf("interface %s: private_key is required", name)
		}

		if iface.Address == "" {
			return fmt.Errorf("interface %s: address is required", name)
		}

		if iface.Netmask == "" {
			return fmt.Errorf("interface %s: netmask is required", name)
		}

		// Validate listen port range
		if iface.ListenPort < 0 || iface.ListenPort > 65535 {
			return fmt.Errorf("interface %s: invalid listen port %d (must be 0-65535)", name, iface.ListenPort)
		}

		// Validate MTU range
		if iface.MTU < 0 || iface.MTU > 9000 {
			return fmt.Errorf("interface %s: invalid MTU %d (must be 0-9000)", name, iface.MTU)
		}

		// Validate IP address format
		if !strings.Contains(iface.Address, "/") {
			// If no CIDR, check it's a valid IP
			if net.ParseIP(iface.Address) == nil {
				return fmt.Errorf("interface %s: invalid IP address: %s", name, iface.Address)
			}
		} else {
			// Validate CIDR
			if _, _, err := net.ParseCIDR(iface.Address); err != nil {
				return fmt.Errorf("interface %s: invalid CIDR address %s: %w", name, iface.Address, err)
			}
		}

		// Validate peers
		for i, peer := range iface.Peers {
			if peer.PublicKey == "" {
				return fmt.Errorf("interface %s, peer %d: public_key is required", name, i)
			}

			// Validate public key format (base64, typically 44 characters for WireGuard)
			if len(peer.PublicKey) < 40 || strings.Contains(peer.PublicKey, " ") {
				return fmt.Errorf("interface %s, peer %d: invalid public_key format (expected base64 key, typically 44 characters)", name, i)
			}

			if len(peer.AllowedIPs) == 0 {
				return fmt.Errorf("interface %s, peer %d: allowed_ips is required", name, i)
			}

			// Validate each allowed IP is valid CIDR
			for j, allowedIP := range peer.AllowedIPs {
				if _, _, err := net.ParseCIDR(allowedIP); err != nil {
					return fmt.Errorf("interface %s, peer %d, allowed_ip %d: invalid CIDR %s: %w", name, i, j, allowedIP, err)
				}
			}

			// Validate endpoint format if provided (host:port)
			if peer.Endpoint != "" {
				parts := strings.Split(peer.Endpoint, ":")
				if len(parts) != 2 {
					return fmt.Errorf("interface %s, peer %d: invalid endpoint format (expected host:port)", name, i)
				}
			}

			// Validate persistent keepalive range
			if peer.PersistentKeepalive < 0 || peer.PersistentKeepalive > 65535 {
				return fmt.Errorf("interface %s, peer %d: invalid persistent_keepalive %d (must be 0-65535)", name, i, peer.PersistentKeepalive)
			}
		}
	}

	return nil
}

// FormatCIDR formats an IP address with netmask into CIDR notation.
func FormatCIDR(ipAddr, netmask string) string {
	if strings.Contains(ipAddr, "/") {
		return ipAddr
	}
	return ipAddr + "/" + NetmaskToCIDR(netmask)
}
