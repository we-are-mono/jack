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

// Package main implements the monitoring plugin for Jack.
package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vishvananda/netlink"
)

// SystemMetrics represents system-wide resource usage
type SystemMetrics struct {
	Timestamp     time.Time
	MemoryUsedMB  uint64
	MemoryTotalMB uint64
	CPUPercent    float64
	MemoryPercent float64
	LoadAvg1      float64
	LoadAvg5      float64
	LoadAvg15     float64
}

// InterfaceMetrics represents network interface statistics
type InterfaceMetrics struct {
	Timestamp   time.Time
	Name        string
	State       string
	RXBytes     uint64
	TXBytes     uint64
	RXPackets   uint64
	TXPackets   uint64
	RXErrors    uint64
	TXErrors    uint64
	RXBytesRate uint64 // Bytes per second
	TXBytesRate uint64 // Bytes per second
}

// GetSystemMetrics collects current system metrics
func GetSystemMetrics() *SystemMetrics {
	metrics := &SystemMetrics{
		Timestamp: time.Now(),
	}

	// Get CPU usage
	cpuPercent, err := getCPUPercent()
	if err == nil {
		metrics.CPUPercent = cpuPercent
	}

	// Get memory info
	memUsed, memTotal, err := getMemoryInfo()
	if err == nil {
		metrics.MemoryUsedMB = memUsed
		metrics.MemoryTotalMB = memTotal
		if memTotal > 0 {
			metrics.MemoryPercent = float64(memUsed) / float64(memTotal) * 100
		}
	}

	// Get load average
	load1, load5, load15, err := getLoadAverage()
	if err == nil {
		metrics.LoadAvg1 = load1
		metrics.LoadAvg5 = load5
		metrics.LoadAvg15 = load15
	}

	return metrics
}

// GetInterfaceMetrics collects metrics for all network interfaces
func GetInterfaceMetrics() ([]InterfaceMetrics, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, fmt.Errorf("failed to list links: %w", err)
	}

	var metrics []InterfaceMetrics
	for _, link := range links {
		attrs := link.Attrs()

		// Skip loopback
		if attrs.Name == "lo" {
			continue
		}

		// Get link statistics
		stats := attrs.Statistics
		if stats == nil {
			continue
		}

		state := "DOWN"
		if attrs.Flags&net.FlagUp != 0 {
			state = "UP"
		}

		metrics = append(metrics, InterfaceMetrics{
			Name:      attrs.Name,
			State:     state,
			RXBytes:   stats.RxBytes,
			TXBytes:   stats.TxBytes,
			RXPackets: stats.RxPackets,
			TXPackets: stats.TxPackets,
			RXErrors:  stats.RxErrors,
			TXErrors:  stats.TxErrors,
			Timestamp: time.Now(),
		})
	}

	return metrics, nil
}

// getCPUPercent calculates CPU usage percentage
func getCPUPercent() (float64, error) {
	// Read /proc/stat twice with a small interval
	idle1, total1, err := readCPUStat()
	if err != nil {
		return 0, err
	}

	time.Sleep(100 * time.Millisecond)

	idle2, total2, err := readCPUStat()
	if err != nil {
		return 0, err
	}

	idleDelta := idle2 - idle1
	totalDelta := total2 - total1

	if totalDelta == 0 {
		return 0, nil
	}

	cpuPercent := 100.0 * (1.0 - float64(idleDelta)/float64(totalDelta))
	return cpuPercent, nil
}

// readCPUStat reads CPU statistics from /proc/stat
func readCPUStat() (idle, total uint64, err error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 0, 0, fmt.Errorf("invalid /proc/stat format")
		}

		// Fields: cpu user nice system idle iowait irq softirq...
		for i := 1; i < len(fields); i++ {
			val, _ := strconv.ParseUint(fields[i], 10, 64) //nolint:errcheck // Use 0 if parse fails
			total += val
			if i == 4 { // idle is the 4th field (index 4)
				idle = val
			}
		}
		break
	}

	return idle, total, scanner.Err()
}

// getMemoryInfo reads memory information from /proc/meminfo
func getMemoryInfo() (usedMB, totalMB uint64, err error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	var memTotal, memFree, memBuffers, memCached uint64

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		value, _ := strconv.ParseUint(fields[1], 10, 64) //nolint:errcheck // Use 0 if parse fails

		switch fields[0] {
		case "MemTotal:":
			memTotal = value
		case "MemFree:":
			memFree = value
		case "Buffers:":
			memBuffers = value
		case "Cached:":
			memCached = value
		}
	}

	// Convert from KB to MB
	totalMB = memTotal / 1024
	// Used = Total - Free - Buffers - Cached
	usedMB = (memTotal - memFree - memBuffers - memCached) / 1024

	return usedMB, totalMB, scanner.Err()
}

// getLoadAverage reads load average from /proc/loadavg
func getLoadAverage() (load1, load5, load15 float64, err error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}

	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("invalid /proc/loadavg format")
	}

	load1, _ = strconv.ParseFloat(fields[0], 64)  //nolint:errcheck // Use 0 if parse fails
	load5, _ = strconv.ParseFloat(fields[1], 64)  //nolint:errcheck // Use 0 if parse fails
	load15, _ = strconv.ParseFloat(fields[2], 64) //nolint:errcheck // Use 0 if parse fails

	return load1, load5, load15, nil
}

// CalculateRates calculates bandwidth rates between two metric snapshots
func CalculateRates(prev, current []InterfaceMetrics) []InterfaceMetrics {
	result := make([]InterfaceMetrics, 0, len(current))

	// Create a map of previous metrics for quick lookup
	prevMap := make(map[string]InterfaceMetrics)
	for _, m := range prev {
		prevMap[m.Name] = m
	}

	for _, curr := range current {
		metric := curr

		if prev, ok := prevMap[curr.Name]; ok {
			timeDelta := curr.Timestamp.Sub(prev.Timestamp).Seconds()
			if timeDelta > 0 {
				metric.RXBytesRate = uint64(float64(curr.RXBytes-prev.RXBytes) / timeDelta)
				metric.TXBytesRate = uint64(float64(curr.TXBytes-prev.TXBytes) / timeDelta)
			}
		}

		result = append(result, metric)
	}

	return result
}
