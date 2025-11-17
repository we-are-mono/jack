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

// Package cmd implements the CLI commands for Jack using cobra.
package cmd

import (
	"fmt"
	"log"
	"strings"

	"github.com/spf13/cobra"
	"github.com/we-are-mono/jack/client"
	"github.com/we-are-mono/jack/daemon"
)

var (
	verboseStatus bool
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show system and configuration status",
	Long:  `Displays system status including daemon, interfaces, services, and pending changes.`,
	Run:   runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().BoolVarP(&verboseStatus, "verbose", "v", false, "Show detailed status")
}

func runStatus(cmd *cobra.Command, args []string) {
	// Get system info
	resp, err := client.Send(daemon.Request{
		Command: "info",
	})

	if err != nil {
		log.Fatalf("[ERROR] %v", err)
	}

	if !resp.Success {
		log.Fatalf("[ERROR] %s", resp.Error)
	}

	// Parse response data
	if verboseStatus {
		printVerboseStatus(resp.Data)
	} else {
		printCompactStatus(resp.Data)
	}
}

func printCompactStatus(data interface{}) {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		fmt.Println("Unable to parse status data")
		return
	}

	fmt.Println("Jack Network Configuration Daemon")
	fmt.Println("==================================")
	fmt.Println()

	// Daemon status
	if daemonData, ok := dataMap["daemon"].(map[string]interface{}); ok {
		if running, ok := daemonData["Running"].(bool); ok && running {
			fmt.Printf("[OK] Daemon:     Running (PID: %v)\n", daemonData["PID"])
		} else {
			fmt.Println("[DOWN] Daemon:   Not running")
		}
	}

	// System info
	if sysData, ok := dataMap["system"].(map[string]interface{}); ok {
		fmt.Printf("  Hostname:   %v\n", sysData["Hostname"])
		fmt.Printf("  Uptime:     %v\n", sysData["Uptime"])
	}

	fmt.Println()

	// Configuration status
	pending := false
	if p, ok := dataMap["pending"].(bool); ok {
		pending = p
	}

	if pending {
		fmt.Println("[WARN] Configuration: Pending changes (use 'jack diff' to view)")
	} else {
		fmt.Println("[OK] Configuration:   No pending changes")
	}

	fmt.Println()

	// IP Forwarding (core network feature)
	if ipf, ok := dataMap["ip_forwarding"].(bool); ok && ipf {
		fmt.Println("[OK] IP Forwarding: Enabled")
	} else {
		fmt.Println("[INFO] IP Forwarding: Disabled")
	}

	// Plugins
	if pluginsData, ok := dataMap["plugins"].(map[string]interface{}); ok && len(pluginsData) > 0 {
		fmt.Println()
		fmt.Println("Plugins:")
		for namespace, pluginStatus := range pluginsData {
			printPluginStatus(namespace, pluginStatus)
		}
	}

	fmt.Println()

	// Interface summary
	if interfacesData, ok := dataMap["interfaces"].([]interface{}); ok {
		upCount := 0
		downCount := 0

		for _, ifaceData := range interfacesData {
			if iface, ok := ifaceData.(map[string]interface{}); ok {
				if state, ok := iface["State"].(string); ok {
					if state == "up" {
						upCount++
					} else {
						downCount++
					}
				}
			}
		}

		fmt.Printf("Interfaces: %d up", upCount)
		if downCount > 0 {
			fmt.Printf(", %d down", downCount)
		}
		fmt.Println()
	}

	fmt.Println()
	fmt.Println("Use 'jack status -v' for detailed interface information")
}

func printVerboseStatus(data interface{}) {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		fmt.Println("Unable to parse status data")
		return
	}

	fmt.Println("Jack Network Configuration Daemon - Detailed Status")
	fmt.Println("====================================================")
	fmt.Println()

	// Daemon details
	fmt.Println("DAEMON")
	fmt.Println("------")
	if daemonData, ok := dataMap["daemon"].(map[string]interface{}); ok {
		running, _ := daemonData["Running"].(bool) //nolint:errcheck // Default to false if not present
		fmt.Printf("Status:      %s\n", boolToStatus(running))
		if running {
			fmt.Printf("PID:         %v\n", daemonData["PID"])
			if uptime, ok := daemonData["Uptime"].(string); ok && uptime != "" {
				fmt.Printf("Uptime:      %s\n", uptime)
			}
		}
		fmt.Printf("Config Path: %v\n", daemonData["ConfigPath"])
	}
	fmt.Println()

	// System details
	fmt.Println("SYSTEM")
	fmt.Println("------")
	if sysData, ok := dataMap["system"].(map[string]interface{}); ok {
		fmt.Printf("Hostname:       %v\n", sysData["Hostname"])
		fmt.Printf("Kernel:         %v\n", sysData["KernelVersion"])
		fmt.Printf("System Uptime:  %v\n", sysData["Uptime"])
	}
	fmt.Println()

	// Configuration
	fmt.Println("CONFIGURATION")
	fmt.Println("-------------")
	pending := false
	if p, ok := dataMap["pending"].(bool); ok {
		pending = p
	}
	fmt.Printf("Pending Changes: %s\n", boolToYesNo(pending))
	if pending {
		fmt.Println("                 (use 'jack diff' to view)")
	}
	fmt.Println()

	// IP Forwarding (core network feature)
	fmt.Println("IP FORWARDING")
	fmt.Println("-------------")
	if ipf, ok := dataMap["ip_forwarding"].(bool); ok {
		fmt.Printf("Status: %s\n", boolToStatus(ipf))
	}
	fmt.Println()

	// Plugins
	if pluginsData, ok := dataMap["plugins"].(map[string]interface{}); ok && len(pluginsData) > 0 {
		fmt.Println("PLUGINS")
		fmt.Println("-------")
		for namespace, pluginStatus := range pluginsData {
			printPluginStatusVerbose(namespace, pluginStatus)
			fmt.Println()
		}
	}

	// Interfaces
	fmt.Println("INTERFACES")
	fmt.Println("----------")
	if interfacesData, ok := dataMap["interfaces"].([]interface{}); ok {
		for _, ifaceData := range interfacesData {
			if iface, ok := ifaceData.(map[string]interface{}); ok {
				printInterface(iface)
			}
		}
	}
}

