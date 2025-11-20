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

// Package daemon implements the Jack daemon server and IPC protocol.
package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/we-are-mono/jack/daemon/logger"
	"github.com/we-are-mono/jack/plugins"
	"github.com/we-are-mono/jack/state"
	"github.com/we-are-mono/jack/system"
	"github.com/we-are-mono/jack/types"
	"github.com/we-are-mono/jack/validation"
)

// GetSocketPath returns the socket path, preferring JACK_SOCKET_PATH env var
func GetSocketPath() string {
	if path := os.Getenv("JACK_SOCKET_PATH"); path != "" {
		return path
	}
	return "/var/run/jack.sock"
}

// handlerFunc is a function that handles a daemon command
type handlerFunc func(Request) Response

type Server struct {
	state           *State
	listener        net.Listener
	done            chan struct{}
	handlers        map[string]handlerFunc
	registry        *PluginRegistry
	serviceRegistry *ServiceRegistry
	networkObserver *NetworkObserver
}

func NewServer() (*Server, error) {
	socketPath := GetSocketPath()
	os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create socket: %w", err)
	}

	if err := os.Chmod(socketPath, 0666); err != nil {
		return nil, fmt.Errorf("failed to set socket permissions: %w", err)
	}

	s := &Server{
		state:           NewState(),
		listener:        listener,
		done:            make(chan struct{}),
		registry:        NewPluginRegistry(),
		serviceRegistry: NewServiceRegistry(),
	}

	// Initialize command handlers
	s.handlers = map[string]handlerFunc{
		"status":           func(req Request) Response { return s.handleStatus() },
		"info":             func(req Request) Response { return s.handleInfo() },
		"diff":             func(req Request) Response { return s.handleDiff() },
		"commit":           func(req Request) Response { return s.handleCommit() },
		"revert":           func(req Request) Response { return s.handleRevert() },
		"apply":            func(req Request) Response { return s.handleApply() },
		"show":             func(req Request) Response { return s.handleShow(req.Path) },
		"get":              func(req Request) Response { return s.handleGet(req.Path) },
		"set":              func(req Request) Response { return s.handleSet(req.Path, req.Value) },
		"validate":         func(req Request) Response { return s.handleValidate(req.Path, req.Value) },
		"plugin-enable":    func(req Request) Response { return s.handlePluginEnable(req.Plugin) },
		"plugin-disable":   func(req Request) Response { return s.handlePluginDisable(req.Plugin) },
		"plugin-rescan":    func(req Request) Response { return s.handlePluginRescan() },
		"plugin-cli":       func(req Request) Response { return s.handlePluginCLI(req.Plugin, req.CLICommand, req.CLIArgs) },
		"rollback":         func(req Request) Response { return s.handleRollback(req.CheckpointID) },
		"checkpoint-list":  func(req Request) Response { return s.handleCheckpointList() },
		"checkpoint-create": func(req Request) Response { return s.handleCheckpointCreate() },
	}

	return s, nil
}

// orderPluginsByDependencies returns a list of plugin names sorted by dependencies
// Plugins with no dependencies come first, then plugins that depend on them, etc.
func orderPluginsByDependencies(pluginsConfig map[string]types.PluginState) []string {
	// Build list of enabled plugins
	var enabledPlugins []string
	pluginDeps := make(map[string][]string)

	for name, pluginState := range pluginsConfig {
		if !pluginState.Enabled {
			continue
		}
		enabledPlugins = append(enabledPlugins, name)

		// Load plugin temporarily to get dependencies from metadata
		plugin, err := LoadPlugin(name)
		if err != nil {
			// If we can't load it, add it without dependencies
			pluginDeps[name] = nil
			continue
		}

		metadata := plugin.Metadata()
		pluginDeps[name] = metadata.Dependencies
		plugin.Close()
	}

	// Topological sort
	var sorted []string
	visited := make(map[string]bool)
	temp := make(map[string]bool)

	var visit func(string) bool
	visit = func(name string) bool {
		if temp[name] {
			// Circular dependency detected, but continue anyway
			return false
		}
		if visited[name] {
			return true
		}

		temp[name] = true
		deps := pluginDeps[name]
		for _, dep := range deps {
			// Only visit if the dependency is enabled
			depEnabled := false
			for _, enabled := range enabledPlugins {
				if enabled == dep {
					depEnabled = true
					break
				}
			}
			if depEnabled {
				visit(dep)
			}
		}
		temp[name] = false
		visited[name] = true
		sorted = append(sorted, name)
		return true
	}

	for _, name := range enabledPlugins {
		if !visited[name] {
			visit(name)
		}
	}

	return sorted
}

// loadPlugins discovers and loads all configured plugins.
// Plugins self-register by declaring their namespace via metadata.
// Only loads plugins that are marked as enabled.
// Plugins are loaded in waves based on service dependencies:
// - Wave 1: Plugins with no service dependencies
// - Wave 2: Plugins whose required services are now ready
// - Wave N: Continue until all plugins loaded
func (s *Server) loadPlugins() error {
	// Load jack config to get plugin list
	jackConfig, err := state.LoadJackConfig()
	if err != nil {
		return fmt.Errorf("failed to load Jack config: %w", err)
	}

	// Get list of enabled plugins
	var enabledPlugins []string
	for name, pluginState := range jackConfig.Plugins {
		if pluginState.Enabled {
			enabledPlugins = append(enabledPlugins, name)
		}
	}

	logger.Info("Loading plugins in waves based on service readiness",
		logger.Field{Key: "enabled", Value: fmt.Sprintf("%v", enabledPlugins)})

	// Track which plugins have been loaded
	loadedPlugins := make(map[string]bool)

	// Load plugins in waves until all are loaded
	wave := 1
	for len(loadedPlugins) < len(enabledPlugins) {
		logger.Info("Loading plugin wave",
			logger.Field{Key: "wave", Value: wave})

		pluginsLoadedThisWave := 0

		// Try to load each remaining plugin
		for _, name := range enabledPlugins {
			if loadedPlugins[name] {
				continue // Already loaded
			}

			// Check if this plugin can be loaded now
			canLoad, requiredServices := s.canLoadPlugin(name, loadedPlugins)
			if !canLoad {
				logger.Info("Plugin not ready - waiting for services",
					logger.Field{Key: "plugin", Value: name},
					logger.Field{Key: "required_services", Value: fmt.Sprintf("%v", requiredServices)})
				continue
			}

			// Load the plugin
			if err := s.loadSinglePlugin(name, jackConfig); err != nil {
				logger.Warn("Failed to load plugin",
					logger.Field{Key: "plugin", Value: name},
					logger.Field{Key: "error", Value: err.Error()})
				// Mark as loaded even if failed to avoid infinite loop
				loadedPlugins[name] = true
				continue
			}

			loadedPlugins[name] = true
			pluginsLoadedThisWave++
		}

		if pluginsLoadedThisWave == 0 {
			// No plugins loaded this wave - check for unmet dependencies
			var remaining []string
			for _, name := range enabledPlugins {
				if !loadedPlugins[name] {
					remaining = append(remaining, name)
				}
			}
			if len(remaining) > 0 {
				logger.Warn("Unable to load remaining plugins - unmet dependencies",
					logger.Field{Key: "plugins", Value: fmt.Sprintf("%v", remaining)})
				break
			}
		}

		wave++
	}

	// Save updated config with versions
	if err := state.SaveJackConfig(jackConfig); err != nil {
		logger.Warn("Failed to save updated plugin versions",
			logger.Field{Key: "error", Value: err.Error()})
	}

	// Store jack config in state for access by observer
	s.state.LoadCommittedJackConfig(jackConfig)

	logger.Info("Loaded plugins",
		logger.Field{Key: "plugins", Value: s.registry.List()})
	return nil
}

