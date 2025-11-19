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

// TestExecuteApply tests the apply command execution.
func TestExecuteApply(t *testing.T) {
	tests := []struct {
		name           string
		mockResponse   *daemon.Response
		mockError      error
		wantError      bool
		wantOutput     string
		wantErrContain string
	}{
		{
			name: "successful apply",
			mockResponse: &daemon.Response{
				Success: true,
				Message: "Configuration applied successfully",
			},
			wantOutput: "Applying configuration...\n[OK] Configuration applied successfully\n",
		},
		{
			name: "daemon error",
			mockResponse: &daemon.Response{
				Success: false,
				Error:   "failed to apply configuration",
			},
			wantError:      true,
			wantErrContain: "failed to apply configuration",
		},
		{
			name:           "connection error",
			mockError:      fmt.Errorf("failed to connect to daemon"),
			wantError:      true,
			wantErrContain: "failed to connect",
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
					assert.Equal(t, "apply", req.Command)
					return tt.mockResponse, nil
				},
			}

			err := executeApply(&buf, mockCli)

			if tt.wantError {
				require.Error(t, err, "executeApply() expected error, got nil")
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

// TestExecuteCommit tests the commit command execution.
func TestExecuteCommit(t *testing.T) {
	tests := []struct {
		name           string
		mockResponse   *daemon.Response
		mockError      error
		wantError      bool
		wantOutput     string
		wantErrContain string
	}{
		{
			name: "successful commit",
			mockResponse: &daemon.Response{
				Success: true,
				Message: "Changes committed",
			},
			wantOutput: "[OK] Changes committed\n",
		},
		{
			name: "daemon error",
			mockResponse: &daemon.Response{
				Success: false,
				Error:   "no pending changes to commit",
			},
			wantError:      true,
			wantErrContain: "no pending changes",
		},
		{
			name:           "connection error",
			mockError:      fmt.Errorf("daemon not running"),
			wantError:      true,
			wantErrContain: "daemon not running",
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
					assert.Equal(t, "commit", req.Command)
					return tt.mockResponse, nil
				},
			}

			err := executeCommit(&buf, mockCli)

			if tt.wantError {
				require.Error(t, err, "executeCommit() expected error, got nil")
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

// TestExecuteRevert tests the revert command execution.
func TestExecuteRevert(t *testing.T) {
	tests := []struct {
		name           string
		mockResponse   *daemon.Response
		mockError      error
		wantError      bool
		wantOutput     string
		wantErrContain string
	}{
		{
			name: "successful revert",
			mockResponse: &daemon.Response{
				Success: true,
				Message: "Pending changes discarded",
			},
			wantOutput: "[OK] Pending changes discarded\n",
		},
		{
			name: "daemon error",
			mockResponse: &daemon.Response{
				Success: false,
				Error:   "no pending changes to revert",
			},
			wantError:      true,
			wantErrContain: "no pending changes",
		},
		{
			name:           "connection error",
			mockError:      fmt.Errorf("socket connection failed"),
			wantError:      true,
			wantErrContain: "socket connection failed",
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
					assert.Equal(t, "revert", req.Command)
					return tt.mockResponse, nil
				},
			}

			err := executeRevert(&buf, mockCli)

			if tt.wantError {
				require.Error(t, err, "executeRevert() expected error, got nil")
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

// TestExecuteDiff tests the diff command execution.
func TestExecuteDiff(t *testing.T) {
	tests := []struct {
		name           string
		mockResponse   *daemon.Response
		mockError      error
		wantError      bool
		wantOutput     string
		wantErrContain string
	}{
		{
			name: "successful diff with changes",
			mockResponse: &daemon.Response{
				Success: true,
				Message: "interfaces.br-lan.ipaddr: 192.168.1.1 -> 192.168.1.2\nfirewall.enabled: false -> true",
			},
			wantOutput: "interfaces.br-lan.ipaddr: 192.168.1.1 -> 192.168.1.2\nfirewall.enabled: false -> true\n",
		},
		{
			name: "diff with no changes",
			mockResponse: &daemon.Response{
				Success: true,
				Message: "No pending changes",
			},
			wantOutput: "No pending changes\n",
		},
		{
			name: "daemon error",
			mockResponse: &daemon.Response{
				Success: false,
				Error:   "failed to compute diff",
			},
			wantError:      true,
			wantErrContain: "failed to compute diff",
		},
		{
			name:           "connection error",
			mockError:      fmt.Errorf("daemon unreachable"),
			wantError:      true,
			wantErrContain: "daemon unreachable",
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
					assert.Equal(t, "diff", req.Command)
					return tt.mockResponse, nil
				},
			}

			err := executeDiff(&buf, mockCli)

			if tt.wantError {
				require.Error(t, err, "executeDiff() expected error, got nil")
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
