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
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/vishvananda/netlink"
)

// getSocketPath returns the socket path, preferring JACK_SOCKET_PATH env var
func getSocketPath() string {
	if path := os.Getenv("JACK_SOCKET_PATH"); path != "" {
		return path
	}
	return "/var/run/jack.sock"
}

// SystemStatus holds overall system status information
type SystemStatus struct {
	System       SystemInfo        `json:"system"`
	Interfaces   []InterfaceStatus `json:"interfaces"`
	Daemon       DaemonStatus      `json:"daemon"`
	IPForwarding bool              `json:"ip_forwarding"`
}

// DaemonStatus holds daemon-specific information
type DaemonStatus struct {
	Uptime     string `json:"uptime"`
	ConfigPath string `json:"config_path"`
	PID        int    `json:"pid"`
	Running    bool   `json:"running"`
}

// InterfaceStatus holds status for a single interface
type InterfaceStatus struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Device    string   `json:"device"`
	State     string   `json:"state"` // up, down, unknown
	IPAddr    []string `json:"ipaddr"`
	MTU       int      `json:"mtu"`
	TXPackets uint64   `json:"tx_packets"`
	RXPackets uint64   `json:"rx_packets"`
	TXBytes   uint64   `json:"tx_bytes"`
	RXBytes   uint64   `json:"rx_bytes"`
	TXErrors  uint64   `json:"tx_errors"`
	RXErrors  uint64   `json:"rx_errors"`
}

// SystemInfo holds general system information
type SystemInfo struct {
	Hostname      string `json:"hostname"`
	KernelVersion string `json:"kernel_version"`
	Uptime        string `json:"uptime"`
}

// GetSystemStatus gathers comprehensive system status
func GetSystemStatus() (*SystemStatus, error) {
	status := &SystemStatus{}

	// Get daemon status
	status.Daemon = getDaemonStatus()

	// Get interface status
	interfaces, err := getInterfaceStatus()
	if err != nil {
		return nil, fmt.Errorf("failed to get interface status: %w", err)
	}
	status.Interfaces = interfaces

	// Get system info
	status.System = getSystemInfo()

	// Check IP forwarding (core networking feature)
	if data, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward"); err == nil {
		status.IPForwarding = strings.TrimSpace(string(data)) == "1"
	}

	return status, nil
}

func getDaemonStatus() DaemonStatus {
	status := DaemonStatus{
		Running:    false,
		ConfigPath: "/etc/jack",
	}

	// Check if daemon is running by checking socket
	if _, err := os.Stat(getSocketPath()); err == nil {
		status.Running = true
	}

	// Try to get PID from systemd
	cmd := exec.Command("systemctl", "show", "-p", "MainPID", "--value", "jack")
	if output, err := cmd.Output(); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(output))); err == nil && pid > 0 {
			status.PID = pid

			// Get process start time for uptime
			if stat, err := os.Stat(fmt.Sprintf("/proc/%d", pid)); err == nil {
				uptime := time.Since(stat.ModTime())
				status.Uptime = formatDuration(uptime)
			}
		}
	}

	return status
}

func getInterfaceStatus() ([]InterfaceStatus, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}

	var interfaces []InterfaceStatus

	for _, link := range links {
		attrs := link.Attrs()

		// Skip loopback
		if attrs.Name == "lo" {
			continue
		}

		iface := InterfaceStatus{
			Name:   attrs.Name,
			Device: attrs.Name,
			MTU:    attrs.MTU,
		}

		// Determine type
		switch link.Type() {
		case "bridge":
			iface.Type = "bridge"
		case "vlan":
			iface.Type = "vlan"
		case "device":
			iface.Type = "physical"
		default:
			iface.Type = link.Type()
		}

		// Get state
		if attrs.Flags&net.FlagUp != 0 {
			iface.State = "up"
		} else {
			iface.State = "down"
		}

		// Get IP addresses
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		if err == nil {
			for _, addr := range addrs {
				iface.IPAddr = append(iface.IPAddr, addr.IPNet.String())
			}
		}

		// Get statistics
		if attrs.Statistics != nil {
			iface.TXPackets = attrs.Statistics.TxPackets
			iface.RXPackets = attrs.Statistics.RxPackets
			iface.TXBytes = attrs.Statistics.TxBytes
			iface.RXBytes = attrs.Statistics.RxBytes
			iface.TXErrors = attrs.Statistics.TxErrors
			iface.RXErrors = attrs.Statistics.RxErrors
		}

		interfaces = append(interfaces, iface)
	}

	return interfaces, nil
}

func getSystemInfo() SystemInfo {
	info := SystemInfo{}

	// Get hostname
	if hostname, err := os.Hostname(); err == nil {
		info.Hostname = hostname
	}

	// Get kernel version
	if data, err := os.ReadFile("/proc/version"); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) >= 3 {
			info.KernelVersion = parts[2]
		}
	}

	// Get system uptime
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 1 {
			if seconds, err := strconv.ParseFloat(fields[0], 64); err == nil {
				duration := time.Duration(seconds * float64(time.Second))
				info.Uptime = formatDuration(duration)
			}
		}
	}

	return info
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
