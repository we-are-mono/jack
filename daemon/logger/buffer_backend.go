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
	"bytes"
	"fmt"
	"sync"
)

// BufferBackend writes log entries to a buffer (for testing)
type BufferBackend struct {
	buffer *bytes.Buffer
	format string // "json" or "text"
	mu     sync.Mutex
}

// NewBufferBackend creates a new buffer backend
func NewBufferBackend(buffer *bytes.Buffer, format string) *BufferBackend {
	return &BufferBackend{
		buffer: buffer,
		format: format,
	}
}

// Write writes a log entry to the buffer
func (b *BufferBackend) Write(entry *Entry) error {
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

	if _, err := b.buffer.WriteString(output); err != nil {
		return fmt.Errorf("failed to write to buffer: %w", err)
	}

	return nil
}

// Close is a no-op for buffer backend
func (b *BufferBackend) Close() error {
	return nil
}