// canLoadPlugin checks if a plugin can be loaded now based on service dependencies
// Returns true if all required services are ready
func (s *Server) canLoadPlugin(name string, loadedPlugins map[string]bool) (bool, []string) {
	// Temporarily load plugin to check its service requirements
	plugin, err := LoadPlugin(name)
	if err != nil {
		return false, nil
	}
	defer plugin.Close()

	loader, ok := plugin.(*PluginLoader)
	if !ok {
		// Plugin doesn't provide service info - load it immediately
		return true, nil
	}

	requiredServices := loader.GetRequiredServices()
	if len(requiredServices) == 0 {
		// No service dependencies - can load immediately
		return true, nil
	}

	// Check if all required services are ready
	if s.serviceRegistry.AreServicesReady(requiredServices) {
		return true, requiredServices
	}

	return false, requiredServices
}

// loadSinglePlugin loads and initializes a single plugin
func (s *Server) loadSinglePlugin(name string, jackConfig *types.JackConfig) error {
	pluginState := jackConfig.Plugins[name]

	plugin, err := LoadPlugin(name)
	if err != nil {
		return fmt.Errorf("failed to load plugin: %w", err)
	}

	// Update version in config
	metadata := plugin.Metadata()
	pluginState.Version = metadata.Version
	jackConfig.Plugins[name] = pluginState

	if err := s.registry.Register(plugin, name); err != nil {
		plugin.Close()
		return fmt.Errorf("failed to register plugin: %w", err)
	}

	// Register plugin services with service registry
	if loader, ok := plugin.(*PluginLoader); ok {
		providedServices := loader.GetProvidedServices()
		if err := s.serviceRegistry.RegisterPlugin(metadata.Namespace, plugin, providedServices); err != nil {
			s.registry.Unregister(metadata.Namespace)
			plugin.Close()
			return fmt.Errorf("failed to register plugin services: %w", err)
		}

		// Setup bidirectional RPC for plugin-to-plugin calls
		daemonService := NewDaemonServiceImpl(s.serviceRegistry)
		if err := loader.client.SetupDaemonService(daemonService); err != nil {
			logger.Warn("Failed to setup daemon service for plugin",
				logger.Field{Key: "plugin", Value: name},
				logger.Field{Key: "error", Value: err.Error()})
		}
	}

	// Load and apply plugin config
	var config map[string]interface{}
	namespace := metadata.Namespace
	if err := state.LoadConfig(name, &config); err != nil {
		// Config file doesn't exist - check if plugin provides defaults
		if metadata.DefaultConfig != nil {
			logger.Info("Using plugin default config",
				logger.Field{Key: "plugin", Value: name})
			config = metadata.DefaultConfig
		} else {
			logger.Info("No config file or defaults available",
				logger.Field{Key: "plugin", Value: name})
			config = make(map[string]interface{})
		}
	}

	// Store in state
	s.state.LoadCommitted(namespace, config)

	if len(config) > 0 {
		logger.Info("Loaded plugin configuration",
			logger.Field{Key: "plugin", Value: name},
			logger.Field{Key: "namespace", Value: namespace})

		// Apply config to initialize plugin
		if err := plugin.ApplyConfig(config); err != nil {
			return fmt.Errorf("failed to apply plugin config: %w", err)
		}

		// Mark plugin's services as ready after successful ApplyConfig
		if loader, ok := plugin.(*PluginLoader); ok {
			providedServices := loader.GetProvidedServices()
			for _, service := range providedServices {
				s.serviceRegistry.MarkServiceReady(service.Name)
				logger.Info("Service is ready",
					logger.Field{Key: "service", Value: service.Name},
					logger.Field{Key: "provider", Value: namespace})
			}
		}
	}

	// Register plugin with logger emitter if logger is initialized
	s.registerPluginLogSubscriber(plugin, name)

	logger.Info("Plugin loaded successfully",
		logger.Field{Key: "plugin", Value: name})

	return nil
}

// registerPluginLogSubscriber registers a plugin with the logger emitter
// so it can receive log events (e.g., sqlite3 database plugin)
func (s *Server) registerPluginLogSubscriber(plugin plugins.Plugin, name string) {
	// Get the logger emitter
	emitter := logger.GetEmitter()
	if emitter == nil {
		logger.Warn("Cannot register plugin - logger emitter is nil",
			logger.Field{Key: "plugin", Value: name})
		return // Logger not initialized
	}

	// Get the underlying RPC provider
	loader, ok := plugin.(*PluginLoader)
	if !ok {
		logger.Warn("Cannot register plugin - not a PluginLoader",
			logger.Field{Key: "plugin", Value: name},
			logger.Field{Key: "type", Value: fmt.Sprintf("%T", plugin)})
		return // Not a PluginLoader
	}

	provider := loader.GetProvider()

	// Create a subscriber that forwards log events to the plugin
	subscriber := NewPluginLogSubscriber(provider, name)

	// Register with the emitter
	emitter.Subscribe(subscriber)

	logger.Info("Registered plugin for log events",
		logger.Field{Key: "plugin", Value: name})
}

