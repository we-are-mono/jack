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

package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/we-are-mono/jack/daemon"
)

func TestParseGetPath(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantPath  string
		wantError bool
	}{
		{
			name:     "simple path",
			args:     []string{"interfaces"},
			wantPath: "interfaces",
		},
		{
			name:     "nested path",
			args:     []string{"interfaces", "br-lan", "ipaddr"},
			wantPath: "interfaces.br-lan.ipaddr",
		},
		{
			name:     "deep nested path",
			args:     []string{"dhcp", "dhcp_pools", "lan", "start"},
			wantPath: "dhcp.dhcp_pools.lan.start",
		},
		{
			name:     "firewall path",
			args:     []string{"firewall", "zones", "wan", "input"},
			wantPath: "firewall.zones.wan.input",
		},
		{
			name:     "led path with colon",
			args:     []string{"led", "status:green", "brightness"},
			wantPath: "led.status:green.brightness",
		},
		{
			name:     "no arguments - empty path for namespace listing",
			args:     []string{},
			wantPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := parseGetPath(tt.args)

			if tt.wantError {
				assert.Error(t, err, "parseGetPath() expected error, got nil")
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantPath, path)
		})
	}
}

func TestExecuteGet(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		mockResponse   *daemon.Response
		mockError      error
		wantError      bool
		wantOutputJSON string
		wantErrContain string
	}{
		{
			name: "successful get - string value",
			args: []string{"interfaces", "br-lan", "ipaddr"},
			mockResponse: &daemon.Response{
				Success: true,
				Data:    "192.168.1.1",
			},
			wantOutputJSON: `"192.168.1.1"`,
		},
		{
			name: "successful get - boolean value",
			args: []string{"firewall", "enabled"},
			mockResponse: &daemon.Response{
				Success: true,
				Data:    true,
			},
			wantOutputJSON: "true",
		},
		{
			name: "successful get - integer value",
			args: []string{"dhcp", "lease_time"},
			mockResponse: &daemon.Response{
				Success: true,
				Data:    3600,
			},
			wantOutputJSON: "3600",
		},
		{
			name: "successful get - object value",
			args: []string{"interfaces", "br-lan"},
			mockResponse: &daemon.Response{
				Success: true,
				Data: map[string]interface{}{
					"type":    "bridge",
					"enabled": true,
					"ipaddr":  "192.168.1.1",
				},
			},
			wantOutputJSON: `{
  "enabled": true,
  "ipaddr": "192.168.1.1",
  "type": "bridge"
}`,
		},
		{
			name: "daemon error",
			args: []string{"nonexistent", "path"},
			mockResponse: &daemon.Response{
				Success: false,
				Error:   "path not found",
			},
			wantError:      true,
			wantErrContain: "path not found",
		},
		{
			name:           "connection error",
			args:           []string{"interfaces"},
			mockError:      fmt.Errorf("failed to connect to daemon"),
			wantError:      true,
			wantErrContain: "failed to connect",
		},
		{
			name: "list namespaces - no arguments",
			args: []string{},
			mockResponse: &daemon.Response{
				Success: true,
				Data: map[string]interface{}{
					"core":       []interface{}{"interfaces", "routes"},
					"firewall":   []interface{}{"firewall"},
					"dhcp":       []interface{}{"dhcp"},
					"monitoring": []interface{}{"monitoring"},
				},
				Message: "Available configuration namespaces grouped by category",
			},
			wantOutputJSON: "Available configuration namespaces:\n\ncore:\n  interfaces\n  routes\n\nfirewall:\n  firewall\n\ndhcp:\n  dhcp\n\nmonitoring:\n  monitoring",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			mockCli := &mockClient{
				sendFunc: func(req daemon.Request) (*daemon.Response, error) {
					if tt.mockError != nil {
						return nil, tt.mockError
					}
					// Verify the request is constructed correctly
					assert.Equal(t, "get", req.Command)
					return tt.mockResponse, nil
				},
			}

			err := executeGet(&buf, mockCli, tt.args)

			if tt.wantError {
				require.Error(t, err, "executeGet() expected error, got nil")
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
				return
			}

			require.NoError(t, err)

			// Verify JSON output
			output := strings.TrimSpace(buf.String())
			assert.Equal(t, tt.wantOutputJSON, output)
		})
	}
}

func TestExecuteGet_RequestFields(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantPath string
	}{
		{
			name:     "simple path",
			args:     []string{"interfaces"},
			wantPath: "interfaces",
		},
		{
			name:     "complex nested path",
			args:     []string{"firewall", "zones", "wan", "policy"},
			wantPath: "firewall.zones.wan.policy",
		},
		{
			name:     "path with numeric component",
			args:     []string{"vlan", "10", "name"},
			wantPath: "vlan.10.name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			var capturedReq daemon.Request

			mockCli := &mockClient{
				sendFunc: func(req daemon.Request) (*daemon.Response, error) {
					capturedReq = req
					return &daemon.Response{
						Success: true,
						Data:    "test-value",
					}, nil
				},
			}

			err := executeGet(&buf, mockCli, tt.args)
			require.NoError(t, err)

			assert.Equal(t, "get", capturedReq.Command)
			assert.Equal(t, tt.wantPath, capturedReq.Path)
		})
	}
}
