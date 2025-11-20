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
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	nflog "github.com/florianl/go-nflog/v2"
)

const (
	// NFLOG groups used by nftables firewall
	NFLOGGroupDrops   = 100 // Group for DROP/REJECT packets
	NFLOGGroupAccepts = 101 // Group for ACCEPT packets
)

// DatabaseOperations defines the interface for database operations
type DatabaseOperations interface {
	InsertLog(ctx context.Context, entry *FirewallLogEntry) error
	CleanupOldLogs(ctx context.Context, retentionDays int) (int64, error)
	EnforceMaxEntries(ctx context.Context, maxEntries int) (int64, error)
	Vacuum(ctx context.Context) error
}

// LogCapture handles capturing and processing NFLOG firewall logs
type LogCapture struct {
	db              DatabaseOperations
	config          *FirewallLoggingConfig
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	samplingCounter int
	rateLimiter     *RateLimiter
	nflogDrops      *nflog.Nflog // NFLOG handle for drops group
	nflogAccepts    *nflog.Nflog // NFLOG handle for accepts group
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

// Start begins capturing NFLOG packets
func (c *LogCapture) Start() error {
	if !c.config.Enabled {
		return nil // Not enabled, don't start
	}

	// Set up NFLOG listener for drops if configured
	if c.config.LogDrops {
		nflogDrops, err := nflog.Open(&nflog.Config{
			Group:    NFLOGGroupDrops,
			Copymode: nflog.CopyPacket,
		})
		if err != nil {
			return fmt.Errorf("failed to open NFLOG group %d: %w", NFLOGGroupDrops, err)
		}
		c.nflogDrops = nflogDrops

		hookFnDrops := func(attrs nflog.Attribute) int {
			c.processNFLOGPacket(attrs, "DROP")
			return 0
		}

		if err := c.nflogDrops.RegisterWithErrorFunc(c.ctx, hookFnDrops, func(err error) int {
			if err != nil {
				log.Printf("[FIREWALL] NFLOG drops error: %v\n", err)
			}
			return 0
		}); err != nil {
			c.nflogDrops.Close()
			return fmt.Errorf("failed to register NFLOG drops callback: %w", err)
		}
	}

	// Set up NFLOG listener for accepts if configured
	if c.config.LogAccepts {
		nflogAccepts, err := nflog.Open(&nflog.Config{
			Group:    NFLOGGroupAccepts,
			Copymode: nflog.CopyPacket,
		})
		if err != nil {
			if c.nflogDrops != nil {
				c.nflogDrops.Close()
			}
			return fmt.Errorf("failed to open NFLOG group %d: %w", NFLOGGroupAccepts, err)
		}
		c.nflogAccepts = nflogAccepts

		hookFnAccepts := func(attrs nflog.Attribute) int {
			c.processNFLOGPacket(attrs, "ACCEPT")
			return 0
		}

		if err := c.nflogAccepts.RegisterWithErrorFunc(c.ctx, hookFnAccepts, func(err error) int {
			if err != nil {
				log.Printf("[FIREWALL] NFLOG accepts error: %v\n", err)
			}
			return 0
		}); err != nil {
			c.nflogAccepts.Close()
			if c.nflogDrops != nil {
				c.nflogDrops.Close()
			}
			return fmt.Errorf("failed to register NFLOG accepts callback: %w", err)
		}
	}

	return nil
}

// Stop stops the log capture and closes NFLOG handles
func (c *LogCapture) Stop() {
	c.cancel()

	// Close NFLOG handles
	if c.nflogDrops != nil {
		c.nflogDrops.Close()
	}
	if c.nflogAccepts != nil {
		c.nflogAccepts.Close()
	}

	c.wg.Wait()
}

// processNFLOGPacket processes a single NFLOG packet
func (c *LogCapture) processNFLOGPacket(attrs nflog.Attribute, action string) {
	// Extract packet payload
	if attrs.Payload == nil || len(*attrs.Payload) == 0 {
		return
	}

	// Parse IP packet from payload
	entry, err := ParseIPPacket(*attrs.Payload, action)
	if err != nil {
		// Failed to parse packet, skip
		return
	}
	if entry == nil {
		// Not a valid packet
		return
	}

	// Extract interface information from NFLOG attributes
	if attrs.InDev != nil {
		entry.InterfaceIn = fmt.Sprintf("if%d", *attrs.InDev)
	}
	if attrs.OutDev != nil {
		entry.InterfaceOut = fmt.Sprintf("if%d", *attrs.OutDev)
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
	if err := c.db.InsertLog(context.Background(), entry); err != nil {
		// Failed to insert, but don't stop processing
		return
	}
}

// PerformMaintenance runs database maintenance tasks
func (c *LogCapture) PerformMaintenance() error {
	ctx := context.Background()

	// Cleanup old logs based on retention policy
	if c.config.RetentionDays > 0 {
		if _, err := c.db.CleanupOldLogs(ctx, c.config.RetentionDays); err != nil {
			return fmt.Errorf("failed to cleanup old logs: %w", err)
		}
	}

	// Enforce max entries limit
	if c.config.MaxLogEntries > 0 {
		if _, err := c.db.EnforceMaxEntries(ctx, c.config.MaxLogEntries); err != nil {
			return fmt.Errorf("failed to enforce max entries: %w", err)
		}
	}

	// Vacuum database to reclaim space
	if err := c.db.Vacuum(ctx); err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}

	return nil
}
