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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/we-are-mono/jack/daemon"
)

// mockClient is a mock implementation of ClientInterface for testing.
type mockClient struct {
	sendFunc func(req daemon.Request) (*daemon.Response, error)
}

func (m *mockClient) Send(req daemon.Request) (*daemon.Response, error) {
	if m.sendFunc != nil {
		return m.sendFunc(req)
	}
	return &daemon.Response{Success: true, Message: "OK"}, nil
}

func TestParseSetArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantPath  string
		wantValue interface{}
		wantError bool
	}{
		{
			name:      "simple string value",
			args:      []string{"interfaces", "br-lan", "ipaddr", "192.168.1.1"},
			wantPath:  "interfaces.br-lan.ipaddr",
			wantValue: "192.168.1.1",
		},
		{
			name:      "boolean true",
			args:      []string{"firewall", "enabled", "true"},
			wantPath:  "firewall.enabled",
			wantValue: true,
		},
		{
			name:      "boolean false",
			args:      []string{"firewall", "enabled", "false"},
			wantPath:  "firewall.enabled",
			wantValue: false,
		},
		{
			name:      "integer value",
			args:      []string{"dhcp", "dhcp_pools", "lan", "start", "100"},
			wantPath:  "dhcp.dhcp_pools.lan.start",
			wantValue: 100,
		},
		{
			name:      "negative integer",
			args:      []string{"system", "priority", "-10"},
			wantPath:  "system.priority",
			wantValue: -10,
		},
		{
			name:      "single path component",
			args:      []string{"hostname", "router"},
			wantPath:  "hostname",
			wantValue: "router",
		},
		{
			name:      "string that looks like path",
			args:      []string{"dns", "server", "8.8.8.8"},
			wantPath:  "dns.server",
			wantValue: "8.8.8.8",
		},
		{
			name:      "led brightness",
			args:      []string{"led", "status:green", "brightness", "127"},
			wantPath:  "led.status:green.brightness",
			wantValue: 127,
		},
		{
			name:      "insufficient arguments",
			args:      []string{"path"},
			wantError: true,
		},
		{
			name:      "no arguments - empty path for namespace listing",
			args:      []string{},
			wantPath:  "",
			wantValue: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, value, err := parseSetArgs(tt.args)

			if tt.wantError {
				assert.Error(t, err, "parseSetArgs() expected error, got nil")
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantPath, path)
			assert.Equal(t, tt.wantValue, value)
		})
	}
}

func TestExecuteSet(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		mockResponse   *daemon.Response
		mockError      error
		wantError      bool
		wantOutput     string
		wantErrContain string
	}{
		{
			name: "successful set",
			args: []string{"interfaces", "br-lan", "ipaddr", "192.168.1.1"},
			mockResponse: &daemon.Response{
				Success: true,
				Message: "Configuration updated",
			},
			wantOutput: "Configuration updated\n",
		},
		{
			name: "daemon error",
			args: []string{"firewall", "enabled", "true"},
			mockResponse: &daemon.Response{
				Success: false,
				Error:   "invalid configuration",
			},
			wantError:      true,
			wantErrContain: "invalid configuration",
		},
		{
			name:           "connection error",
			args:           []string{"interfaces", "br-lan", "enabled", "true"},
			mockError:      fmt.Errorf("failed to connect to daemon"),
			wantError:      true,
			wantErrContain: "failed to connect",
		},
		{
			name: "set boolean value",
			args: []string{"system", "debug", "true"},
			mockResponse: &daemon.Response{
				Success: true,
				Message: "Debug enabled",
			},
			wantOutput: "Debug enabled\n",
		},
		{
			name: "set integer value",
			args: []string{"dhcp", "lease_time", "3600"},
			mockResponse: &daemon.Response{
				Success: true,
				Message: "Lease time updated",
			},
			wantOutput: "Lease time updated\n",
		},
		{
			name:           "invalid arguments",
			args:           []string{"path"},
			wantError:      true,
			wantErrContain: "requires at least 2 arguments",
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
					assert.Equal(t, "set", req.Command)
					return tt.mockResponse, nil
				},
			}

			err := executeSet(&buf, mockCli, tt.args)

			if tt.wantError {
				require.Error(t, err, "executeSet() expected error, got nil")
				if tt.wantErrContain != "" {
					assert.Contains(t, err.Error(), tt.wantErrContain)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantOutput, buf.String())
		})
	}
}

func TestExecuteSet_RequestFields(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantPath  string
		wantValue interface{}
	}{
		{
			name:      "complex path",
			args:      []string{"interfaces", "br-lan", "dhcp", "enabled", "true"},
			wantPath:  "interfaces.br-lan.dhcp.enabled",
			wantValue: true,
		},
		{
			name:      "integer value",
			args:      []string{"vlan", "10", "mtu", "1500"},
			wantPath:  "vlan.10.mtu",
			wantValue: 1500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			var capturedReq daemon.Request

			mockCli := &mockClient{
				sendFunc: func(req daemon.Request) (*daemon.Response, error) {
					capturedReq = req
					return &daemon.Response{Success: true, Message: "OK"}, nil
				},
			}

			err := executeSet(&buf, mockCli, tt.args)
			require.NoError(t, err)

			assert.Equal(t, "set", capturedReq.Command)
			assert.Equal(t, tt.wantPath, capturedReq.Path)
			assert.Equal(t, tt.wantValue, capturedReq.Value)
		})
	}
}
