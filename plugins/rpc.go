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

// Package plugins defines the plugin system for Jack using Hashicorp's go-plugin framework.
package plugins

import (
	"context"
	"net/rpc"

	"github.com/hashicorp/go-plugin"
)

// Handshake is used to verify that client and server are compatible.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "JACK_PLUGIN",
	MagicCookieValue: "generic",
}

// Provider is the interface that all plugins must implement for RPC communication.
// It uses JSON serialization for configs to be completely type-agnostic.
type Provider interface {
	// Metadata returns plugin information
	Metadata(ctx context.Context) (MetadataResponse, error)

	// ApplyConfig applies configuration (config is JSON-encoded)
	ApplyConfig(ctx context.Context, configJSON []byte) error

	// ValidateConfig validates configuration (config is JSON-encoded)
	ValidateConfig(ctx context.Context, configJSON []byte) error

	// Flush removes all configuration
	Flush(ctx context.Context) error

	// Status returns current status (response is JSON-encoded)
	Status(ctx context.Context) ([]byte, error)

	// ExecuteCLICommand executes a CLI command provided by the plugin
	ExecuteCLICommand(ctx context.Context, command string, args []string) ([]byte, error)

	// OnLogEvent receives a log event from the daemon (optional, returns error if not implemented)
	// logEventJSON is the JSON-encoded log entry
	OnLogEvent(ctx context.Context, logEventJSON []byte) error
}

// CLICommand describes a CLI command provided by a plugin
type CLICommand struct {
	Name         string   `json:"name"`                    // Command name (e.g., "monitor")
	Short        string   `json:"short"`                   // Short description
	Long         string   `json:"long"`                    // Long description
	Subcommands  []string `json:"subcommands"`             // Available subcommands (e.g., ["stats", "bandwidth"])
	Continuous   bool     `json:"continuous,omitempty"`    // If true, command outputs continuously (live polling)
	PollInterval int      `json:"poll_interval,omitempty"` // Polling interval in seconds (default: 2), only used if Continuous is true
}

// MetadataResponse contains plugin metadata
type MetadataResponse struct {
	Namespace     string                 `json:"namespace"`
	Version       string                 `json:"version"`
	Description   string                 `json:"description"`
	Category      string                 `json:"category,omitempty"`     // Plugin category (e.g., "firewall", "vpn", "dhcp", "hardware", "monitoring")
	ConfigPath    string                 `json:"config_path"`
	DefaultConfig map[string]interface{} `json:"default_config,omitempty"` // Default configuration for the plugin
	Dependencies  []string               `json:"dependencies,omitempty"`
	PathPrefix    string                 `json:"path_prefix,omitempty"`  // Prefix to auto-insert in paths (e.g., "leds")
	CLICommands   []CLICommand           `json:"cli_commands,omitempty"` // CLI commands provided by this plugin
}

// RPCPlugin is the go-plugin Plugin implementation
type RPCPlugin struct {
	plugin.Plugin
	Impl Provider
}

// Server returns the RPC server for this plugin
func (p *RPCPlugin) Server(*plugin.MuxBroker) (interface{}, error) {
	return &RPCServer{Impl: p.Impl}, nil
}

// Client returns the RPC client for this plugin
func (p *RPCPlugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &RPCClient{client: c}, nil
}

// ============================================================================
// RPC Server Implementation
// ============================================================================

// RPCServer is the RPC server that wraps Provider
type RPCServer struct {
	Impl Provider
}

type MetadataArgs struct{}
type MetadataReply struct {
	Error    string
	Metadata MetadataResponse
}

func (s *RPCServer) Metadata(args *MetadataArgs, reply *MetadataReply) error {
	metadata, err := s.Impl.Metadata(context.Background())
	if err != nil {
		reply.Error = err.Error()
		return nil
	}
	reply.Metadata = metadata
	return nil
}

type ApplyConfigArgs struct {
	ConfigJSON []byte
}
type ApplyConfigReply struct {
	Error string
}

func (s *RPCServer) ApplyConfig(args *ApplyConfigArgs, reply *ApplyConfigReply) error {
	err := s.Impl.ApplyConfig(context.Background(), args.ConfigJSON)
	if err != nil {
		reply.Error = err.Error()
	}
	return nil
}

type ValidateConfigArgs struct {
	ConfigJSON []byte
}
type ValidateConfigReply struct {
	Error string
}

func (s *RPCServer) ValidateConfig(args *ValidateConfigArgs, reply *ValidateConfigReply) error {
	err := s.Impl.ValidateConfig(context.Background(), args.ConfigJSON)
	if err != nil {
		reply.Error = err.Error()
	}
	return nil
}

type FlushArgs struct{}
type FlushReply struct {
	Error string
}

