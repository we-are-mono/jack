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

// Package cmd implements the CLI commands for Jack using cobra.
package cmd

import (
	"github.com/we-are-mono/jack/client"
	"github.com/we-are-mono/jack/daemon"
)

// ClientInterface defines the interface for communicating with the Jack daemon.
// This interface allows for easy testing by enabling mock implementations.
type ClientInterface interface {
	Send(req daemon.Request) (*daemon.Response, error)
}

// realClient wraps the actual client.Send function to implement ClientInterface.
type realClient struct{}

func (r *realClient) Send(req daemon.Request) (*daemon.Response, error) {
	return client.Send(req)
}

// defaultClient is the default client used by CLI commands.
// Tests can replace this with a mock implementation.
var defaultClient ClientInterface = &realClient{}