func (s *Server) Start(applyOnStartup bool) error {
	logger.Info("Jack daemon starting")

	// Load plugins
	if err := s.loadPlugins(); err != nil {
		logger.Warn("Failed to load plugins", logger.Field{Key: "error", Value: err.Error()})
	}

	// Start network observer to detect external configuration changes
	s.networkObserver = NewNetworkObserver(s)
	go func() {
		if err := s.networkObserver.Run(s.done); err != nil {
			logger.Error("Network observer failed", logger.Field{Key: "error", Value: err.Error()})
		}
	}()

	// Load snapshots from disk
	if err := s.state.LoadSnapshotsFromDisk(); err != nil {
		logger.Warn("Failed to load snapshots", logger.Field{Key: "error", Value: err.Error()})
	} else {
		snapshots := s.state.ListSnapshots()
		logger.Info("Loaded snapshots from disk", logger.Field{Key: "count", Value: len(snapshots)})
	}

	// Load interfaces config (auto-generates default config on first boot)
	interfacesConfig, err := state.LoadInterfacesConfig()
	if err != nil {
		return fmt.Errorf("failed to load interfaces config: %w\n\nTip: Run 'jack validate' to check for configuration errors", err)
	}
	s.state.LoadCommittedInterfaces(interfacesConfig)
	logger.Info("Loaded interfaces", logger.Field{Key: "count", Value: len(interfacesConfig.Interfaces)})

	// Load routes config (required for core functionality)
	var routesConfig types.RoutesConfig
	if err := state.LoadConfig("routes", &routesConfig); err != nil {
		logger.Warn("Failed to load routes config", logger.Field{Key: "error", Value: err.Error()})
		// Initialize empty routes config so it can be modified via set command
		routesConfig = types.RoutesConfig{
			Routes: make(map[string]types.Route),
		}
	}
	// Always register routes config (even if empty)
	s.state.LoadCommittedRoutes(&routesConfig)
	logger.Info("Loaded routes", logger.Field{Key: "count", Value: len(routesConfig.Routes)})

	// Load plugin configs generically for all registered plugins
	for _, namespace := range s.registry.List() {
		// Get plugin name for config file (use plugin name, not namespace)
		pluginName, found := s.registry.GetPluginNameForNamespace(namespace)
		if !found {
			logger.Warn("No plugin name found for namespace, skipping config load",
				logger.Field{Key: "namespace", Value: namespace})
			continue
		}

		var config map[string]interface{}
		if err := state.LoadConfig(pluginName, &config); err != nil {
			// Config file doesn't exist - check if plugin provides defaults
			if plugin, exists := s.registry.Get(namespace); exists {
				metadata := plugin.Metadata()
				if metadata.DefaultConfig != nil {
					logger.Info("Using plugin default config",
						logger.Field{Key: "plugin", Value: pluginName})
					config = metadata.DefaultConfig
				} else {
					logger.Info("No config file or defaults available",
						logger.Field{Key: "plugin", Value: pluginName})
					// Still register empty config so 'set' command works
					config = make(map[string]interface{})
				}
			} else {
				logger.Info("No config file found",
					logger.Field{Key: "plugin", Value: pluginName})
				config = make(map[string]interface{})
			}
		}
		// Store in state using generic method (still use namespace as key internally)
		s.state.LoadCommitted(namespace, config)
		if len(config) > 0 {
			logger.Info("Loaded plugin configuration",
				logger.Field{Key: "plugin", Value: pluginName},
				logger.Field{Key: "namespace", Value: namespace})
		}
	}

	// Apply config on startup if requested
	if applyOnStartup {
		// Mark that Jack is making changes
		if s.networkObserver != nil {
			s.networkObserver.MarkChange()
		}

		logger.Info("Applying configuration on startup")

		// Apply interfaces first
		orderedNames := orderInterfaces(interfacesConfig.Interfaces)
		for _, name := range orderedNames {
			iface := interfacesConfig.Interfaces[name]
			if err := system.ApplyInterfaceConfig(name, iface); err != nil {
				logger.Error("Failed to configure interface",
					logger.Field{Key: "interface", Value: name},
					logger.Field{Key: "error", Value: err.Error()})
			}
		}

		// Note: Plugin configs already applied in loadPlugins() during initialization
		// No need to reapply them here - they're already running

		// Apply routes AFTER plugins have started (so VPN interfaces exist)
		if err := system.ApplyRoutesConfig(&routesConfig); err != nil {
			logger.Error("Failed to apply routes",
				logger.Field{Key: "error", Value: err.Error()})
		}

		logger.Info("Configuration applied on startup")
	}
	// Note: Plugins are already initialized in loadPlugins(), no need to apply again here

	logger.Info("Daemon listening", logger.Field{Key: "socket", Value: GetSocketPath()})

	// Accept connections
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// Check if we're shutting down
			select {
			case <-s.done:
				return nil
			default:
				logger.Error("Failed to accept connection",
					logger.Field{Key: "error", Value: err.Error()})
				continue
			}
		}

		go s.handleConnection(conn)
	}
}

func (s *Server) Stop() error {
	close(s.done)
	if s.listener != nil {
		s.listener.Close()
	}
	// Clean up all plugin processes
	if s.registry != nil {
		s.registry.CloseAll()
	}
	os.Remove(GetSocketPath())
	return nil
}

func (s *Server) handleConnection(conn net.Conn) {
	reader := bufio.NewReader(conn)
	data, err := reader.ReadBytes('\n')
	if err != nil {
		conn.Close()
		return
	}

	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		s.sendResponse(conn, Response{
			Success: false,
			Error:   fmt.Sprintf("invalid request: %v", err),
		})
		conn.Close()
		return
	}

	// Handle streaming log subscription specially (keeps connection open)
	if req.Command == "logs-subscribe" {
		defer conn.Close()

		// Use empty filter if not provided
		filter := req.LogFilter
		if filter == nil {
			filter = &LogFilter{}
		}

		s.handleLogsSubscribe(conn, filter)
		return
	}

	// For all other commands, use normal request-response pattern
	defer conn.Close()
	resp := s.handleRequest(req)
	s.sendResponse(conn, resp)
}

func (s *Server) handleRequest(req Request) Response {
	handler, exists := s.handlers[req.Command]
	if !exists {
		return Response{
			Success: false,
			Error:   fmt.Sprintf("unknown command: %s", req.Command),
		}
	}
	return handler(req)
}

func (s *Server) handleStatus() Response {
	if s.state.HasPending() {
		return Response{
			Success: true,
			Message: "Pending changes exist",
		}
	}
	return Response{
		Success: true,
		Message: "No pending changes",
	}
}

func (s *Server) handleInfo() Response {
	sysStatus, err := system.GetSystemStatus()
	if err != nil {
		return Response{
			Success: false,
			Error:   fmt.Sprintf("failed to get system status: %v", err),
		}
	}

	// Collect status from all registered plugins
	pluginStatuses := make(map[string]interface{})
	for _, namespace := range s.registry.List() {
		if plugin, exists := s.registry.Get(namespace); exists {
			if status, err := plugin.Status(); err == nil {
				pluginStatuses[namespace] = status
			}
		}
	}

	// Build response data
	data := map[string]interface{}{
		"daemon":        sysStatus.Daemon,
		"interfaces":    sysStatus.Interfaces,
		"system":        sysStatus.System,
		"ip_forwarding": sysStatus.IPForwarding,
		"plugins":       pluginStatuses,
		"pending":       s.state.HasPending(),
	}

	return Response{
		Success: true,
		Data:    data,
		Message: "System status retrieved",
	}
}

func (s *Server) handleDiff() Response {
	if !s.state.HasPending() {
		return Response{
			Success: true,
			Data:    "",
			Message: "No pending changes",
		}
	}

	// Get all config types with pending changes
	pendingTypes := s.state.GetPendingTypes()

	// Collect diffs from all pending config types
	var allDiffs []DiffResult
	for _, configType := range pendingTypes {
		committed, err := s.state.GetCommitted(configType)
		if err != nil {
			return Response{
				Success: false,
				Error:   fmt.Sprintf("failed to get committed %s config: %v", configType, err),
			}
		}

		pending, err := s.state.GetCurrent(configType)
		if err != nil {
			return Response{
				Success: false,
				Error:   fmt.Sprintf("failed to get pending %s config: %v", configType, err),
			}
		}

		diffs, err := DiffConfigs(configType, committed, pending)
		if err != nil {
			return Response{
				Success: false,
				Error:   fmt.Sprintf("failed to diff %s config: %v", configType, err),
			}
		}

		allDiffs = append(allDiffs, diffs...)
	}

	// Return formatted diff string in Data field for CLI/test consumption
	return Response{
		Success: true,
		Data:    FormatDiff(allDiffs),
		Message: fmt.Sprintf("%d change(s)", len(allDiffs)),
	}
}

