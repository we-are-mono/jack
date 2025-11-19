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
	"fmt"
	"io"
	"log"
	"net/rpc"
	"os"
	"os/exec"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
)

// PluginClient wraps a go-plugin client for lifecycle management
type PluginClient struct {
	client    *plugin.Client
	rpcClient plugin.ClientProtocol
}

// NewPluginClient creates a new plugin client and starts the plugin
func NewPluginClient(pluginPath string) (*PluginClient, error) {
	// Only show plugin startup logs in debug mode
	if os.Getenv("JACK_DEBUG") != "" {
		log.Printf("[PLUGIN] Starting plugin: %s", pluginPath)
	}

	// Create logger that suppresses debug output
	// Check JACK_DEBUG environment variable to enable debug logs
	logLevel := hclog.Error
	if os.Getenv("JACK_DEBUG") != "" {
		logLevel = hclog.Debug
	}

	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "plugin",
		Output: io.Discard, // Discard all plugin framework logs by default
		Level:  logLevel,
	})

	// Create the plugin client using generic protocol
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: Handshake,
		Plugins: map[string]plugin.Plugin{
			"generic": &RPCPlugin{},
		},
		Cmd:    exec.Command(pluginPath),
		Logger: logger,
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolNetRPC,
		},
	})

	// Connect to the plugin
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to get RPC client: %w", err)
	}

	// Only show plugin startup logs in debug mode
	if os.Getenv("JACK_DEBUG") != "" {
		log.Printf("[PLUGIN] Plugin started successfully")
	}

	return &PluginClient{
		client:    client,
		rpcClient: rpcClient,
	}, nil
}

// Dispense dispenses the plugin provider
func (c *PluginClient) Dispense() (Provider, error) {
	raw, err := c.rpcClient.Dispense("generic")
	if err != nil {
		return nil, fmt.Errorf("failed to dispense plugin: %w", err)
	}

	provider, ok := raw.(Provider)
	if !ok {
		return nil, fmt.Errorf("dispensed plugin is not a Provider")
	}

	return provider, nil
}

// SetupDaemonService provides a daemon service to the plugin for bidirectional RPC
// This allows the plugin to call services on other plugins through the daemon
func (c *PluginClient) SetupDaemonService(daemonService DaemonService) error {
	// Get the RPC client (which has the broker)
	raw, err := c.rpcClient.Dispense("generic")
	if err != nil {
		return fmt.Errorf("failed to get plugin provider: %w", err)
	}

	rpcClient, ok := raw.(*RPCClient)
	if !ok {
		return fmt.Errorf("provider is not an RPCClient")
	}

	// Use a fixed broker ID for daemon service
	daemonServiceID := uint32(1)

	// Call SetDaemonService on the plugin - it will start Accept in a goroutine
	var reply SetDaemonServiceReply
	err = rpcClient.client.Call("Plugin.SetDaemonService", &SetDaemonServiceArgs{
		DaemonServiceID: daemonServiceID,
	}, &reply)
	if err != nil {
		return fmt.Errorf("failed to call SetDaemonService: %w", err)
	}
	if reply.Error != "" {
		return fmt.Errorf("SetDaemonService failed: %s", reply.Error)
	}

	// Dial to the plugin's Accept - this blocks until Accept is ready
	conn, err := rpcClient.broker.Dial(daemonServiceID)
	if err != nil {
		return fmt.Errorf("failed to dial daemon service connection: %w", err)
	}

	// Start RPC server to handle daemon service calls from plugin
	// ServeConn blocks, so run it in a goroutine
	server := rpc.NewServer()
	server.RegisterName("DaemonService", &DaemonServiceServer{Impl: daemonService})
	go server.ServeConn(conn)

	// Verify the daemon service was successfully set on the plugin
	// by making a test call that requires the daemon service to be present
	// This blocks until the plugin's Accept goroutine completes and sets the daemon service
	var verifyReply VerifyDaemonServiceReply
	if err := rpcClient.client.Call("Plugin.VerifyDaemonService", &VerifyDaemonServiceArgs{}, &verifyReply); err != nil {
		return fmt.Errorf("failed to verify daemon service setup: %w", err)
	}
	if verifyReply.Error != "" {
		return fmt.Errorf("daemon service verification failed: %s", verifyReply.Error)
	}

	return nil
}

// Close terminates the plugin
func (c *PluginClient) Close() error {
	if c.client != nil {
		c.client.Kill()
	}
	return nil
}