func (s *RPCServer) Flush(args *FlushArgs, reply *FlushReply) error {
	err := s.Impl.Flush(context.Background())
	if err != nil {
		reply.Error = err.Error()
	}
	return nil
}

type StatusArgs struct{}
type StatusReply struct {
	Error      string
	StatusJSON []byte
}

func (s *RPCServer) Status(args *StatusArgs, reply *StatusReply) error {
	statusJSON, err := s.Impl.Status(context.Background())
	if err != nil {
		reply.Error = err.Error()
		return nil
	}
	reply.StatusJSON = statusJSON
	return nil
}

type ExecuteCLICommandArgs struct {
	Command string
	Args    []string
}
type ExecuteCLICommandReply struct {
	Error  string
	Output []byte
}

func (s *RPCServer) ExecuteCLICommand(args *ExecuteCLICommandArgs, reply *ExecuteCLICommandReply) error {
	output, err := s.Impl.ExecuteCLICommand(context.Background(), args.Command, args.Args)
	if err != nil {
		reply.Error = err.Error()
		return nil
	}
	reply.Output = output
	return nil
}

type OnLogEventArgs struct {
	LogEventJSON []byte
}
type OnLogEventReply struct {
	Error string
}

func (s *RPCServer) OnLogEvent(args *OnLogEventArgs, reply *OnLogEventReply) error {
	err := s.Impl.OnLogEvent(context.Background(), args.LogEventJSON)
	if err != nil {
		reply.Error = err.Error()
	}
	return nil
}

// ============================================================================
// RPC Client Implementation
// ============================================================================

// RPCClient is the RPC client that implements Provider
type RPCClient struct {
	client *rpc.Client
}

func (c *RPCClient) Metadata(ctx context.Context) (MetadataResponse, error) {
	var reply MetadataReply
	err := c.client.Call("Plugin.Metadata", &MetadataArgs{}, &reply)
	if err != nil {
		return MetadataResponse{}, err
	}
	if reply.Error != "" {
		return MetadataResponse{}, ErrFromString(reply.Error)
	}
	return reply.Metadata, nil
}

func (c *RPCClient) ApplyConfig(ctx context.Context, configJSON []byte) error {
	var reply ApplyConfigReply
	err := c.client.Call("Plugin.ApplyConfig", &ApplyConfigArgs{ConfigJSON: configJSON}, &reply)
	if err != nil {
		return err
	}
	if reply.Error != "" {
		return ErrFromString(reply.Error)
	}
	return nil
}

func (c *RPCClient) ValidateConfig(ctx context.Context, configJSON []byte) error {
	var reply ValidateConfigReply
	err := c.client.Call("Plugin.ValidateConfig", &ValidateConfigArgs{ConfigJSON: configJSON}, &reply)
	if err != nil {
		return err
	}
	if reply.Error != "" {
		return ErrFromString(reply.Error)
	}
	return nil
}

func (c *RPCClient) Flush(ctx context.Context) error {
	var reply FlushReply
	err := c.client.Call("Plugin.Flush", &FlushArgs{}, &reply)
	if err != nil {
		return err
	}
	if reply.Error != "" {
		return ErrFromString(reply.Error)
	}
	return nil
}

func (c *RPCClient) Status(ctx context.Context) ([]byte, error) {
	var reply StatusReply
	err := c.client.Call("Plugin.Status", &StatusArgs{}, &reply)
	if err != nil {
		return nil, err
	}
	if reply.Error != "" {
		return nil, ErrFromString(reply.Error)
	}
	return reply.StatusJSON, nil
}

func (c *RPCClient) ExecuteCLICommand(ctx context.Context, command string, args []string) ([]byte, error) {
	var reply ExecuteCLICommandReply
	err := c.client.Call("Plugin.ExecuteCLICommand", &ExecuteCLICommandArgs{
		Command: command,
		Args:    args,
	}, &reply)
	if err != nil {
		return nil, err
	}
	if reply.Error != "" {
		return nil, ErrFromString(reply.Error)
	}
	return reply.Output, nil
}

func (c *RPCClient) OnLogEvent(ctx context.Context, logEventJSON []byte) error {
	var reply OnLogEventReply
	err := c.client.Call("Plugin.OnLogEvent", &OnLogEventArgs{
		LogEventJSON: logEventJSON,
	}, &reply)
	if err != nil {
		return err
	}
	if reply.Error != "" {
		return ErrFromString(reply.Error)
	}
	return nil
}

// ErrFromString creates an error from a string
func ErrFromString(s string) error {
	if s == "" {
		return nil
	}
	return &rpcError{msg: s}
}

type rpcError struct {
	msg string
}

func (e *rpcError) Error() string {
	return e.msg
}

// ServePlugin is a helper to serve a plugin using the generic protocol
func ServePlugin(impl Provider) {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]plugin.Plugin{
			"generic": &RPCPlugin{Impl: impl},
		},
	})
}
