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
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// LEDBasePath is the base path for LED sysfs interface
	LEDBasePath = "/sys/class/leds"
)

// ListLEDs discovers all available LEDs on the system
func ListLEDs() ([]string, error) {
	entries, err := os.ReadDir(LEDBasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to list LEDs at %s: %w", LEDBasePath, err)
	}

	var leds []string
	for _, entry := range entries {
		// Check if the LED path exists (handles symlinks correctly)
		ledPath := GetLEDPath(entry.Name())
		if _, err := os.Stat(ledPath); err == nil {
			leds = append(leds, entry.Name())
		}
	}

	return leds, nil
}

// GetLEDPath returns the full path to an LED's sysfs directory
func GetLEDPath(ledName string) string {
	return filepath.Join(LEDBasePath, ledName)
}

// readLEDFile reads a sysfs file for the given LED
func readLEDFile(ledName, fileName string) (string, error) {
	path := filepath.Join(GetLEDPath(ledName), fileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read %s for LED %s: %w", fileName, ledName, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// writeLEDFile writes a value to a sysfs file for the given LED
func writeLEDFile(ledName, fileName, value string) error {
	path := filepath.Join(GetLEDPath(ledName), fileName)
	err := os.WriteFile(path, []byte(value), 0644)
	if err != nil {
		return fmt.Errorf("failed to write %s to %s for LED %s: %w", value, fileName, ledName, err)
	}
	return nil
}

// SetBrightness sets the brightness of an LED (0 to max_brightness)
func SetBrightness(ledName string, brightness int) error {
	return writeLEDFile(ledName, "brightness", strconv.Itoa(brightness))
}

// GetBrightness gets the current brightness of an LED
func GetBrightness(ledName string) (int, error) {
	value, err := readLEDFile(ledName, "brightness")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(value)
}

// GetMaxBrightness gets the maximum brightness value for an LED
func GetMaxBrightness(ledName string) (int, error) {
	value, err := readLEDFile(ledName, "max_brightness")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(value)
}

// SetTrigger sets the trigger type for an LED
func SetTrigger(ledName, trigger string) error {
	return writeLEDFile(ledName, "trigger", trigger)
}

// GetTrigger gets the current trigger for an LED
// Returns the active trigger (the one in brackets, e.g., "[none]" -> "none")
func GetTrigger(ledName string) (string, error) {
	value, err := readLEDFile(ledName, "trigger")
	if err != nil {
		return "", err
	}

	return ParseActiveTrigger(value), nil
}

// GetAvailableTriggers gets all available triggers for an LED
func GetAvailableTriggers(ledName string) ([]string, error) {
	value, err := readLEDFile(ledName, "trigger")
	if err != nil {
		return nil, err
	}

	return ParseAvailableTriggers(value), nil
}

// SetDelayOn sets the delay_on parameter for timer trigger (milliseconds)
func SetDelayOn(ledName string, ms int) error {
	return writeLEDFile(ledName, "delay_on", strconv.Itoa(ms))
}

// SetDelayOff sets the delay_off parameter for timer trigger (milliseconds)
func SetDelayOff(ledName string, ms int) error {
	return writeLEDFile(ledName, "delay_off", strconv.Itoa(ms))
}

// SetPattern sets the pattern for pattern trigger
// Format: "brightness1 duration1 brightness2 duration2 ..."
func SetPattern(ledName, pattern string) error {
	return writeLEDFile(ledName, "pattern", pattern)
}

// SetRepeat sets the repeat count for pattern trigger (-1 for infinite)
func SetRepeat(ledName string, count int) error {
	return writeLEDFile(ledName, "repeat", strconv.Itoa(count))
}

// SetNetdevDevice sets the network device for netdev trigger
func SetNetdevDevice(ledName, deviceName string) error {
	return writeLEDFile(ledName, "device_name", deviceName)
}

// SetNetdevMode sets the mode for netdev trigger (e.g., "link tx rx")
// The mode string is parsed and each flag (link, tx, rx, etc.) is written as "1" to its corresponding file
func SetNetdevMode(ledName, mode string) error {
	// Validate mode using pure function
	if err := ValidateNetdevMode(mode); err != nil {
		return err
	}

	// Parse mode string into individual flags
	flags := strings.Fields(mode)

	// Enable requested flags (don't disable unrequested ones - let user manage that explicitly)
	for _, flag := range flags {
		if err := writeLEDFile(ledName, flag, "1"); err != nil {
			return fmt.Errorf("failed to enable %s: %w", flag, err)
		}
	}

	return nil
}

// LEDExists checks if an LED exists
func LEDExists(ledName string) bool {
	_, err := os.Stat(GetLEDPath(ledName))
	return err == nil
}
