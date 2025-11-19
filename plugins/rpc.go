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
	"fmt"
	"log"
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

	// GetProvidedServices returns the list of services this plugin provides
	// Returns empty list if plugin doesn't provide any services
	GetProvidedServices(ctx context.Context) ([]ServiceDescriptor, error)

	// CallService calls a method on a service provided by this plugin
	// serviceName: name of the service (e.g., "database")
	// method: method name (e.g., "InsertLog")
	// argsJSON: JSON-encoded method arguments
	// Returns JSON-encoded result or error
	CallService(ctx context.Context, serviceName string, method string, argsJSON []byte) ([]byte, error)

	// SetDaemonService sets the daemon service interface for bidirectional communication
	// This allows plugins to call services on other plugins through the daemon
	SetDaemonService(daemon DaemonService)
}

// DaemonService is the interface that the daemon provides to plugins for bidirectional RPC.
// Plugins can use this to call services on other plugins through the daemon's service registry.
type DaemonService interface {
	// Ping verifies the daemon service connection is responsive
	// Used to ensure RPC connection is ready before using CallService
	Ping(ctx context.Context) error

	// CallService calls a service method on another plugin through the daemon's service registry
	// serviceName: name of the service (e.g., "database")
	// method: method name (e.g., "InsertLog")
	// argsJSON: JSON-encoded method arguments
	// Returns JSON-encoded result or error
	CallService(ctx context.Context, serviceName string, method string, argsJSON []byte) ([]byte, error)
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

// ServiceDescriptor describes a service provided by a plugin
type ServiceDescriptor struct {
	Name        string   `json:"name"`        // Service name (e.g., "database")
	Description string   `json:"description"` // Human-readable description
	Methods     []string `json:"methods"`     // Available methods (e.g., ["InsertLog", "QueryLogs"])
}

// MetadataResponse contains plugin metadata
type MetadataResponse struct {
	Namespace        string                 `json:"namespace"`
	Version          string                 `json:"version"`
	Description      string                 `json:"description"`
	Category         string                 `json:"category,omitempty"`          // Plugin category (e.g., "firewall", "vpn", "dhcp", "hardware", "monitoring")
	ConfigPath       string                 `json:"config_path"`
	DefaultConfig    map[string]interface{} `json:"default_config,omitempty"`    // Default configuration for the plugin
	Dependencies     []string               `json:"dependencies,omitempty"`      // Plugin dependencies (plugin names)
	RequiredServices []string               `json:"required_services,omitempty"` // Required services (service names like "database")
	ProvidedServices []ServiceDescriptor    `json:"provided_services,omitempty"` // Services provided by this plugin
	PathPrefix       string                 `json:"path_prefix,omitempty"`       // Prefix to auto-insert in paths (e.g., "leds")
	CLICommands      []CLICommand           `json:"cli_commands,omitempty"`      // CLI commands provided by this plugin
}

// RPCPlugin is the go-plugin Plugin implementation
type RPCPlugin struct {
	plugin.Plugin
	Impl Provider
}

// Server returns the RPC server for this plugin
func (p *RPCPlugin) Server(broker *plugin.MuxBroker) (interface{}, error) {
	return &RPCServer{
		Impl:   p.Impl,
		broker: broker,
	}, nil
}

// Client returns the RPC client for this plugin
func (p *RPCPlugin) Client(broker *plugin.MuxBroker, client *rpc.Client) (interface{}, error) {
	return &RPCClient{
		client: client,
		broker: broker,
	}, nil
}

// ============================================================================
// RPC Server Implementation
// ============================================================================

// RPCServer is the RPC server that wraps Provider
type RPCServer struct {
	Impl                   Provider
	broker                 *plugin.MuxBroker
	daemonServiceReadyChan chan struct{}
	daemonServiceSetupErr  error
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

type GetProvidedServicesArgs struct{}
type GetProvidedServicesReply struct {
	Error    string
	Services []ServiceDescriptor
}

func (s *RPCServer) GetProvidedServices(args *GetProvidedServicesArgs, reply *GetProvidedServicesReply) error {
	services, err := s.Impl.GetProvidedServices(context.Background())
	if err != nil {
		reply.Error = err.Error()
		return nil
	}
	reply.Services = services
	return nil
}

type CallServiceArgs struct {
	ServiceName string
	Method      string
	ArgsJSON    []byte
}
type CallServiceReply struct {
	Error      string
	ResultJSON []byte
}

func (s *RPCServer) CallService(args *CallServiceArgs, reply *CallServiceReply) error {
	resultJSON, err := s.Impl.CallService(context.Background(), args.ServiceName, args.Method, args.ArgsJSON)
	if err != nil {
		reply.Error = err.Error()
		return nil
	}
	reply.ResultJSON = resultJSON
	return nil
}

type SetDaemonServiceArgs struct {
	DaemonServiceID uint32 // MuxBroker ID for the daemon service
}
type SetDaemonServiceReply struct {
	Error string
}

func (s *RPCServer) SetDaemonService(args *SetDaemonServiceArgs, reply *SetDaemonServiceReply) error {
	// Initialize two channels for two-stage synchronization:
	// 1. acceptReadyChan - signals Accept() has been called
	// 2. daemonServiceReadyChan - signals SetDaemonService on impl has been called
	acceptReadyChan := make(chan struct{})
	s.daemonServiceReadyChan = make(chan struct{})
	s.daemonServiceSetupErr = nil

	// Accept connection from daemon service via MuxBroker asynchronously
	// We need to return from this RPC call so the daemon can Dial to us
	go func() {
		// Ensure the ready channel is always closed, even on error
		defer close(s.daemonServiceReadyChan)

		// Signal that Accept() is about to be called
		// This allows the RPC to return and Dial() to proceed
		close(acceptReadyChan)

		conn, err := s.broker.Accept(args.DaemonServiceID)
		if err != nil {
			s.daemonServiceSetupErr = fmt.Errorf("failed to accept daemon service connection: %w", err)
			log.Printf("[PLUGIN] %v", s.daemonServiceSetupErr)
			return
		}

		// Create RPC client for daemon service
		daemonClient := rpc.NewClient(conn)
		daemonService := &DaemonServiceClient{client: daemonClient}

		// Pass it to the plugin implementation
		s.Impl.SetDaemonService(daemonService)
	}()

	// Wait for Accept() to be called before returning
	// This ensures Dial() won't be called before Accept() is ready
	<-acceptReadyChan

	reply.Error = ""
	return nil
}

type VerifyDaemonServiceArgs struct{}
type VerifyDaemonServiceReply struct {
	Error string
}

func (s *RPCServer) VerifyDaemonService(args *VerifyDaemonServiceArgs, reply *VerifyDaemonServiceReply) error {
	// Block until the daemon service setup is complete
	// The channel is closed by SetDaemonService's goroutine after setup completes
	<-s.daemonServiceReadyChan

	// Check if setup failed
	if s.daemonServiceSetupErr != nil {
		reply.Error = s.daemonServiceSetupErr.Error()
		return nil
	}

	reply.Error = ""
	return nil
}

// ============================================================================
// RPC Client Implementation
// ============================================================================

// RPCClient is the RPC client that implements Provider
type RPCClient struct {
	client *rpc.Client
	broker *plugin.MuxBroker
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

func (c *RPCClient) GetProvidedServices(ctx context.Context) ([]ServiceDescriptor, error) {
	var reply GetProvidedServicesReply
	err := c.client.Call("Plugin.GetProvidedServices", &GetProvidedServicesArgs{}, &reply)
	if err != nil {
		return nil, err
	}
	if reply.Error != "" {
		return nil, ErrFromString(reply.Error)
	}
	return reply.Services, nil
}

func (c *RPCClient) CallService(ctx context.Context, serviceName string, method string, argsJSON []byte) ([]byte, error) {
	var reply CallServiceReply
	err := c.client.Call("Plugin.CallService", &CallServiceArgs{
		ServiceName: serviceName,
		Method:      method,
		ArgsJSON:    argsJSON,
	}, &reply)
	if err != nil {
		return nil, err
	}
	if reply.Error != "" {
		return nil, ErrFromString(reply.Error)
	}
	return reply.ResultJSON, nil
}

func (c *RPCClient) SetDaemonService(daemon DaemonService) {
	// This is called by the daemon on the client side
	// Not actually used in this direction, but required by interface
}

// ============================================================================
// DaemonService RPC Implementation
// ============================================================================

// DaemonServiceServer is the RPC server that wraps DaemonService
type DaemonServiceServer struct {
	Impl DaemonService
}

type DaemonCallServiceArgs struct {
	ServiceName string
	Method      string
	ArgsJSON    []byte
}
type DaemonCallServiceReply struct {
	Error      string
	ResultJSON []byte
}

func (s *DaemonServiceServer) CallService(args *DaemonCallServiceArgs, reply *DaemonCallServiceReply) error {
	resultJSON, err := s.Impl.CallService(context.Background(), args.ServiceName, args.Method, args.ArgsJSON)
	if err != nil {
		reply.Error = err.Error()
		return nil
	}
	reply.ResultJSON = resultJSON
	return nil
}

type DaemonPingArgs struct{}
type DaemonPingReply struct {
	Error string
}

func (s *DaemonServiceServer) Ping(args *DaemonPingArgs, reply *DaemonPingReply) error {
	err := s.Impl.Ping(context.Background())
	if err != nil {
		reply.Error = err.Error()
		return nil
	}
	return nil
}

// DaemonServiceClient is the RPC client that implements DaemonService
type DaemonServiceClient struct {
	client *rpc.Client
}

func (c *DaemonServiceClient) CallService(ctx context.Context, serviceName string, method string, argsJSON []byte) ([]byte, error) {
	var reply DaemonCallServiceReply
	err := c.client.Call("DaemonService.CallService", &DaemonCallServiceArgs{
		ServiceName: serviceName,
		Method:      method,
		ArgsJSON:    argsJSON,
	}, &reply)
	if err != nil {
		return nil, err
	}
	if reply.Error != "" {
		return nil, ErrFromString(reply.Error)
	}
	return reply.ResultJSON, nil
}

func (c *DaemonServiceClient) Ping(ctx context.Context) error {
	var reply DaemonPingReply
	err := c.client.Call("DaemonService.Ping", &DaemonPingArgs{}, &reply)
	if err != nil {
		return err
	}
	if reply.Error != "" {
		return ErrFromString(reply.Error)
	}
	return nil
}

// ============================================================================
// Helper Functions
// ============================================================================

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
