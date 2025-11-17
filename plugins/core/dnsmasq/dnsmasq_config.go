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

// Package main implements the dnsmasq plugin for Jack.
package main

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/we-are-mono/jack/types"
)

// ValidateDHCPConfig validates a DHCP configuration.
func ValidateDHCPConfig(config *DHCPConfig) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	// If DHCP is enabled, we need at least one pool
	if config.Server.Enabled && len(config.DHCPPools) == 0 {
		return fmt.Errorf("DHCP enabled but no pools defined")
	}

	// Validate each pool
	for poolName, pool := range config.DHCPPools {
		if pool.Interface == "" {
			return fmt.Errorf("pool %s: interface is required", poolName)
		}

		if pool.Start < 0 || pool.Start > 255 {
			return fmt.Errorf("pool %s: invalid start address %d (must be 0-255)", poolName, pool.Start)
		}

		if pool.Limit < 1 || pool.Limit > 255 {
			return fmt.Errorf("pool %s: invalid limit %d (must be 1-255)", poolName, pool.Limit)
		}

		if pool.Start+pool.Limit > 255 {
			return fmt.Errorf("pool %s: start (%d) + limit (%d) exceeds 255", poolName, pool.Start, pool.Limit)
		}

		// Validate lease time format (e.g., "12h", "24h", "7d")
		if pool.LeaseTime != "" && !isValidLeaseTime(pool.LeaseTime) {
			return fmt.Errorf("pool %s: invalid lease time format %s (expected format: 1h, 12h, 7d)", poolName, pool.LeaseTime)
		}

		// Validate DNS servers are valid IPs
		for i, dns := range pool.DNS {
			if net.ParseIP(dns) == nil {
				return fmt.Errorf("pool %s: invalid DNS server IP at index %d: %s", poolName, i, dns)
			}
		}
	}

	// Validate static leases
	for i, lease := range config.StaticLeases {
		if lease.MAC == "" {
			return fmt.Errorf("static lease %d: MAC address is required", i)
		}

		if !isValidMAC(lease.MAC) {
			return fmt.Errorf("static lease %d: invalid MAC address format: %s", i, lease.MAC)
		}

		if lease.IP == "" {
			return fmt.Errorf("static lease %d: IP address is required", i)
		}

		if net.ParseIP(lease.IP) == nil {
			return fmt.Errorf("static lease %d: invalid IP address: %s", i, lease.IP)
		}
	}

	return nil
}

// ValidateDNSConfig validates a DNS configuration.
func ValidateDNSConfig(config *DNSConfig) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	// Validate port range if specified
	if config.Server.Port != 0 && (config.Server.Port < 1 || config.Server.Port > 65535) {
		return fmt.Errorf("invalid DNS port %d (must be 1-65535)", config.Server.Port)
	}

	// Validate upstream DNS servers
	for i, upstream := range config.Server.Upstreams {
		if net.ParseIP(upstream) == nil {
			return fmt.Errorf("invalid upstream DNS server at index %d: %s", i, upstream)
		}
	}

	// Validate A records
	for i, record := range config.Records.ARecords {
		if record.Name == "" {
			return fmt.Errorf("A record %d: name is required", i)
		}
		if net.ParseIP(record.IP) == nil {
			return fmt.Errorf("A record %d (%s): invalid IPv4 address: %s", i, record.Name, record.IP)
		}
	}

	// Validate AAAA records
	for i, record := range config.Records.AAAARecords {
		if record.Name == "" {
			return fmt.Errorf("AAAA record %d: name is required", i)
		}
		ip := net.ParseIP(record.IP)
		if ip == nil || ip.To4() != nil {
			return fmt.Errorf("AAAA record %d (%s): invalid IPv6 address: %s", i, record.Name, record.IP)
		}
	}

	// Validate CNAME records
	for i, record := range config.Records.CNAMERecords {
		if record.CNAME == "" {
			return fmt.Errorf("CNAME record %d: cname is required", i)
		}
		if record.Target == "" {
			return fmt.Errorf("CNAME record %d (%s): target is required", i, record.CNAME)
		}
	}

	// Validate MX records
	for i, record := range config.Records.MXRecords {
		if record.Domain == "" {
			return fmt.Errorf("MX record %d: domain is required", i)
		}
		if record.Server == "" {
			return fmt.Errorf("MX record %d (%s): server is required", i, record.Domain)
		}
		if record.Priority < 0 {
			return fmt.Errorf("MX record %d (%s): priority cannot be negative", i, record.Domain)
		}
	}

	// Validate SRV records
	for i, record := range config.Records.SRVRecords {
		if record.Service == "" {
			return fmt.Errorf("SRV record %d: service is required", i)
		}
		if record.Proto == "" {
			return fmt.Errorf("SRV record %d (%s): proto is required", i, record.Service)
		}
		if record.Target == "" {
			return fmt.Errorf("SRV record %d (%s): target is required", i, record.Service)
		}
		if record.Port < 1 || record.Port > 65535 {
			return fmt.Errorf("SRV record %d (%s): invalid port %d", i, record.Service, record.Port)
		}
	}

	// Validate PTR records
	for i, record := range config.Records.PTRRecords {
		if record.IP == "" {
			return fmt.Errorf("PTR record %d: IP is required", i)
		}
		if net.ParseIP(record.IP) == nil {
			return fmt.Errorf("PTR record %d: invalid IP address: %s", i, record.IP)
		}
		if record.Name == "" {
			return fmt.Errorf("PTR record %d (%s): name is required", i, record.IP)
		}
	}

	return nil
}

// NetworkPortion extracts the network portion from an IP address (everything before the last dot).
func NetworkPortion(ip string) string {
	lastDot := strings.LastIndex(ip, ".")
	if lastDot == -1 {
		return ip
	}
	return ip[:lastDot]
}

// isValidLeaseTime checks if a lease time string is valid (e.g., "12h", "7d", "30m").
func isValidLeaseTime(leaseTime string) bool {
	// Match patterns like: 1h, 12h, 7d, 30m, 1w
	matched, _ := regexp.MatchString(`^\d+[mhdw]$`, leaseTime)
	return matched
}

// isValidMAC checks if a MAC address is valid.
func isValidMAC(mac string) bool {
	// Match MAC address formats: aa:bb:cc:dd:ee:ff or aa-bb-cc-dd-ee-ff (but not mixed)
	colonMatch, _ := regexp.MatchString(`^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$`, mac)
	hyphenMatch, _ := regexp.MatchString(`^([0-9A-Fa-f]{2}-){5}([0-9A-Fa-f]{2})$`, mac)
	return colonMatch || hyphenMatch
}

// GetInterfaceIPsFromConfig extracts IP addresses from an interfaces configuration.
func GetInterfaceIPsFromConfig(interfacesConfig types.InterfacesConfig) map[string]string {
	ips := make(map[string]string)

	for name, iface := range interfacesConfig.Interfaces {
		if iface.IPAddr != "" {
			ips[name] = iface.IPAddr
		}
	}

	return ips
}

// GetInterfaceDevicesFromConfig extracts device names from an interfaces configuration.
func GetInterfaceDevicesFromConfig(interfacesConfig types.InterfacesConfig) map[string]string {
	devices := make(map[string]string)

	for name, iface := range interfacesConfig.Interfaces {
		if iface.Type == "bridge" && iface.DeviceName != "" {
			devices[name] = iface.DeviceName
		} else if iface.Type == "physical" && iface.Device != "" {
			devices[name] = iface.Device
		}
	}

	return devices
}