// validatePending validates all pending configurations before committing
func (s *Server) validatePending() error {
	v := validation.NewCollector()

	pendingTypes := s.state.GetPendingTypes()
	for _, configType := range pendingTypes {
		config, err := s.state.GetCurrent(configType)
		if err != nil {
			v.CheckMsg(err, fmt.Sprintf("failed to get %s config", configType))
			continue
		}

		switch configType {
		case "interfaces":
			if ifacesConfig, ok := config.(*types.InterfacesConfig); ok {
				for ifaceName, iface := range ifacesConfig.Interfaces {
					if err := iface.Validate(); err != nil {
						v.CheckMsg(err, fmt.Sprintf("interface %s", ifaceName))
					}
				}
			}

		case "routes":
			if routesConfig, ok := config.(*types.RoutesConfig); ok {
				for _, route := range routesConfig.Routes {
					if err := route.Validate(); err != nil {
						v.Check(err) // Route.Validate() already includes route name in context
					}
				}
			}

		default:
			// Plugin config - call plugin's ValidateConfig RPC method
			plugin, exists := s.registry.Get(configType)
			if !exists {
				// If plugin not loaded, skip validation (might be disabled)
				logger.Debug("Skipping validation for unloaded plugin",
					logger.Field{Key: "namespace", Value: configType})
				continue
			}

			// Call plugin's ValidateConfig method
			if err := plugin.ValidateConfig(config); err != nil {
				v.CheckMsg(err, fmt.Sprintf("%s plugin", configType))
			}
		}
	}

	return v.Error()
}

func (s *Server) handleCommit() Response {
	// Validate all pending configs before committing
	if err := s.validatePending(); err != nil {
		return Response{
			Success: false,
			Error:   fmt.Sprintf("validation failed:\n%v", err),
		}
	}

	// Get list of pending types before committing
	pendingTypes := s.state.GetPendingTypes()
	if len(pendingTypes) == 0 {
		// Idempotent behavior: commit with no changes succeeds
		return Response{
			Success: true,
			Message: "No pending changes to commit",
		}
	}

	// Commit all pending changes
	if err := s.state.CommitPending(); err != nil {
		return Response{
			Success: false,
			Error:   err.Error(),
		}
	}

	// Save each committed config type to disk
	for _, configType := range pendingTypes {
		config, err := s.state.GetCommitted(configType)
		if err != nil {
			return Response{
				Success: false,
				Error:   fmt.Sprintf("failed to get committed %s config: %v", configType, err),
			}
		}

		// Determine filename for saving
		// Core configs (interfaces, routes) use their type as filename
		// Plugin configs use plugin name (not namespace)
		filename := configType
		if configType != "interfaces" && configType != "routes" {
			// This is a plugin namespace - get plugin name for filename
			if pluginName, found := s.registry.GetPluginNameForNamespace(configType); found {
				filename = pluginName
			}
		}

		// Save config using plugin name or config type
		if err := state.SaveConfig(filename, config); err != nil {
			return Response{
				Success: false,
				Error:   fmt.Sprintf("failed to save %s config: %v", filename, err),
			}
		}
		logger.Info("Saved configuration file",
			logger.Field{Key: "filename", Value: filename})
	}

	return Response{
		Success: true,
		Message: "Changes committed",
	}
}

func (s *Server) handleRevert() Response {
	if err := s.state.RevertPending(); err != nil {
		return Response{
			Success: false,
			Error:   err.Error(),
		}
	}

	return Response{
		Success: true,
		Message: "Pending changes discarded",
	}
}

func (s *Server) handleApply() Response {
	logger.Info("Starting apply operation")

	// Capture snapshot before any changes
	snapshot, err := system.CaptureSystemSnapshot()
	if err != nil {
		return Response{
			Success: false,
			Error:   fmt.Sprintf("failed to capture snapshot: %v", err),
		}
	}

	// Store snapshot for potential rollback
	checkpointID := snapshot.CheckpointID
	if err := s.state.SaveSnapshot(checkpointID, snapshot); err != nil {
		logger.Warn("Failed to save snapshot",
			logger.Field{Key: "error", Value: err.Error()})
		// Continue anyway - don't block apply
	} else {
		logger.Info("Created checkpoint",
			logger.Field{Key: "checkpoint_id", Value: checkpointID})
	}

	// Execute apply with automatic rollback on error
	warnings, err := s.executeApplyWithRollback(snapshot)
	if err != nil {
		return Response{
			Success: false,
			Error:   fmt.Sprintf("apply failed, rolled back to checkpoint %s: %v", checkpointID, err),
		}
	}

	// Success - prune old checkpoints (keep last 10)
	s.state.PruneOldSnapshots(10)

	// Build response message with warnings
	message := fmt.Sprintf("Configuration applied (interfaces + %s + routes)",
		strings.Join(s.registry.List(), " + "))

	if len(warnings) > 0 {
		message += "\n" + strings.Join(warnings, "\n")
	}

	return Response{
		Success: true,
		Message: message,
	}
}

