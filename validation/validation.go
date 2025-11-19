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

// Package validation provides reusable validation helpers for Jack configuration types.
package validation

import (
	"encoding/base64"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// ValidatePort validates that a port number is in the valid range [1, 65535].
func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port %d out of valid range [1, 65535]", port)
	}
	return nil
}

// ValidatePortString validates a port number or port range string.
// Valid formats: "80", "8080-8090"
func ValidatePortString(portStr string) error {
	if portStr == "" {
		return fmt.Errorf("port string cannot be empty")
	}

	// Check if it's a port range (contains hyphen)
	if strings.Contains(portStr, "-") {
		parts := strings.Split(portStr, "-")
		if len(parts) != 2 {
			return fmt.Errorf("invalid port range format: %s (expected format: 'start-end')", portStr)
		}

		start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return fmt.Errorf("invalid start port in range %s: %w", portStr, err)
		}

		end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return fmt.Errorf("invalid end port in range %s: %w", portStr, err)
		}

		if err := ValidatePort(start); err != nil {
			return fmt.Errorf("invalid start port in range %s: %w", portStr, err)
		}

		if err := ValidatePort(end); err != nil {
			return fmt.Errorf("invalid end port in range %s: %w", portStr, err)
		}

		if start >= end {
			return fmt.Errorf("invalid port range %s: start port must be less than end port", portStr)
		}

		return nil
	}

	// Single port number
	port, err := strconv.Atoi(strings.TrimSpace(portStr))
	if err != nil {
		return fmt.Errorf("invalid port number %s: %w", portStr, err)
	}

	return ValidatePort(port)
}

// ValidateIP validates that a string is a valid IPv4 or IPv6 address.
func ValidateIP(ip string) error {
	if ip == "" {
		return fmt.Errorf("IP address cannot be empty")
	}

	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}

	return nil
}

// ValidateCIDR validates that a string is valid CIDR notation.
func ValidateCIDR(cidr string) error {
	if cidr == "" {
		return fmt.Errorf("CIDR cannot be empty")
	}

	// Special case: "default" is allowed for default routes
	if cidr == "default" {
		return nil
	}

	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR notation %s: %w", cidr, err)
	}

	return nil
}

// ValidatePolicy validates that a policy string is ACCEPT, DROP, or REJECT (case-insensitive).
func ValidatePolicy(policy string) error {
	if policy == "" {
		return fmt.Errorf("policy cannot be empty")
	}

	normalized := strings.ToUpper(policy)
	if normalized != "ACCEPT" && normalized != "DROP" && normalized != "REJECT" {
		return fmt.Errorf("invalid policy %s (must be ACCEPT, DROP, or REJECT)", policy)
	}

	return nil
}

// ValidateProtocol validates that a protocol string is in the allowed list.
func ValidateProtocol(proto string, allowed []string) error {
	if proto == "" {
		return nil // Empty protocol is often optional
	}

	for _, validProto := range allowed {
		if proto == validProto {
			return nil
		}
	}

	return fmt.Errorf("invalid protocol %s (must be one of: %s)", proto, strings.Join(allowed, ", "))
}

