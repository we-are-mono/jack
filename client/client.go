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

// Package client provides a client library for communicating with the Jack daemon.
package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/we-are-mono/jack/daemon"
)

// GetSocketPath returns the socket path, preferring JACK_SOCKET_PATH env var
func GetSocketPath() string {
	if path := os.Getenv("JACK_SOCKET_PATH"); path != "" {
		return path
	}
	return "/var/run/jack.sock"
}

func Send(req daemon.Request) (*daemon.Response, error) {
	conn, err := net.Dial("unix", GetSocketPath())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon (is it running?): %w", err)
	}
	defer conn.Close()

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	data = append(data, '\n')
	if _, err = conn.Write(data); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	reader := bufio.NewReader(conn)
	respData, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var resp daemon.Response
	if err := json.Unmarshal(respData, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &resp, nil
}

// StreamLogs streams logs from the daemon and calls the callback for each entry
// The connection stays open until an error occurs or the context is canceled
func StreamLogs(filter *daemon.LogFilter, callback func([]byte) error) error {
	conn, err := net.Dial("unix", GetSocketPath())
	if err != nil {
		return fmt.Errorf("failed to connect to daemon (is it running?): %w", err)
	}
	defer conn.Close()

	// Send logs-subscribe request
	req := daemon.Request{
		Command:   "logs-subscribe",
		LogFilter: filter,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	data = append(data, '\n')
	if _, err = conn.Write(data); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	// Read log entries continuously until connection closes
	reader := bufio.NewReader(conn)
	for {
		logData, err := reader.ReadBytes('\n')
		if err != nil {
			// Connection closed or error occurred
			return err
		}

		// Call callback with log entry JSON
		if err := callback(logData); err != nil {
			return err
		}
	}
}
