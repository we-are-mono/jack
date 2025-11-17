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

// Package main implements a self-contained LED control plugin for Jack.
package main

import (
	"fmt"
	"log"
	"sync"
)

// LEDProvider implements the core LED control logic
type LEDProvider struct {
	mu            sync.RWMutex
	appliedLEDs   map[string]LEDSettings // Track what we've configured
	originalState map[string]LEDSettings // Store original state for restore
}

// NewLEDProvider creates a new LED provider
func NewLEDProvider() *LEDProvider {
	return &LEDProvider{
		appliedLEDs:   make(map[string]LEDSettings),
		originalState: make(map[string]LEDSettings),
	}
}

// ApplyConfig applies the LED configuration
func (p *LEDProvider) ApplyConfig(config *LEDConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Validate all LEDs exist first
	for ledName := range config.LEDs {
		if !LEDExists(ledName) {
			return fmt.Errorf("LED does not exist: %s", ledName)
		}
	}

	// Store original state before first apply (for restoration)
	if len(p.originalState) == 0 {
		for ledName := range config.LEDs {
			if err := p.saveOriginalState(ledName); err != nil {
				log.Printf("[WARN] Failed to save original state for %s: %v", ledName, err)
			}
		}
	}

	// Apply each LED configuration
	for ledName, settings := range config.LEDs {
		if err := p.applyLEDSettings(ledName, settings); err != nil {
			return fmt.Errorf("failed to configure LED %s: %w", ledName, err)
		}
		p.appliedLEDs[ledName] = settings
	}

	log.Printf("[INFO] Applied LED configuration for %d LEDs", len(config.LEDs))
	return nil
}

// saveOriginalState saves the original state of an LED before modification
func (p *LEDProvider) saveOriginalState(ledName string) error {
	brightness, err := GetBrightness(ledName)
	if err != nil {
		return err
	}

	trigger, err := GetTrigger(ledName)
	if err != nil {
		return err
	}

	p.originalState[ledName] = LEDSettings{
		Brightness: brightness,
		Trigger:    trigger,
	}

	return nil
}

// applyLEDSettings applies settings to a single LED
func (p *LEDProvider) applyLEDSettings(ledName string, settings LEDSettings) error {
	// Set trigger first (if specified)
	if settings.Trigger != "" {
		if err := SetTrigger(ledName, settings.Trigger); err != nil {
			return fmt.Errorf("failed to set trigger: %w", err)
		}

		// Apply trigger-specific parameters
		switch settings.Trigger {
		case "timer":
			if settings.DelayOn > 0 {
				if err := SetDelayOn(ledName, settings.DelayOn); err != nil {
					return fmt.Errorf("failed to set delay_on: %w", err)
				}
			}
			if settings.DelayOff > 0 {
				if err := SetDelayOff(ledName, settings.DelayOff); err != nil {
					return fmt.Errorf("failed to set delay_off: %w", err)
				}
			}

		case "pattern":
			if settings.Pattern != "" {
				if err := SetPattern(ledName, settings.Pattern); err != nil {
					return fmt.Errorf("failed to set pattern: %w", err)
				}
			}
			if settings.Repeat != 0 {
				if err := SetRepeat(ledName, settings.Repeat); err != nil {
					return fmt.Errorf("failed to set repeat: %w", err)
				}
			}

		case "netdev":
			if settings.DeviceName != "" {
				if err := SetNetdevDevice(ledName, settings.DeviceName); err != nil {
					return fmt.Errorf("failed to set device_name: %w", err)
				}
			}
			if settings.Mode != "" {
				if err := SetNetdevMode(ledName, settings.Mode); err != nil {
					return fmt.Errorf("failed to set mode: %w", err)
				}
			}
		}
	}

	// Set brightness (always set if specified, even if trigger is active)
	if settings.Brightness >= 0 {
		if err := SetBrightness(ledName, settings.Brightness); err != nil {
			return fmt.Errorf("failed to set brightness: %w", err)
		}
	}

	return nil
}

// ValidateConfig validates the LED configuration
func (p *LEDProvider) ValidateConfig(config *LEDConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Check that all LEDs exist
	for ledName, settings := range config.LEDs {
		if !LEDExists(ledName) {
			return fmt.Errorf("LED does not exist: %s", ledName)
		}

		// Validate brightness
		if settings.Brightness < 0 {
			return fmt.Errorf("invalid brightness for %s: must be >= 0", ledName)
		}

		maxBrightness, err := GetMaxBrightness(ledName)
		if err != nil {
			return fmt.Errorf("failed to get max_brightness for %s: %w", ledName, err)
		}

		if settings.Brightness > maxBrightness {
			return fmt.Errorf("brightness %d exceeds max_brightness %d for %s", settings.Brightness, maxBrightness, ledName)
		}

		// Validate trigger if specified
		if settings.Trigger != "" {
			availTriggers, err := GetAvailableTriggers(ledName)
			if err != nil {
				return fmt.Errorf("failed to get available triggers for %s: %w", ledName, err)
			}

			validTrigger := false
			for _, t := range availTriggers {
				if t == settings.Trigger {
					validTrigger = true
					break
				}
			}

			if !validTrigger {
				return fmt.Errorf("invalid trigger '%s' for %s (available: %v)", settings.Trigger, ledName, availTriggers)
			}
		}
	}

	return nil
}

// Flush removes all LED configuration and restores original state
func (p *LEDProvider) Flush() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Restore original state for all LEDs we modified
	for ledName, originalSettings := range p.originalState {
		// Restore trigger first
		if originalSettings.Trigger != "" {
			if err := SetTrigger(ledName, originalSettings.Trigger); err != nil {
				log.Printf("[WARN] Failed to restore trigger for %s: %v", ledName, err)
			}
		}

		// Restore brightness
		if err := SetBrightness(ledName, originalSettings.Brightness); err != nil {
			log.Printf("[WARN] Failed to restore brightness for %s: %v", ledName, err)
		}
	}

	// Clear state
	p.appliedLEDs = make(map[string]LEDSettings)

	log.Printf("[INFO] Flushed LED configuration")
	return nil
}

// Status returns the current status of all LEDs
func (p *LEDProvider) Status() (interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	allLEDs, err := ListLEDs()
	if err != nil {
		log.Printf("[ERROR] Failed to list LEDs: %v", err)
		return nil, fmt.Errorf("failed to list LEDs: %w", err)
	}

	log.Printf("[DEBUG] Found %d LEDs: %v", len(allLEDs), allLEDs)

	status := &LEDStatus{
		LEDs: make([]LEDState, 0, len(allLEDs)),
	}

	for _, ledName := range allLEDs {
		brightness, err := GetBrightness(ledName)
		if err != nil {
			log.Printf("[WARN] Failed to get brightness for %s: %v", ledName, err)
			continue
		}

		maxBrightness, err := GetMaxBrightness(ledName)
		if err != nil {
			log.Printf("[WARN] Failed to get max_brightness for %s: %v", ledName, err)
			continue
		}

		currentTrigger, err := GetTrigger(ledName)
		if err != nil {
			log.Printf("[WARN] Failed to get trigger for %s: %v", ledName, err)
			continue
		}

		availTriggers, err := GetAvailableTriggers(ledName)
		if err != nil {
			log.Printf("[WARN] Failed to get available triggers for %s: %v", ledName, err)
			availTriggers = []string{}
		}

		status.LEDs = append(status.LEDs, LEDState{
			Name:           ledName,
			Brightness:     brightness,
			MaxBrightness:  maxBrightness,
			CurrentTrigger: currentTrigger,
			AvailTriggers:  availTriggers,
		})
	}

	return status, nil
}
