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

// Package state manages configuration state and persistence for Jack.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	defaultConfigBasePath = "/etc/jack"
)

// GetConfigDir returns the configuration directory path.
// Checks JACK_CONFIG_DIR environment variable, falls back to /etc/jack
func GetConfigDir() string {
	if dir := os.Getenv("JACK_CONFIG_DIR"); dir != "" {
		return dir
	}
	return defaultConfigBasePath
}

// LoadConfig loads configuration for a given namespace from the config file
// The config parameter should be a pointer to the config struct to unmarshal into
func LoadConfig(namespace string, config interface{}) error {
	path := filepath.Join(GetConfigDir(), namespace+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read %s config: %w", namespace, err)
	}

	if err := json.Unmarshal(data, config); err != nil {
		// Provide more helpful error message for JSON syntax errors
		if syntaxErr, ok := err.(*json.SyntaxError); ok {
			// Calculate line and column of the error
			line, col := getLineCol(data, syntaxErr.Offset)
			return fmt.Errorf("failed to parse %s config at %s line %d, column %d: %w",
				namespace, path, line, col, err)
		}
		return fmt.Errorf("failed to parse %s config: %w", namespace, err)
	}

	return nil
}

// getLineCol calculates the line and column number for a byte offset in JSON data
func getLineCol(data []byte, offset int64) (line, col int) {
	line = 1
	col = 1
	for i := int64(0); i < offset && i < int64(len(data)); i++ {
		if data[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return
}

// SaveConfig saves configuration for a given namespace to the config file
// Automatically creates backups and uses atomic writes
func SaveConfig(namespace string, config interface{}) error {
	path := filepath.Join(GetConfigDir(), namespace+".json")

	// Create backup if file exists
	if _, err := os.Stat(path); err == nil {
		backupPath := fmt.Sprintf("%s.backup.%s", path, time.Now().Format("20060102-150405"))
		if err := copyFile(path, backupPath); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal %s config: %w", namespace, err)
	}

	// Write atomically (temp file + rename)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, 0600)
}

// UnmarshalJSON unmarshals JSON data with enhanced error reporting
func UnmarshalJSON(data []byte, v interface{}) error {
	if err := json.Unmarshal(data, v); err != nil {
		// Provide more helpful error message for JSON syntax errors
		if syntaxErr, ok := err.(*json.SyntaxError); ok {
			line, col := getLineCol(data, syntaxErr.Offset)
			return fmt.Errorf("JSON syntax error at line %d, column %d: %w", line, col, err)
		}
		return err
	}
	return nil
}