// executeApplyWithRollback performs the actual apply operations with automatic rollback on failure.
// Returns warnings and error. Warnings are informational messages that don't prevent success.
func (s *Server) executeApplyWithRollback(snapshot *system.SystemSnapshot) ([]string, error) {
	// Mark that Jack is making changes to prevent observer from treating them as external
	if s.networkObserver != nil {
		s.networkObserver.MarkChange()
	}

	// Track what we've applied for granular rollback
	appliedSteps := []string{}

	// Step 1: Enable IP forwarding
	if err := system.EnableIPForwarding(); err != nil {
		return nil, fmt.Errorf("ip forwarding: %w", err)
	}
	appliedSteps = append(appliedSteps, "ipforward")

	// Step 2: Apply interface configuration in correct order
	logger.Info("[DEBUG] About to call GetCommittedInterfaces()")
	interfaces := s.state.GetCommittedInterfaces()
	logger.Info("[DEBUG] GetCommittedInterfaces() returned")
	orderedNames := orderInterfaces(interfaces.Interfaces)

	logger.Info("[DEBUG] About to apply interfaces",
		logger.Field{Key: "interface_count", Value: len(orderedNames)})

	// Add interfaces to appliedSteps BEFORE applying, so rollback will restore them if any interface fails
	appliedSteps = append(appliedSteps, "interfaces")

	for _, name := range orderedNames {
		logger.Info("[DEBUG] Applying interface",
			logger.Field{Key: "interface", Value: name})
		iface := interfaces.Interfaces[name]
		if err := system.ApplyInterfaceConfig(name, iface); err != nil {
			// Rollback interfaces and IP forwarding
			logger.Error("Interface apply failed, rolling back",
				logger.Field{Key: "interface", Value: name},
				logger.Field{Key: "error", Value: err.Error()})
			if rollbackErr := system.RestoreSnapshot(snapshot, appliedSteps); rollbackErr != nil {
				logger.Error("Rollback failed",
					logger.Field{Key: "error", Value: rollbackErr.Error()})
			}
			return nil, fmt.Errorf("interface %s: %w", name, err)
		}
	}

	logger.Info("[DEBUG] Finished applying interfaces")

	// Step 3: Apply plugin configurations in dependency order
	// Get the jack config to determine plugin order
	jackConfig := s.state.GetCurrentJackConfig()

	// Order plugins by dependencies
	var orderedNamespaces []string
	if jackConfig != nil {
		orderedPlugins := orderPluginsByDependencies(jackConfig.Plugins)
		// Convert plugin names to namespaces
		for _, pluginName := range orderedPlugins {
			if namespace, found := s.registry.GetNamespaceForPlugin(pluginName); found {
				orderedNamespaces = append(orderedNamespaces, namespace)
			}
		}
	} else {
		// Fallback to registry order if we can't get jack config
		orderedNamespaces = s.registry.List()
	}

	logger.Info("Applying plugins in dependency order",
		logger.Field{Key: "order", Value: fmt.Sprintf("%v", orderedNamespaces)})

	logger.Info("[DEBUG] About to start plugin application loop",
		logger.Field{Key: "plugin_count", Value: len(orderedNamespaces)})

	for _, namespace := range orderedNamespaces {
		logger.Info("[DEBUG] Processing plugin in loop",
			logger.Field{Key: "namespace", Value: namespace})
		plugin, _ := s.registry.Get(namespace)

		// Get plugin name for config file
		pluginName, found := s.registry.GetPluginNameForNamespace(namespace)
		if !found {
			logger.Warn("No plugin name found for namespace, skipping",
				logger.Field{Key: "namespace", Value: namespace})
			continue
		}

		// Try to get committed config from state first (from set/commit commands)
		var config map[string]interface{}
		if committedConfig, err := s.state.GetCommitted(namespace); err == nil {
			// Use committed config from state
			if cfgMap, ok := committedConfig.(map[string]interface{}); ok {
				config = cfgMap
				logger.Info("Using committed configuration from state",
					logger.Field{Key: "plugin", Value: namespace})
			}
		} else {
			// No committed config - try to load from file
			if err := state.LoadConfig(pluginName, &config); err != nil {
				// No config file exists - try to use default config from plugin metadata
				metadata := plugin.Metadata()
				if metadata.DefaultConfig != nil {
					logger.Info("Using plugin default config",
						logger.Field{Key: "plugin", Value: pluginName},
						logger.Field{Key: "namespace", Value: namespace})
					config = metadata.DefaultConfig
				} else {
					logger.Info("No config file or defaults available, skipping",
						logger.Field{Key: "plugin", Value: pluginName},
						logger.Field{Key: "namespace", Value: namespace})
					continue
				}
			}
		}

		// Check if config has changed since last apply
		lastApplied, err := s.state.GetLastApplied(namespace)
		if err != nil {
			logger.Warn("Failed to get last applied config",
				logger.Field{Key: "plugin", Value: namespace},
				logger.Field{Key: "error", Value: err.Error()})
		} else if ConfigsEqual(config, lastApplied) {
			logger.Info("Configuration unchanged, skipping apply",
				logger.Field{Key: "plugin", Value: namespace})
			continue
		}

		// Apply config through the plugin
		// Dependency ordering ensures service providers are applied before consumers
		logger.Info("Applying plugin configuration",
			logger.Field{Key: "plugin", Value: namespace},
			logger.Field{Key: "config_file", Value: pluginName + ".json"})
		if err := plugin.ApplyConfig(config); err != nil {
			// Rollback everything including plugins
			logger.Error("Plugin apply failed, rolling back",
				logger.Field{Key: "plugin", Value: namespace},
				logger.Field{Key: "error", Value: err.Error()})
			s.rollbackPlugins(snapshot)
			if rollbackErr := system.RestoreSnapshot(snapshot, appliedSteps); rollbackErr != nil {
				logger.Error("Rollback failed",
					logger.Field{Key: "error", Value: rollbackErr.Error()})
			}
			return nil, fmt.Errorf("plugin %s: %w", namespace, err)
		}

		// Track this config as successfully applied
		s.state.SetLastApplied(namespace, config)
		logger.Info("Applied plugin configuration",
			logger.Field{Key: "plugin", Value: namespace})

		// Mark plugin's services as ready after successful ApplyConfig
		if loader, ok := plugin.(*PluginLoader); ok {
			providedServices := loader.GetProvidedServices()
			for _, service := range providedServices {
				s.serviceRegistry.MarkServiceReady(service.Name)
			}
		}
	}
	appliedSteps = append(appliedSteps, "plugins")

	// Step 4: Apply static routes
	var routesConfig types.RoutesConfig
	if err := state.LoadConfig("routes", &routesConfig); err != nil {
		logger.Warn("No routes config found",
			logger.Field{Key: "error", Value: err.Error()})
	} else {
		if err := system.ApplyRoutesConfig(&routesConfig); err != nil {
			// Full rollback
			logger.Error("Routes apply failed, rolling back",
				logger.Field{Key: "error", Value: err.Error()})
			s.rollbackPlugins(snapshot)
			if rollbackErr := system.RestoreSnapshot(snapshot, []string{"all"}); rollbackErr != nil {
				logger.Error("Rollback failed",
					logger.Field{Key: "error", Value: rollbackErr.Error()})
			}
			return nil, fmt.Errorf("routes: %w", err)
		}
	}
	appliedSteps = append(appliedSteps, "routes")

	// Collect warnings from all plugins
	var warnings []string
	for _, namespace := range s.registry.List() {
		plugin, _ := s.registry.Get(namespace)

		// Get plugin status to check for warnings
		statusData, err := plugin.Status()
		if err != nil {
			continue // Skip if status fails
		}

		// Type assert to []byte
		statusJSON, ok := statusData.([]byte)
		if !ok {
			continue // Skip if not []byte
		}

		var status map[string]interface{}
		if err := json.Unmarshal(statusJSON, &status); err != nil {
			continue
		}

		// Extract warnings array if present
		if warningsRaw, ok := status["warnings"]; ok {
			if warningsArray, ok := warningsRaw.([]interface{}); ok {
				for _, w := range warningsArray {
					if warningStr, ok := w.(string); ok {
						warnings = append(warnings, fmt.Sprintf("[WARN] %s: %s", namespace, warningStr))
					}
				}
			}
		}
	}

	logger.Info("Apply operation completed successfully")
	return warnings, nil
}

// rollbackPlugins rolls back all plugins to their snapshot state.
func (s *Server) rollbackPlugins(snapshot *system.SystemSnapshot) {
	logger.Info("Rolling back plugins")

	for _, namespace := range s.registry.List() {
		plugin, _ := s.registry.Get(namespace)

		// Flush current state
		if err := plugin.Flush(); err != nil {
			logger.Warn("Failed to flush plugin during rollback",
				logger.Field{Key: "plugin", Value: namespace},
				logger.Field{Key: "error", Value: err.Error()})
		}

		// Get plugin name
		pluginName, found := s.registry.GetPluginNameForNamespace(namespace)
		if !found {
			continue
		}

		// Try to re-apply old config from LastApplied
		if oldConfig, err := s.state.GetLastApplied(namespace); err == nil && oldConfig != nil {
			logger.Info("Restoring plugin to last known good config",
				logger.Field{Key: "plugin", Value: namespace})
			if err := plugin.ApplyConfig(oldConfig); err != nil {
				logger.Warn("Failed to restore plugin",
					logger.Field{Key: "plugin", Value: namespace},
					logger.Field{Key: "error", Value: err.Error()})
			}
		} else {
			// Try loading from file as fallback
			var config map[string]interface{}
			if err := state.LoadConfig(pluginName, &config); err == nil {
				logger.Info("Restoring plugin from config file",
					logger.Field{Key: "plugin", Value: namespace})
				if err := plugin.ApplyConfig(config); err != nil {
					logger.Warn("Failed to restore plugin",
						logger.Field{Key: "plugin", Value: namespace},
						logger.Field{Key: "error", Value: err.Error()})
				}
			}
		}
	}
}

