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
	"encoding/json"
	"log"
	"sync"
	"time"
)

// InterfaceHistory stores bandwidth history for graphing
type InterfaceHistory struct {
	RXRates    []float64
	TXRates    []float64
	Timestamps []time.Time
	maxSize    int
}

// NewInterfaceHistory creates a new history buffer with the specified size
func NewInterfaceHistory(size int) *InterfaceHistory {
	return &InterfaceHistory{
		RXRates:    make([]float64, 0, size),
		TXRates:    make([]float64, 0, size),
		Timestamps: make([]time.Time, 0, size),
		maxSize:    size,
	}
}

// Add appends a new data point to the history (ring buffer)
func (h *InterfaceHistory) Add(rxRate, txRate float64, timestamp time.Time) {
	if len(h.RXRates) >= h.maxSize {
		// Remove oldest entry (shift left)
		h.RXRates = h.RXRates[1:]
		h.TXRates = h.TXRates[1:]
		h.Timestamps = h.Timestamps[1:]
	}
	h.RXRates = append(h.RXRates, rxRate)
	h.TXRates = append(h.TXRates, txRate)
	h.Timestamps = append(h.Timestamps, timestamp)
}

// MonitoringProvider implements monitoring and metrics collection
type MonitoringProvider struct {
	lastUpdate           time.Time
	systemMetrics        *SystemMetrics
	config               *MonitoringConfig
	stopChan             chan struct{}
	interfaceMetrics     []InterfaceMetrics
	prevInterfaceMetrics []InterfaceMetrics
	bandwidthHistory     map[string]*InterfaceHistory // Interface name -> history
	mu                   sync.RWMutex
}

// NewMonitoringProvider creates a new monitoring provider
func NewMonitoringProvider() *MonitoringProvider {
	return &MonitoringProvider{
		lastUpdate:       time.Now(),
		stopChan:         make(chan struct{}),
		bandwidthHistory: make(map[string]*InterfaceHistory),
	}
}

// ApplyConfig applies monitoring configuration and starts collection
func (p *MonitoringProvider) ApplyConfig(config *MonitoringConfig) error {
	log.Printf("[MONITOR] ApplyConfig called with: %+v", config)

	p.mu.Lock()
	p.config = config
	p.mu.Unlock()

	if config == nil {
		log.Println("[MONITOR] ERROR: config is nil!")
		return nil
	}

	if !config.Enabled {
		log.Printf("[MONITOR] Monitoring disabled (Enabled=%v)", config.Enabled)
		return p.Stop()
	}

	// Stop any existing monitoring loop
	if err := p.Stop(); err != nil {
		log.Printf("[WARN] Failed to stop existing monitoring: %v", err)
	}

	// Create new stopChan for the new monitoring loop
	p.mu.Lock()
	p.stopChan = make(chan struct{})
	p.mu.Unlock()

	log.Printf("[MONITOR] Starting monitoring collection... (Enabled=%v, Interval=%d)", config.Enabled, config.CollectionInterval)
	go p.monitoringLoop()

	return nil
}

// Validate validates monitoring configuration
func (p *MonitoringProvider) Validate(config *MonitoringConfig) error {
	// No validation needed for now
	return nil
}

// Stop stops the monitoring collection
func (p *MonitoringProvider) Stop() error {
	select {
	case <-p.stopChan:
		// Already stopped
	default:
		close(p.stopChan)
		log.Println("[MONITOR] Stopped monitoring collection")
	}
	return nil
}

// Status returns the current monitoring status
func (p *MonitoringProvider) Status() (interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"enabled":           p.config != nil && p.config.Enabled,
		"last_update":       p.lastUpdate,
		"system_metrics":    p.systemMetrics,
		"interface_metrics": p.interfaceMetrics,
	}, nil
}

// monitoringLoop continuously collects metrics
func (p *MonitoringProvider) monitoringLoop() {
	// Collect initial metrics
	p.updateMetrics()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.updateMetrics()
		case <-p.stopChan:
			return
		}
	}
}

// updateMetrics collects current metrics and updates the store
func (p *MonitoringProvider) updateMetrics() {
	// Collect system metrics
	sysMetrics := GetSystemMetrics()
	p.mu.Lock()
	p.systemMetrics = sysMetrics
	p.mu.Unlock()

	// Collect interface metrics
	ifMetrics, err := GetInterfaceMetrics()
	if err != nil {
		log.Printf("[MONITOR] Failed to collect interface metrics: %v", err)
		return
	}

	p.mu.Lock()
	// Store previous metrics for rate calculation
	p.prevInterfaceMetrics = p.interfaceMetrics
	p.interfaceMetrics = ifMetrics
	p.lastUpdate = time.Now()

	// Calculate rates if we have previous data
	if len(p.prevInterfaceMetrics) > 0 {
		p.interfaceMetrics = CalculateRates(
			p.prevInterfaceMetrics,
			p.interfaceMetrics,
		)

		// Update bandwidth history for graphing (12 points = 1 minute at 5s intervals)
		for _, metric := range p.interfaceMetrics {
			if _, exists := p.bandwidthHistory[metric.Name]; !exists {
				p.bandwidthHistory[metric.Name] = NewInterfaceHistory(12)
			}
			// Convert bytes/sec to Mbps for graphing
			rxMbps := float64(metric.RXBytesRate) * 8 / 1000000
			txMbps := float64(metric.TXBytesRate) * 8 / 1000000
			p.bandwidthHistory[metric.Name].Add(rxMbps, txMbps, metric.Timestamp)
		}
	}
	p.mu.Unlock()
}

// GetMetrics returns current metrics as JSON for RPC
func (p *MonitoringProvider) GetMetrics() ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	data := map[string]interface{}{
		"system_metrics":    p.systemMetrics,
		"interface_metrics": p.interfaceMetrics,
		"last_update":       p.lastUpdate,
	}

	return json.Marshal(data)
}
