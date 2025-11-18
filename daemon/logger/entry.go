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

// Package logger provides structured logging for the Jack daemon.
package logger

import (
	"encoding/json"
	"time"
)

// Entry represents a single log entry with structured fields
type Entry struct {
	Timestamp string                 `json:"timestamp"` // RFC3339 format
	Level     string                 `json:"level"`     // debug, info, warn, error
	Component string                 `json:"component"` // observer, loader, server, etc.
	Message   string                 `json:"message"`   // Log message
	Fields    map[string]interface{} `json:"fields"`    // Additional structured fields
}

// NewEntry creates a new log entry with the current timestamp
func NewEntry(level, component, message string, fields map[string]interface{}) *Entry {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	return &Entry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level,
		Component: component,
		Message:   message,
		Fields:    fields,
	}
}

// ToJSON returns the JSON representation of the log entry
func (e *Entry) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// ToText returns a human-readable text representation of the log entry
func (e *Entry) ToText() string {
	levelStr := "[" + e.Level + "]"
	componentStr := ""
	if e.Component != "" {
		componentStr = " [" + e.Component + "]"
	}

	fieldsStr := ""
	if len(e.Fields) > 0 {
		// Format fields as key=value pairs
		for k, v := range e.Fields {
			fieldsStr += " " + k + "=" + jsonString(v)
		}
	}

	return e.Timestamp + " " + levelStr + componentStr + " " + e.Message + fieldsStr
}

// jsonString converts a value to a JSON string representation
func jsonString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
