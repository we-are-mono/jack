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
	"sync"
)

// Subscriber is the interface for log event subscribers (plugins)
type Subscriber interface {
	OnLogEvent(entry *Entry) error
}

// Emitter emits log events to subscribed plugins
type Emitter struct {
	subscribers []Subscriber
	mu          sync.RWMutex
}

// NewEmitter creates a new log event emitter
func NewEmitter() *Emitter {
	return &Emitter{
		subscribers: make([]Subscriber, 0),
	}
}

// Subscribe adds a subscriber to receive log events
func (e *Emitter) Subscribe(sub Subscriber) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.subscribers = append(e.subscribers, sub)
}

// Unsubscribe removes a subscriber from receiving log events
func (e *Emitter) Unsubscribe(sub Subscriber) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i, s := range e.subscribers {
		if s == sub {
			e.subscribers = append(e.subscribers[:i], e.subscribers[i+1:]...)
			return
		}
	}
}

// Emit sends a log entry to all subscribers
// Errors from subscribers are ignored to prevent log failures from affecting subscribers
func (e *Emitter) Emit(entry *Entry) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, sub := range e.subscribers {
		// Run in goroutine to prevent slow subscribers from blocking logging
		go func(s Subscriber) {
			_ = s.OnLogEvent(entry) // Ignore errors
		}(sub)
	}
}
