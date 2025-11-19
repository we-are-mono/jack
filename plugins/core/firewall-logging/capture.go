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

package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

const (
	// KernelLogDevice is the path to the kernel log device
	KernelLogDevice = "/dev/kmsg"
)

// DatabaseOperations defines the interface for database operations
type DatabaseOperations interface {
	InsertLog(entry *FirewallLogEntry) error
	CleanupOldLogs(retentionDays int) (int64, error)
	EnforceMaxEntries(maxEntries int) (int64, error)
	Vacuum() error
}

// LogCapture handles capturing and processing kernel firewall logs
type LogCapture struct {
	db              DatabaseOperations
	config          *FirewallLoggingConfig
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	samplingCounter int
	rateLimiter     *RateLimiter
}

// RateLimiter implements a simple token bucket rate limiter
type RateLimiter struct {
	maxPerSec int
	tokens    int
	lastReset time.Time
	mu        sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(maxPerSec int) *RateLimiter {
	return &RateLimiter{
		maxPerSec: maxPerSec,
		tokens:    maxPerSec,
		lastReset: time.Now(),
	}
}

// Allow checks if an action is allowed under the rate limit
func (r *RateLimiter) Allow() bool {
	if r.maxPerSec <= 0 {
		return true // No rate limiting
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Reset tokens if a second has passed
	now := time.Now()
	if now.Sub(r.lastReset) >= time.Second {
		r.tokens = r.maxPerSec
		r.lastReset = now
	}

	// Check if we have tokens available
	if r.tokens > 0 {
		r.tokens--
		return true
	}

	return false
}

// NewLogCapture creates a new log capture instance
func NewLogCapture(db DatabaseOperations, config *FirewallLoggingConfig) *LogCapture {
	ctx, cancel := context.WithCancel(context.Background())
	return &LogCapture{
		db:          db,
		config:      config,
		ctx:         ctx,
		cancel:      cancel,
		rateLimiter: NewRateLimiter(config.RateLimitPerSec),
	}
}

// Start begins capturing kernel logs in a background goroutine
func (c *LogCapture) Start() error {
	if !c.config.Enabled {
		return nil // Not enabled, don't start
	}

	c.wg.Add(1)
	go c.captureLoop()

	return nil
}

// Stop stops the log capture and waits for cleanup
func (c *LogCapture) Stop() {
	c.cancel()
	c.wg.Wait()
}

// captureLoop is the main loop that reads from /dev/kmsg
func (c *LogCapture) captureLoop() {
	defer c.wg.Done()

	// Open kernel log device
	file, err := os.Open(KernelLogDevice)
	if err != nil {
		// If we can't open /dev/kmsg, we can't capture logs
		// This is expected in non-privileged environments or tests
		return
	}
	defer file.Close()

	// Seek to end to only capture new logs
	file.Seek(0, 2)

	scanner := bufio.NewScanner(file)
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			if !scanner.Scan() {
				// Check for errors
				if err := scanner.Err(); err != nil {
					// Log error and retry
					time.Sleep(100 * time.Millisecond)
					continue
				}
				// EOF or closed, stop
				return
			}

			line := scanner.Text()
			c.processLine(line)
		}
	}
}

// processLine processes a single kernel log line
func (c *LogCapture) processLine(line string) {
	// Parse the log line
	entry, err := ParseKernelLogLine(line)
	if err != nil {
		// Invalid firewall log format, skip
		return
	}
	if entry == nil {
		// Not a firewall log, skip
		return
	}

	// Check if we should log this entry based on action type
	if !ShouldLogEntry(entry, c.config) {
		return
	}

	// Apply sampling
	c.samplingCounter++
	if !ApplySampling(c.samplingCounter, c.config.SamplingRate) {
		return
	}

	// Apply rate limiting
	if !c.rateLimiter.Allow() {
		return
	}

	// Insert into database
	if err := c.db.InsertLog(entry); err != nil {
		// Failed to insert, but don't stop processing
		return
	}
}

// PerformMaintenance runs database maintenance tasks
func (c *LogCapture) PerformMaintenance() error {
	// Cleanup old logs based on retention policy
	if c.config.RetentionDays > 0 {
		if _, err := c.db.CleanupOldLogs(c.config.RetentionDays); err != nil {
			return fmt.Errorf("failed to cleanup old logs: %w", err)
		}
	}

	// Enforce max entries limit
	if c.config.MaxLogEntries > 0 {
		if _, err := c.db.EnforceMaxEntries(c.config.MaxLogEntries); err != nil {
			return fmt.Errorf("failed to enforce max entries: %w", err)
		}
	}

	// Vacuum database to reclaim space
	if err := c.db.Vacuum(); err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}

	return nil
}