func (s *Server) handleShow(path string) Response {
	// If no path specified, return all configs
	if path == "" {
		allConfigs := map[string]interface{}{}

		// Add interfaces - return just the map, not the whole struct
		if interfaces := s.state.GetCurrentInterfaces(); interfaces != nil {
			allConfigs["interfaces"] = interfaces.Interfaces
		}

		// Add routes - return just the map, not the whole struct
		if routes := s.state.GetCurrentRoutes(); routes != nil {
			allConfigs["routes"] = routes.Routes
		}

		// Add plugin configs dynamically
		for _, namespace := range s.registry.List() {
			if config, err := s.state.GetCurrent(namespace); err == nil {
				allConfigs[namespace] = config
			}
		}
		return Response{
			Success: true,
			Data:    allConfigs,
		}
	}

	// Parse config type from path
	configType, err := ParseConfigType(path)
	if err != nil {
		return Response{
			Success: false,
			Error:   err.Error(),
		}
	}

	// Get the appropriate config
	config, err := s.state.GetCurrent(configType)
	if err != nil {
		return Response{
			Success: false,
			Error:   err.Error(),
		}
	}

	return Response{
		Success: true,
		Data:    config,
	}
}

// rewritePath rewrites paths for plugins that have a PathPrefix configured
// Example: "led.status:green.brightness" becomes "led.leds.status:green.brightness"
func (s *Server) rewritePath(path string) string {
	parts := strings.Split(path, ".")
	if len(parts) < 2 {
		return path
	}

	namespace := parts[0]
	prefix := s.registry.GetPathPrefix(namespace)
	if prefix == "" {
		return path // No rewriting needed
	}

	// Insert the prefix after the namespace
	// Example: ["led", "status:green", "brightness"] -> ["led", "leds", "status:green", "brightness"]
	rewritten := []string{namespace, prefix}
	rewritten = append(rewritten, parts[1:]...)
	return strings.Join(rewritten, ".")
}

func (s *Server) handleGet(path string) Response {
	// If no path provided, return all current configs
	if path == "" {
		allConfigs := make(map[string]interface{})

		// Add interfaces - return just the map, not the whole struct
		if interfaces := s.state.GetCurrentInterfaces(); interfaces != nil {
			allConfigs["interfaces"] = interfaces.Interfaces
		}

		// Add routes - return just the map, not the whole struct
		if routes := s.state.GetCurrentRoutes(); routes != nil {
			allConfigs["routes"] = routes.Routes
		}

		// Add plugin configs dynamically
		for _, namespace := range s.registry.List() {
			if config, err := s.state.GetCurrent(namespace); err == nil {
				allConfigs[namespace] = config
			}
		}

		return Response{
			Success: true,
			Data:    allConfigs,
		}
	}

	// Rewrite path if plugin has a path prefix
	path = s.rewritePath(path)

	// Parse config type from path
	configType, err := ParseConfigType(path)
	if err != nil {
		return Response{
			Success: false,
			Error:   err.Error(),
		}
	}

	// Get the appropriate config from state
	config, err := s.state.GetCurrent(configType)
	if err != nil {
		return Response{
			Success: false,
			Error:   err.Error(),
		}
	}

	// Get the value from the config
	value, err := GetValue(config, path)
	if err != nil {
		return Response{
			Success: false,
			Error:   err.Error(),
		}
	}

	return Response{
		Success: true,
		Data:    value,
	}
}

func (s *Server) handleSet(path string, value interface{}) Response {
	// Rewrite path if plugin has a path prefix
	path = s.rewritePath(path)

	// Parse config type from path
	configType, err := ParseConfigType(path)
	if err != nil {
		return Response{
			Success: false,
			Error:   err.Error(),
		}
	}

	// Get current config for this type
	config, err := s.state.GetCurrent(configType)
	if err != nil {
		return Response{
			Success: false,
			Error:   err.Error(),
		}
	}

	// If no pending changes for this config type, create a copy from committed
	if !s.state.HasPendingFor(configType) {
		committed, err := s.state.GetCommitted(configType)
		if err != nil {
			return Response{
				Success: false,
				Error:   err.Error(),
			}
		}

		// Deep copy based on config type
		switch configType {
		case "interfaces":
			if committedIface, ok := committed.(*types.InterfacesConfig); ok {
				config = deepCopyInterfaces(committedIface)
			}
		case "routes":
			if committedRoutes, ok := committed.(*types.RoutesConfig); ok {
				config = deepCopyRoutes(committedRoutes)
			}
		default:
			// Generic deep copy for plugin configs using JSON marshaling
			config = deepCopyGeneric(committed)
		}
	}

	// Apply the change
	if err := SetValue(config, path, value); err != nil {
		return Response{
			Success: false,
			Error:   err.Error(),
		}
	}

	// Save as pending
	s.state.SetPending(configType, config)

	return Response{
		Success: true,
		Message: fmt.Sprintf("Staged: %s = %v", path, value),
	}
}

// handleValidate validates configuration without applying it
func (s *Server) handleValidate(path string, value interface{}) Response {
	// Rewrite path if plugin has a path prefix
	path = s.rewritePath(path)

	// Parse config type from path
	configType, err := ParseConfigType(path)
	if err != nil {
		return Response{
			Success: false,
			Error:   err.Error(),
		}
	}

	// Try to parse and validate the configuration by marshaling/unmarshaling
	// This ensures the structure is valid
	switch configType {
	case "interfaces":
		// Try to convert to InterfacesConfig structure
		var interfacesMap map[string]types.Interface
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return Response{
				Success: false,
				Error:   fmt.Sprintf("failed to marshal interfaces: %v", err),
			}
		}
		if err := json.Unmarshal(jsonBytes, &interfacesMap); err != nil {
			return Response{
				Success: false,
				Error:   fmt.Sprintf("invalid interfaces structure: %v", err),
			}
		}

		// Basic validation of interface fields
		for name, iface := range interfacesMap {
			if iface.Type == "" {
				return Response{
					Success: false,
					Error:   fmt.Sprintf("interface %s: type is required", name),
				}
			}
			if iface.Type != "physical" && iface.Type != "bridge" && iface.Type != "vlan" {
				return Response{
					Success: false,
					Error:   fmt.Sprintf("interface %s: invalid type %s", name, iface.Type),
				}
			}
		}

	case "routes":
		// Try to convert to routes structure (map of Route)
		var routesMap map[string]types.Route
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return Response{
				Success: false,
				Error:   fmt.Sprintf("failed to marshal routes: %v", err),
			}
		}
		if err := json.Unmarshal(jsonBytes, &routesMap); err != nil {
			return Response{
				Success: false,
				Error:   fmt.Sprintf("invalid routes structure: %v", err),
			}
		}

	default:
		// For plugin configs, just check if it's valid JSON
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return Response{
				Success: false,
				Error:   fmt.Sprintf("failed to marshal config: %v", err),
			}
		}

		var testMap map[string]interface{}
		if err := json.Unmarshal(jsonBytes, &testMap); err != nil {
			return Response{
				Success: false,
				Error:   fmt.Sprintf("invalid config structure: %v", err),
			}
		}
	}

	return Response{
		Success: true,
		Message: "Configuration is valid",
	}
}

// Deep copy functions for each config type
func deepCopyInterfaces(src *types.InterfacesConfig) *types.InterfacesConfig {
	if src == nil {
		return nil
	}
	dst := *src
	dst.Interfaces = make(map[string]types.Interface)
	for k, v := range src.Interfaces {
		dst.Interfaces[k] = v
	}
	return &dst
}

