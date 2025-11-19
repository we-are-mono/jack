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

package daemon

import (
	"context"
	"net"
	"strings"
	"sync"

	"github.com/we-are-mono/jack/daemon/logger"
	"github.com/we-are-mono/jack/plugins"
)

// PluginLogSubscriber wraps a plugin RPC client and implements logger.Subscriber
type PluginLogSubscriber struct {
	plugin plugins.Provider
}

// NewPluginLogSubscriber creates a subscriber that forwards log events to a plugin
func NewPluginLogSubscriber(plugin plugins.Provider, name string) *PluginLogSubscriber {
	return &PluginLogSubscriber{
		plugin: plugin,
	}
}

// OnLogEvent converts a log entry to JSON and sends it to the plugin via RPC
func (s *PluginLogSubscriber) OnLogEvent(entry *logger.Entry) error {
	// Serialize entry to JSON
	logEventJSON, err := entry.ToJSON()
	if err != nil {
		return err
	}

	// Call plugin's OnLogEvent method via RPC
	// Use a background context since this is async
	err = s.plugin.OnLogEvent(context.Background(), logEventJSON)

	// Silently ignore "not implemented" errors - not all plugins handle log events
	if err != nil && err.Error() == "plugin does not implement log event handling" {
		return nil
	}

	return err
}

// SocketLogSubscriber writes log events to a Unix socket connection
type SocketLogSubscriber struct {
	conn   net.Conn
	filter *LogFilter
	mu     sync.Mutex
	closed bool
}

// NewSocketLogSubscriber creates a subscriber that streams logs to a client socket
func NewSocketLogSubscriber(conn net.Conn, filter *LogFilter) *SocketLogSubscriber {
	return &SocketLogSubscriber{
		conn:   conn,
		filter: filter,
	}
}

// OnLogEvent writes the log entry to the socket if it matches the filter
func (s *SocketLogSubscriber) OnLogEvent(entry *logger.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If connection is closed, skip
	if s.closed {
		return nil
	}

	// Apply level filter
	if s.filter != nil && s.filter.Level != "" {
		if !strings.EqualFold(entry.Level, s.filter.Level) {
			return nil
		}
	}

	// Apply component filter
	if s.filter != nil && s.filter.Component != "" {
		if entry.Component != s.filter.Component {
			return nil
		}
	}

	// Serialize entry to JSON and write to socket
	logEventJSON, err := entry.ToJSON()
	if err != nil {
		return err
	}

	// Write JSON line to socket
	if _, err := s.conn.Write(append(logEventJSON, '\n')); err != nil {
		s.closed = true
		return err
	}

	return nil
}

// Close marks the subscriber as closed
func (s *SocketLogSubscriber) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
}
