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

package logger

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// JournaldBackend writes log entries to systemd journal
type JournaldBackend struct {
	format string // "json" or "text"
	mu     sync.Mutex
}

// NewJournaldBackend creates a new journald backend
// Returns an error if systemd journal is not available
func NewJournaldBackend(format string) (*JournaldBackend, error) {
	// Check if systemd-cat is available
	if _, err := exec.LookPath("systemd-cat"); err != nil {
		return nil, fmt.Errorf("systemd-cat not found: %w", err)
	}

	return &JournaldBackend{
		format: format,
	}, nil
}

// Write writes a log entry to systemd journal
func (b *JournaldBackend) Write(entry *Entry) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	var output string
	if b.format == "json" {
		jsonBytes, err := entry.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to marshal log entry: %w", err)
		}
		output = string(jsonBytes)
	} else {
		output = entry.ToText()
	}

	// Determine systemd priority based on log level
	priority := "6" // info
	switch entry.Level {
	case "debug":
		priority = "7"
	case "info":
		priority = "6"
	case "warn":
		priority = "4"
	case "error":
		priority = "3"
	}

	// Write to journal using systemd-cat
	cmd := exec.Command("systemd-cat", "-t", "jack", "-p", priority)
	cmd.Stdin = strings.NewReader(output)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to write to journal: %w", err)
	}

	return nil
}

// Close closes the journald backend
func (b *JournaldBackend) Close() error {
	// Nothing to close for journald
	return nil
}