func deepCopyRoutes(src *types.RoutesConfig) *types.RoutesConfig {
	if src == nil {
		return nil
	}
	dst := *src

	// Copy routes map
	dst.Routes = make(map[string]types.Route, len(src.Routes))
	for name, route := range src.Routes {
		dst.Routes[name] = route
	}

	return &dst
}

// deepCopyGeneric performs a generic deep copy using JSON marshaling
// This is used for plugin configs which are map[string]interface{}
func deepCopyGeneric(src interface{}) interface{} {
	if src == nil {
		return nil
	}

	// Marshal to JSON and back to create a deep copy
	jsonData, err := json.Marshal(src)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal for deep copy: %v", err)
		return src
	}

	var dst interface{}
	if err := json.Unmarshal(jsonData, &dst); err != nil {
		log.Printf("[ERROR] Failed to unmarshal for deep copy: %v", err)
		return src
	}

	return dst
}

// orderInterfaces returns interface names ordered by dependency
// Order: physical -> vlan -> bridge
func orderInterfaces(interfaces map[string]types.Interface) []string {
	physical := []string{}
	vlan := []string{}
	bridge := []string{}
	other := []string{}

	for name, iface := range interfaces {
		switch iface.Type {
		case "physical":
			physical = append(physical, name)
		case "vlan":
			vlan = append(vlan, name)
		case "bridge":
			bridge = append(bridge, name)
		default:
			other = append(other, name)
		}
	}

	// Combine in order: physical, bridge, vlan, other
	// Bridges must come before VLANs since VLANs can be on top of bridges
	result := make([]string, 0, len(interfaces))
	result = append(result, physical...)
	result = append(result, bridge...)
	result = append(result, vlan...)
	result = append(result, other...)

	return result
}

func (s *Server) sendResponse(conn net.Conn, resp Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		logger.Error("Failed to marshal response",
			logger.Field{Key: "error", Value: err.Error()})
		return
	}

	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		logger.Error("Failed to write response",
			logger.Field{Key: "error", Value: err.Error()})
	}
}

// handlePluginEnable enables a plugin at runtime
func (s *Server) handlePluginEnable(pluginName string) Response {
	if pluginName == "" {
		return Response{Success: false, Error: "plugin name required"}
	}

	// Load jack config
	jackConfig, err := state.LoadJackConfig()
	if err != nil {
		return Response{Success: false, Error: fmt.Sprintf("failed to load config: %v", err)}
	}

	// Check if plugin already enabled and loaded
	pluginState, exists := jackConfig.Plugins[pluginName]
	if exists && pluginState.Enabled && s.registry.IsRegistered(pluginState.Version) {
		return Response{Success: false, Error: fmt.Sprintf("plugin '%s' is already enabled", pluginName)}
	}

	// Load the plugin
	plugin, err := LoadPlugin(pluginName)
	if err != nil {
		return Response{Success: false, Error: fmt.Sprintf("failed to load plugin: %v", err)}
	}

	metadata := plugin.Metadata()

	// Register plugin
	if err := s.registry.Register(plugin, pluginName); err != nil {
		plugin.Close()
		return Response{Success: false, Error: fmt.Sprintf("failed to register plugin: %v", err)}
	}

	// Register plugin services with service registry
	if loader, ok := plugin.(*PluginLoader); ok {
		providedServices := loader.GetProvidedServices()
		if err := s.serviceRegistry.RegisterPlugin(metadata.Namespace, plugin, providedServices); err != nil {
			s.registry.Unregister(metadata.Namespace)
			plugin.Close()
			return Response{Success: false, Error: fmt.Sprintf("failed to register plugin services: %v", err)}
		}

		// Validate that required services are available
		requiredServices := loader.GetRequiredServices()
		if err := s.serviceRegistry.ValidateServiceDependencies(requiredServices); err != nil {
			logger.Warn("Plugin has unmet service dependencies",
				logger.Field{Key: "plugin", Value: pluginName},
				logger.Field{Key: "error", Value: err.Error()})
			// Don't fail - dependencies may be satisfied later
		}
	}

	// Load and apply config if it exists (use plugin name for config file)
	var pluginConfig interface{}
	if err := state.LoadConfig(pluginName, &pluginConfig); err == nil {
		if err := plugin.ApplyConfig(pluginConfig); err != nil {
			logger.Warn("Failed to apply config from file",
				logger.Field{Key: "plugin", Value: pluginName},
				logger.Field{Key: "error", Value: err.Error()})
		} else {
			logger.Info("Applied config from file",
				logger.Field{Key: "plugin", Value: pluginName},
				logger.Field{Key: "config_file", Value: pluginName + ".json"})
		}
	} else {
		// No config file exists - use default config from plugin metadata
		if metadata.DefaultConfig != nil {
			logger.Info("Using default config for plugin",
				logger.Field{Key: "plugin", Value: pluginName})
			if err := plugin.ApplyConfig(metadata.DefaultConfig); err != nil {
				logger.Warn("Failed to apply default config",
					logger.Field{Key: "plugin", Value: pluginName},
					logger.Field{Key: "error", Value: err.Error()})
			} else {
				logger.Info("Applied default config",
					logger.Field{Key: "plugin", Value: pluginName})
			}
		} else {
			logger.Info("No config file or defaults available",
				logger.Field{Key: "plugin", Value: pluginName})
		}
	}

	// Update jack config
	jackConfig.Plugins[pluginName] = types.PluginState{
		Enabled: true,
		Version: metadata.Version,
	}

	if err := state.SaveJackConfig(jackConfig); err != nil {
		return Response{Success: false, Error: fmt.Sprintf("failed to save config: %v", err)}
	}

	return Response{
		Success: true,
		Message: fmt.Sprintf("Plugin '%s' enabled successfully", pluginName),
	}
}

// handlePluginDisable disables a plugin at runtime
func (s *Server) handlePluginDisable(pluginName string) Response {
	if pluginName == "" {
		return Response{Success: false, Error: "plugin name required"}
	}

	// Load jack config
	jackConfig, err := state.LoadJackConfig()
	if err != nil {
		return Response{Success: false, Error: fmt.Sprintf("failed to load config: %v", err)}
	}

	// Check if plugin exists in config
	pluginState, exists := jackConfig.Plugins[pluginName]
	if !exists {
		return Response{Success: false, Error: fmt.Sprintf("plugin '%s' not found in configuration", pluginName)}
	}

	if !pluginState.Enabled {
		return Response{Success: false, Error: fmt.Sprintf("plugin '%s' is already disabled", pluginName)}
	}

	// Check dependencies
	if err := CheckDependencies(pluginName, s.registry); err != nil {
		return Response{Success: false, Error: err.Error()}
	}

	// Get namespace for this plugin name
	namespace, found := s.registry.GetNamespaceForPlugin(pluginName)
	if !found {
		// Plugin not currently loaded, just mark as disabled
		pluginState.Enabled = false
		jackConfig.Plugins[pluginName] = pluginState
		if err := state.SaveJackConfig(jackConfig); err != nil {
			return Response{Success: false, Error: fmt.Sprintf("failed to save config: %v", err)}
		}
		return Response{
			Success: true,
			Message: fmt.Sprintf("Plugin '%s' disabled (was not loaded)", pluginName),
		}
	}

	// Get the plugin from registry by namespace
	plugin, ok := s.registry.Get(namespace)
	if !ok {
		return Response{Success: false, Error: fmt.Sprintf("plugin '%s' not found in registry", pluginName)}
	}

	// Unload plugin (flush and close)
	if err := UnloadPlugin(plugin, namespace); err != nil {
		return Response{Success: false, Error: fmt.Sprintf("failed to unload plugin: %v", err)}
	}

	// Unregister from plugin registry
	s.registry.Unregister(namespace)

	// Unregister from service registry
	s.serviceRegistry.UnregisterPlugin(namespace)

	// Update jack config
	pluginState.Enabled = false
	jackConfig.Plugins[pluginName] = pluginState

	if err := state.SaveJackConfig(jackConfig); err != nil {
		return Response{Success: false, Error: fmt.Sprintf("failed to save config: %v", err)}
	}

	return Response{
		Success: true,
		Message: fmt.Sprintf("Plugin '%s' disabled successfully", pluginName),
	}
}

