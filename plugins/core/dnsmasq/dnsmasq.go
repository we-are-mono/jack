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

// Package dnsmasq implements the dnsmasq plugin for Jack.
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"github.com/we-are-mono/jack/state"
	"github.com/we-are-mono/jack/types"
)

const (
	dnsmasqConfigPath   = "/etc/dnsmasq.d/jack.conf"
	dnsmasqTemplatePath = "/etc/jack/templates/dnsmasq.conf.tmpl"
)

// DnsmasqProvider implements both DHCPProvider and DNSProvider interfaces
type DnsmasqProvider struct {
	dhcpConfigPath string
	templatePath   string
}

// NewDnsmasq creates a new dnsmasq provider
func NewDnsmasq() (*DnsmasqProvider, error) {
	// Check if dnsmasq is available
	if _, err := exec.LookPath("dnsmasq"); err != nil {
		return nil, fmt.Errorf("dnsmasq not found: %w", err)
	}

	return &DnsmasqProvider{
		dhcpConfigPath: dnsmasqConfigPath,
		templatePath:   dnsmasqTemplatePath,
	}, nil
}

// ============================================================================
// DHCP Provider Implementation
// ============================================================================

// ApplyConfig implements DHCPProvider.ApplyConfig
func (d *DnsmasqProvider) ApplyDHCPConfig(ctx context.Context, config *DHCPConfig) error {
	fmt.Println("  Applying DHCP configuration...")

	if !config.Server.Enabled {
		fmt.Println("    [INFO] DHCP server disabled")
		return d.stopService()
	}

	// Generate config content (don't write yet)
	newConfig, err := d.generateDHCPConfigString(config)
	if err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}

	// Read existing config if it exists
	existingConfig, err := os.ReadFile(d.dhcpConfigPath)
	if err == nil {
		// Config file exists - compare content
		if string(existingConfig) == newConfig {
			fmt.Println("    [OK] DHCP configuration unchanged, skipping restart")
			return nil
		}
		fmt.Println("    [INFO] DHCP configuration changed, updating...")
	} else {
		fmt.Println("    [INFO] Creating new DHCP configuration...")
	}

	// Write new config
	if err := d.writeDHCPConfig(newConfig); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Test config
	if err := d.testConfig(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Restart dnsmasq
	if err := d.reloadService(); err != nil {
		return fmt.Errorf("failed to reload dnsmasq: %w", err)
	}

	fmt.Println("    [OK] DHCP server configured")
	return nil
}

// ValidateDHCP implements DHCPProvider.Validate
func (d *DnsmasqProvider) ValidateDHCP(ctx context.Context, config *DHCPConfig) error {
	return ValidateDHCPConfig(config)
}

// FlushDHCP implements DHCPProvider.Flush
func (d *DnsmasqProvider) FlushDHCP(ctx context.Context) error {
	// Remove DHCP-specific configuration
	// For now, this is a no-op as we regenerate the full config
	return nil
}

// EnableDHCP implements DHCPProvider.Enable
func (d *DnsmasqProvider) EnableDHCP(ctx context.Context) error {
	return d.startService()
}

// DisableDHCP implements DHCPProvider.Disable
func (d *DnsmasqProvider) DisableDHCP(ctx context.Context) error {
	return d.stopService()
}

// StatusDHCP implements DHCPProvider.Status
func (d *DnsmasqProvider) StatusDHCP(ctx context.Context) (bool, string, int, error) {
	enabled, err := d.isServiceRunning()
	if err != nil {
		return false, "", 0, err
	}

	// Count active leases from lease file
	leaseCount := 0
	leaseFile := "/var/lib/misc/dnsmasq.leases"

	data, err := os.ReadFile(leaseFile)
	if err == nil {
		// Each line in the lease file is one lease
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				leaseCount++
			}
		}
	}

	return enabled, "dnsmasq", leaseCount, nil
}

// ============================================================================
// DNS Provider Implementation
// ============================================================================

// ApplyDNSConfig implements DNSProvider.ApplyConfig
func (d *DnsmasqProvider) ApplyDNSConfig(ctx context.Context, config *DNSConfig) error {
	fmt.Println("  Applying DNS configuration...")

	if !config.Server.Enabled {
		fmt.Println("    [INFO] DNS server disabled")
		return d.stopService()
	}

	// Generate config file
	if err := d.generateDNSConfig(config); err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}

	// Test config
	if err := d.testConfig(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Restart dnsmasq
	if err := d.reloadService(); err != nil {
		return fmt.Errorf("failed to reload dnsmasq: %w", err)
	}

	fmt.Println("    [OK] DNS server configured")
	return nil
}

