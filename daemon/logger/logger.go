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
	"fmt"
	"os"
	"sync"
)

// Logger is the interface for structured logging
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	With(fields ...Field) Logger // Create child logger with preset fields
}

// Field represents a structured log field
type Field struct {
	Key   string
	Value interface{}
}

// Backend is the interface for log output backends
type Backend interface {
	Write(entry *Entry) error
	Close() error
}

// Config holds logger configuration
type Config struct {
	Level     string   // debug, info, warn, error
	Format    string   // text, json
	Outputs   []string // file, journald
	FilePath  string   // Path to log file
	Component string   // Default component name
}

// standardLogger is the default implementation of Logger
type standardLogger struct {
	level     LogLevel
	format    string
	backends  []Backend
	emitter   *Emitter
	component string
	fields    map[string]interface{}
	mu        sync.RWMutex
}

// LogLevel represents the severity level of a log message
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

// ParseLevel converts a string to a LogLevel
func ParseLevel(level string) LogLevel {
	switch level {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// String returns the string representation of a LogLevel
func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "info"
	}
}

// New creates a new logger with the given configuration and backends
func New(config Config, backends []Backend, emitter *Emitter) Logger {
	return &standardLogger{
		level:     ParseLevel(config.Level),
		format:    config.Format,
		backends:  backends,
		emitter:   emitter,
		component: config.Component,
		fields:    make(map[string]interface{}),
	}
}

// Debug logs a debug message
func (l *standardLogger) Debug(msg string, fields ...Field) {
	l.log(LevelDebug, msg, fields...)
}

// Info logs an info message
func (l *standardLogger) Info(msg string, fields ...Field) {
	l.log(LevelInfo, msg, fields...)
}

// Warn logs a warning message
func (l *standardLogger) Warn(msg string, fields ...Field) {
	l.log(LevelWarn, msg, fields...)
}

// Error logs an error message
func (l *standardLogger) Error(msg string, fields ...Field) {
	l.log(LevelError, msg, fields...)
}

// With creates a child logger with preset fields
func (l *standardLogger) With(fields ...Field) Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Copy existing fields
	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}

	// Add new fields
	for _, f := range fields {
		newFields[f.Key] = f.Value
		// Special handling for component field
		if f.Key == "component" {
			if s, ok := f.Value.(string); ok {
				return &standardLogger{
					level:     l.level,
					format:    l.format,
					backends:  l.backends,
					emitter:   l.emitter,
					component: s,
					fields:    newFields,
				}
			}
		}
	}

	return &standardLogger{
		level:     l.level,
		format:    l.format,
		backends:  l.backends,
		emitter:   l.emitter,
		component: l.component,
		fields:    newFields,
	}
}

// log is the internal method that performs the actual logging
func (l *standardLogger) log(level LogLevel, msg string, fields ...Field) {
	// Check if we should log this level
	if level < l.level {
		return
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	// Merge preset fields with new fields
	mergedFields := make(map[string]interface{})
	for k, v := range l.fields {
		mergedFields[k] = v
	}
	for _, f := range fields {
		mergedFields[f.Key] = f.Value
	}

	// Create log entry
	entry := NewEntry(level.String(), l.component, msg, mergedFields)

	// Write to all backends
	for _, backend := range l.backends {
		if err := backend.Write(entry); err != nil {
			// Log backend errors to stderr (fallback)
			fmt.Fprintf(os.Stderr, "Logger backend error: %v\n", err)
		}
	}

	// Emit event to plugins
	if l.emitter != nil {
		l.emitter.Emit(entry)
	}
}

// Global logger instance
var std Logger
var globalEmitter *Emitter

// Init initializes the global logger
func Init(config Config, backends []Backend, emitter *Emitter) {
	globalEmitter = emitter
	std = New(config, backends, emitter)
}

// GetEmitter returns the global emitter for plugin subscription
func GetEmitter() *Emitter {
	return globalEmitter
}

// Debug logs a debug message using the global logger
func Debug(msg string, fields ...Field) {
	if std != nil {
		std.Debug(msg, fields...)
	}
}

// Info logs an info message using the global logger
func Info(msg string, fields ...Field) {
	if std != nil {
		std.Info(msg, fields...)
	}
}

// Warn logs a warning message using the global logger
func Warn(msg string, fields ...Field) {
	if std != nil {
		std.Warn(msg, fields...)
	}
}

// Error logs an error message using the global logger
func Error(msg string, fields ...Field) {
	if std != nil {
		std.Error(msg, fields...)
	}
}