// handlePluginRescan scans for new plugins and adds them to config as disabled
func (s *Server) handlePluginRescan() Response {
	// Scan for all available plugins
	availablePlugins, err := ScanPlugins()
	if err != nil {
		return Response{Success: false, Error: fmt.Sprintf("failed to scan plugins: %v", err)}
	}

	// Load jack config
	jackConfig, err := state.LoadJackConfig()
	if err != nil {
		return Response{Success: false, Error: fmt.Sprintf("failed to load config: %v", err)}
	}

	// Add new plugins as disabled
	var newPlugins []string
	for pluginName, metadata := range availablePlugins {
		if _, exists := jackConfig.Plugins[pluginName]; !exists {
			jackConfig.Plugins[pluginName] = types.PluginState{
				Enabled: false,
				Version: metadata.Version,
			}
			newPlugins = append(newPlugins, pluginName)
		}
	}

	if len(newPlugins) == 0 {
		return Response{
			Success: true,
			Message: "No new plugins found",
		}
	}

	// Save updated config
	if err := state.SaveJackConfig(jackConfig); err != nil {
		return Response{Success: false, Error: fmt.Sprintf("failed to save config: %v", err)}
	}

	return Response{
		Success: true,
		Message: fmt.Sprintf("Added %d new plugin(s) as disabled: %v", len(newPlugins), newPlugins),
		Data:    newPlugins,
	}
}

// handlePluginCLI forwards CLI commands to the daemon's plugin instance
func (s *Server) handlePluginCLI(pluginName, cliCommand string, cliArgs []string) Response {
	// Try to get namespace from plugin name first (converts "leds" -> "led")
	namespace, found := s.registry.GetNamespaceForPlugin(pluginName)
	if !found {
		// Fall back to treating pluginName as namespace directly
		namespace = pluginName
	}

	// Get plugin from registry by namespace
	plugin, exists := s.registry.Get(namespace)
	if !exists {
		return Response{
			Success: false,
			Error:   fmt.Sprintf("plugin not found or not loaded: %s", pluginName),
		}
	}

	// Cast to PluginLoader to access ExecuteCLICommand
	loader, ok := plugin.(*PluginLoader)
	if !ok {
		return Response{
			Success: false,
			Error:   fmt.Sprintf("plugin does not support CLI commands: %s", pluginName),
		}
	}

	// Execute CLI command on daemon's plugin instance
	output, err := loader.ExecuteCLICommand(context.Background(), cliCommand, cliArgs)
	if err != nil {
		return Response{
			Success: false,
			Error:   fmt.Sprintf("command execution failed: %v", err),
		}
	}

	return Response{
		Success: true,
		Data:    string(output),
	}
}

// handleRollback restores the system to a previous checkpoint.
func (s *Server) handleRollback(checkpointID string) Response {
	// Default to latest if not specified
	if checkpointID == "" {
		checkpointID = "latest"
	}

	logger.Info("Rolling back to checkpoint",
		logger.Field{Key: "checkpoint_id", Value: checkpointID})

	// Load the snapshot
	snapshot, err := s.state.LoadSnapshot(checkpointID)
	if err != nil {
		return Response{
			Success: false,
			Error:   fmt.Sprintf("failed to load checkpoint: %v", err),
		}
	}

	// Rollback plugins first
	s.rollbackPlugins(snapshot)

	// Rollback system state
	if err := system.RestoreSnapshot(snapshot, []string{"all"}); err != nil {
		return Response{
			Success: false,
			Error:   fmt.Sprintf("rollback failed: %v", err),
		}
	}

	// Restore firewall rules if present
	if snapshot.NftablesRules != "" {
		if err := system.RestoreNftablesRules(snapshot.NftablesRules); err != nil {
			logger.Warn("Failed to restore firewall rules",
				logger.Field{Key: "error", Value: err.Error()})
		}
	}

	logger.Info("Successfully rolled back to checkpoint",
		logger.Field{Key: "checkpoint_id", Value: checkpointID})

	return Response{
		Success: true,
		Message: fmt.Sprintf("Successfully rolled back to checkpoint: %s", checkpointID),
	}
}

// handleCheckpointList returns a list of available checkpoints.
func (s *Server) handleCheckpointList() Response {
	snapshots := s.state.ListSnapshots()

	return Response{
		Success: true,
		Data:    snapshots,
	}
}

// handleCheckpointCreate creates a manual checkpoint.
func (s *Server) handleCheckpointCreate() Response {
	logger.Info("Creating manual checkpoint")

	// Capture snapshot
	snapshot, err := system.CaptureSystemSnapshot()
	if err != nil {
		return Response{
			Success: false,
			Error:   fmt.Sprintf("failed to capture snapshot: %v", err),
		}
	}

	// Use manual prefix for checkpoint ID
	snapshot.CheckpointID = fmt.Sprintf("manual-%d", snapshot.Timestamp.Unix())

	// Save snapshot
	if err := s.state.SaveSnapshot(snapshot.CheckpointID, snapshot); err != nil {
		return Response{
			Success: false,
			Error:   fmt.Sprintf("failed to save checkpoint: %v", err),
		}
	}

	logger.Info("Created checkpoint",
		logger.Field{Key: "checkpoint_id", Value: snapshot.CheckpointID})

	return Response{
		Success: true,
		Message: fmt.Sprintf("Created checkpoint: %s", snapshot.CheckpointID),
		Data:    snapshot.CheckpointID,
	}
}

// handleLogsSubscribe handles streaming log subscription
// This is a special handler that keeps the connection open
func (s *Server) handleLogsSubscribe(conn net.Conn, filter *LogFilter) {
	// Don't defer conn.Close() - it's handled by caller when client disconnects

	// Create socket subscriber with filter
	subscriber := NewSocketLogSubscriber(conn, filter)

	// Get the global logger emitter
	emitter := logger.GetEmitter()
	if emitter == nil {
		logger.Error("Logger emitter not initialized")
		return
	}

	// Subscribe to log events
	emitter.Subscribe(subscriber)
	defer func() {
		emitter.Unsubscribe(subscriber)
		subscriber.Close()
	}()

	logger.Info("Client subscribed to log stream",
		logger.Field{Key: "level", Value: filter.Level},
		logger.Field{Key: "component", Value: filter.Component})

	// Keep connection open until client disconnects
	// Just read until EOF (client closing)
	buffer := make([]byte, 1)
	for {
		_, err := conn.Read(buffer)
		if err != nil {
			// Client disconnected
			logger.Info("Client unsubscribed from log stream")
			return
		}
	}
}