func printInterface(iface map[string]interface{}) {
	name, _ := iface["Name"].(string)      //nolint:errcheck // Default to empty if not present
	ifaceType, _ := iface["Type"].(string) //nolint:errcheck // Default to empty if not present
	state, _ := iface["State"].(string)    //nolint:errcheck // Default to empty if not present

	stateSymbol := "[UP]"
	if state != "up" {
		stateSymbol = "[DOWN]"
	}

	fmt.Printf("%s %s (%s)\n", stateSymbol, name, ifaceType)
	fmt.Printf("    State:      %s\n", state)

	// IP addresses
	if ipAddrs, ok := iface["IPAddr"].([]interface{}); ok && len(ipAddrs) > 0 {
		fmt.Printf("    IP Address: ")
		for i, ip := range ipAddrs {
			if i > 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("%v", ip)
		}
		fmt.Println()
	} else {
		fmt.Println("    IP Address: (none)")
	}

	// MTU
	if mtu, ok := iface["MTU"].(float64); ok && mtu > 0 {
		fmt.Printf("    MTU:        %d\n", int(mtu))
	}

	// Statistics
	if rxPackets, ok := iface["RXPackets"].(float64); ok {
		fmt.Printf("    RX:         %d packets", int(rxPackets))
		if rxBytes, ok := iface["RXBytes"].(float64); ok {
			fmt.Printf(" (%s)", formatBytes(int64(rxBytes)))
		}
		if rxErrors, ok := iface["RXErrors"].(float64); ok && rxErrors > 0 {
			fmt.Printf(" [%d errors]", int(rxErrors))
		}
		fmt.Println()
	}

	if txPackets, ok := iface["TXPackets"].(float64); ok {
		fmt.Printf("    TX:         %d packets", int(txPackets))
		if txBytes, ok := iface["TXBytes"].(float64); ok {
			fmt.Printf(" (%s)", formatBytes(int64(txBytes)))
		}
		if txErrors, ok := iface["TXErrors"].(float64); ok && txErrors > 0 {
			fmt.Printf(" [%d errors]", int(txErrors))
		}
		fmt.Println()
	}

	fmt.Println()
}

func boolToStatus(b bool) string {
	if b {
		return "Active"
	}
	return "Inactive"
}

func boolToYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func printPluginStatus(namespace string, pluginStatus interface{}) {
	statusMap, ok := pluginStatus.(map[string]interface{})
	if !ok {
		return
	}

	// Try to get enabled/running status
	enabled := false
	if e, ok := statusMap["enabled"].(bool); ok {
		enabled = e
	} else if r, ok := statusMap["running"].(bool); ok {
		enabled = r
	}

	status := "[UP]"
	if !enabled {
		status = "[DOWN]"
	}

	fmt.Printf("  %s %-12s", status, namespace)

	// Print brief summary if available
	if provider, ok := statusMap["provider"].(string); ok {
		fmt.Printf(" (%s)", provider)
	}
	if leaseCount, ok := statusMap["lease_count"].(float64); ok {
		fmt.Printf(" - %d leases", int(leaseCount))
	}
	if ruleCount, ok := statusMap["rule_count"].(float64); ok {
		fmt.Printf(" - %d rules", int(ruleCount))
	}
	fmt.Println()
}

func printPluginStatusVerbose(namespace string, pluginStatus interface{}) {
	statusMap, ok := pluginStatus.(map[string]interface{})
	if !ok {
		return
	}

	fmt.Printf("%s\n", namespace)

	// Print all status fields
	for key, value := range statusMap {
		// Skip complex nested data structures for monitoring plugin
		if namespace == "monitoring" && (key == "system_metrics" || key == "interface_metrics") {
			continue
		}

		// Format the key nicely
		formattedKey := toTitleCase(strings.ReplaceAll(key, "_", " "))

		// Format the value based on type
		switch v := value.(type) {
		case bool:
			fmt.Printf("  %-12s %s\n", formattedKey+":", boolToStatus(v))
		case float64:
			fmt.Printf("  %-12s %.0f\n", formattedKey+":", v)
		case string:
			fmt.Printf("  %-12s %s\n", formattedKey+":", v)
		default:
			// Skip printing complex data structures
			continue
		}
	}
}

// toTitleCase converts a string to title case (first letter of each word capitalized)
func toTitleCase(s string) string {
	words := strings.Fields(s)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}