// ValidateDomain validates a DNS domain name.
// Allows standard domain names and wildcards (e.g., "*.example.com").
func ValidateDomain(domain string) error {
	if domain == "" {
		return nil // Empty domain is often optional
	}

	// Basic domain name regex - allows letters, numbers, hyphens, dots, and wildcards
	// RFC 1035 compliant with wildcard support
	domainRegex := regexp.MustCompile(`^(\*\.)?([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

	if !domainRegex.MatchString(domain) {
		return fmt.Errorf("invalid domain name: %s", domain)
	}

	// Check total length (RFC 1035: max 253 characters)
	if len(domain) > 253 {
		return fmt.Errorf("domain name too long: %s (max 253 characters)", domain)
	}

	// Check individual label length (max 63 characters)
	labels := strings.Split(strings.TrimPrefix(domain, "*."), ".")
	for _, label := range labels {
		if len(label) > 63 {
			return fmt.Errorf("domain label too long in %s (max 63 characters per label)", domain)
		}
	}

	return nil
}

// ValidateMAC validates a MAC address in common formats.
// Accepts formats: "00:11:22:33:44:55", "00-11-22-33-44-55", "0011.2233.4455"
func ValidateMAC(mac string) error {
	if mac == "" {
		return nil // Empty MAC is often optional
	}

	// Try parsing as hardware address
	_, err := net.ParseMAC(mac)
	if err != nil {
		return fmt.Errorf("invalid MAC address %s: %w", mac, err)
	}

	return nil
}

// ValidateMTU validates that an MTU value is within reasonable bounds.
// RFC 791: Minimum IPv4 MTU is 68 bytes
// Practical maximum is 65535 (jumbo frames go higher but are uncommon)
func ValidateMTU(mtu int) error {
	if mtu < 68 || mtu > 65535 {
		return fmt.Errorf("MTU %d out of valid range [68, 65535]", mtu)
	}
	return nil
}

// ValidateVLANID validates that a VLAN ID is in the valid range [1, 4094].
// VLAN ID 0 is reserved for priority tagging, 4095 is reserved.
func ValidateVLANID(vlanID int) error {
	if vlanID < 1 || vlanID > 4094 {
		return fmt.Errorf("VLAN ID %d out of valid range [1, 4094]", vlanID)
	}
	return nil
}

// ValidateWireGuardKey validates a WireGuard public or private key.
// WireGuard keys are 32-byte values encoded as base64 (44 characters including padding).
func ValidateWireGuardKey(key string) error {
	if key == "" {
		return fmt.Errorf("WireGuard key cannot be empty")
	}

	// Decode base64
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return fmt.Errorf("invalid WireGuard key (not valid base64): %w", err)
	}

	// WireGuard keys must be exactly 32 bytes
	if len(decoded) != 32 {
		return fmt.Errorf("invalid WireGuard key length: expected 32 bytes, got %d", len(decoded))
	}

	return nil
}

// ValidateEndpoint validates a WireGuard endpoint in "host:port" format.
// Host can be an IP address or hostname.
func ValidateEndpoint(endpoint string) error {
	if endpoint == "" {
		return nil // Empty endpoint is often optional
	}

	// Split host and port
	host, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint format %s (expected 'host:port'): %w", endpoint, err)
	}

	// Validate host is either IP or valid hostname
	if net.ParseIP(host) == nil {
		// Not an IP, check if it's a valid hostname
		if err := ValidateDomain(host); err != nil {
			return fmt.Errorf("invalid endpoint host %s: %w", endpoint, err)
		}
	}

	// Validate port
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid endpoint port in %s: %w", endpoint, err)
	}

	if err := ValidatePort(port); err != nil {
		return fmt.Errorf("invalid endpoint port in %s: %w", endpoint, err)
	}

	return nil
}

// ValidateNetmask validates an IPv4 netmask.
// Accepts formats: "255.255.255.0" (dotted decimal) or "/24" (CIDR prefix)
func ValidateNetmask(netmask string) error {
	if netmask == "" {
		return fmt.Errorf("netmask cannot be empty")
	}

	// Check if it's CIDR prefix notation (e.g., "/24")
	if strings.HasPrefix(netmask, "/") {
		prefix := strings.TrimPrefix(netmask, "/")
		prefixLen, err := strconv.Atoi(prefix)
		if err != nil {
			return fmt.Errorf("invalid CIDR prefix %s: %w", netmask, err)
		}
		if prefixLen < 0 || prefixLen > 32 {
			return fmt.Errorf("invalid CIDR prefix %s: must be between 0 and 32", netmask)
		}
		return nil
	}

	// Check if it's dotted decimal notation
	ip := net.ParseIP(netmask)
	if ip == nil {
		return fmt.Errorf("invalid netmask format %s (expected dotted decimal or CIDR prefix)", netmask)
	}

	// Verify it's a valid netmask (contiguous 1 bits followed by 0 bits)
	ipv4 := ip.To4()
	if ipv4 == nil {
		return fmt.Errorf("invalid netmask %s (not an IPv4 address)", netmask)
	}

	// Convert to uint32 and check if it's a valid netmask
	mask := uint32(ipv4[0])<<24 | uint32(ipv4[1])<<16 | uint32(ipv4[2])<<8 | uint32(ipv4[3])

	// A valid netmask has contiguous 1s followed by contiguous 0s
	// Invert it, add 1, and check if it's a power of 2
	inverted := ^mask
	if (inverted+1)&inverted != 0 && mask != 0 {
		return fmt.Errorf("invalid netmask %s (not a valid contiguous netmask)", netmask)
	}

	return nil
}

// ValidateTableID validates a Linux routing table ID.
// Valid range is [0, 4294967295] (uint32 max).
func ValidateTableID(tableID int) error {
	if tableID < 0 || tableID > 4294967295 {
		return fmt.Errorf("table ID %d out of valid range [0, 4294967295]", tableID)
	}
	return nil
}

// ValidateMetric validates a routing metric value.
// Metric must be non-negative.
func ValidateMetric(metric int) error {
	if metric < 0 {
		return fmt.Errorf("metric %d cannot be negative", metric)
	}
	return nil
}