// ValidateDNS implements DNSProvider.Validate
func (d *DnsmasqProvider) ValidateDNS(ctx context.Context, config *DNSConfig) error {
	return ValidateDNSConfig(config)
}

// FlushDNS implements DNSProvider.Flush
func (d *DnsmasqProvider) FlushDNS(ctx context.Context) error {
	// Clear DNS cache
	// dnsmasq doesn't have a direct cache flush command
	// We'd need to send SIGHUP or restart
	return d.reloadService()
}

// EnableDNS implements DNSProvider.Enable
func (d *DnsmasqProvider) EnableDNS(ctx context.Context) error {
	return d.startService()
}

// DisableDNS implements DNSProvider.Disable
func (d *DnsmasqProvider) DisableDNS(ctx context.Context) error {
	return d.stopService()
}

// StatusDNS implements DNSProvider.Status
func (d *DnsmasqProvider) StatusDNS(ctx context.Context) (bool, string, int, error) {
	enabled, err := d.isServiceRunning()
	if err != nil {
		return false, "", 0, err
	}

	// TODO: Count DNS records
	recordCount := 0

	return enabled, "dnsmasq", recordCount, nil
}

// ============================================================================
// Helper methods
// ============================================================================

func (d *DnsmasqProvider) startService() error {
	cmd := exec.Command("systemctl", "start", "dnsmasq")
	return cmd.Run()
}

func (d *DnsmasqProvider) stopService() error {
	cmd := exec.Command("systemctl", "stop", "dnsmasq")
	return cmd.Run()
}

func (d *DnsmasqProvider) reloadService() error {
	cmd := exec.Command("systemctl", "reload-or-restart", "dnsmasq")
	return cmd.Run()
}

func (d *DnsmasqProvider) isServiceRunning() (bool, error) {
	cmd := exec.Command("systemctl", "is-active", "dnsmasq")
	err := cmd.Run()
	return err == nil, nil
}

// generateDHCPConfigString generates the DHCP config as a string (without writing to disk)
func (d *DnsmasqProvider) generateDHCPConfigString(config *DHCPConfig) (string, error) {
	// Read template
	tmplContent, err := os.ReadFile(d.templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template: %w", err)
	}

	// Parse template with custom functions
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
	}

	tmpl, err := template.New("dnsmasq").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Prepare template data
	data := struct {
		Config           *DHCPConfig
		InterfaceIPs     map[string]string
		InterfaceDevices map[string]string
		NetworkPortion   func(string) string
	}{
		Config:           config,
		InterfaceIPs:     d.getInterfaceIPs(config),
		InterfaceDevices: d.getInterfaceDevices(config),
		NetworkPortion:   NetworkPortion,
	}

	// Execute template to buffer
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// writeDHCPConfig writes the config string to disk
func (d *DnsmasqProvider) writeDHCPConfig(content string) error {
	// Create directory if it doesn't exist
	dir := "/etc/dnsmasq.d"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write config file
	if err := os.WriteFile(d.dhcpConfigPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (d *DnsmasqProvider) generateDNSConfig(config *DNSConfig) error {
	// Create directory if it doesn't exist
	dir := "/etc/dnsmasq.d"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// TODO: Implement DNS config generation
	// For now, this is a placeholder that would read a DNS template
	// and generate dnsmasq DNS configuration

	return fmt.Errorf("DNS configuration not yet implemented")
}

func (d *DnsmasqProvider) getInterfaceIPs(config *DHCPConfig) map[string]string {
	// Load interfaces config
	var interfacesConfig types.InterfacesConfig
	if err := state.LoadConfig("interfaces", &interfacesConfig); err != nil {
		return make(map[string]string)
	}

	return GetInterfaceIPsFromConfig(interfacesConfig)
}

func (d *DnsmasqProvider) getInterfaceDevices(config *DHCPConfig) map[string]string {
	// Load interfaces config
	var interfacesConfig types.InterfacesConfig
	if err := state.LoadConfig("interfaces", &interfacesConfig); err != nil {
		return make(map[string]string)
	}

	return GetInterfaceDevicesFromConfig(interfacesConfig)
}

func (d *DnsmasqProvider) testConfig() error {
	cmd := exec.Command("dnsmasq", "--test", "-C", d.dhcpConfigPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("config test failed: %s: %w", string(output), err)
	}
	return nil
}
