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
	"os"
	"sync"
)

// FileBackend writes log entries to a file
type FileBackend struct {
	path   string
	format string // "json" or "text"
	file   *os.File
	mu     sync.Mutex
}

// NewFileBackend creates a new file backend
func NewFileBackend(path string, format string) (*FileBackend, error) {
	// Ensure parent directory exists
	if err := os.MkdirAll(path[:len(path)-len(path[findLastSlash(path):])], 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file for append
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &FileBackend{
		path:   path,
		format: format,
		file:   file,
	}, nil
}

// Write writes a log entry to the file
func (b *FileBackend) Write(entry *Entry) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	var output string
	if b.format == "json" {
		jsonBytes, err := entry.ToJSON()
		if err != nil {
			return fmt.Errorf("failed to marshal log entry: %w", err)
		}
		output = string(jsonBytes) + "\n"
	} else {
		output = entry.ToText() + "\n"
	}

	if _, err := b.file.WriteString(output); err != nil {
		return fmt.Errorf("failed to write to log file: %w", err)
	}

	return nil
}

// Close closes the log file
func (b *FileBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.file != nil {
		return b.file.Close()
	}
	return nil
}

// findLastSlash finds the index of the last slash in a path
func findLastSlash(path string) int {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return i
		}
	}
	return -1
}
