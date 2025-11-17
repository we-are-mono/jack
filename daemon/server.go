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

	"github.com/we-are-mono/jack/state"
	"github.com/we-are-mono/jack/system"
	"github.com/we-are-mono/jack/types"
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
		state:    NewState(),
		listener: listener,
		done:     make(chan struct{}),
		registry: NewPluginRegistry(),
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

// loadPlugins discovers and loads all configured plugins.
// Plugins self-register by declaring their namespace via metadata.
// Only loads plugins that are marked as enabled.
func (s *Server) loadPlugins() error {
	// Load jack config to get plugin list
	jackConfig, err := state.LoadJackConfig()
	if err != nil {
		return fmt.Errorf("failed to load Jack config: %w", err)
	}

	// Load each configured and enabled plugin
	for name, pluginState := range jackConfig.Plugins {
		if !pluginState.Enabled {
			log.Printf("[INFO] Skipping disabled plugin: %s", name)
			continue
		}

		plugin, err := LoadPlugin(name)
		if err != nil {
			log.Printf("[WARN] Failed to load plugin '%s': %v", name, err)
			continue
		}

		// Update version in config
		metadata := plugin.Metadata()
		pluginState.Version = metadata.Version
		jackConfig.Plugins[name] = pluginState

		if err := s.registry.Register(plugin, name); err != nil {
			log.Printf("[WARN] Failed to register plugin '%s': %v", name, err)
			plugin.Close()
			continue
		}
	}

	// Save updated config with versions
	if err := state.SaveJackConfig(jackConfig); err != nil {
		log.Printf("[WARN] Failed to save updated plugin versions: %v", err)
	}

	// Store jack config in state for access by observer
	s.state.LoadCommittedJackConfig(jackConfig)

	log.Printf("[REGISTRY] Loaded plugins: %v", s.registry.List())
	return nil
}

func (s *Server) Start(applyOnStartup bool) error {
	log.Println("Jack daemon starting...")

	// Load plugins
	if err := s.loadPlugins(); err != nil {
		log.Printf("[WARN] Failed to load plugins: %v", err)
	}

	// Start network observer to detect external configuration changes
	s.networkObserver = NewNetworkObserver(s)
	go func() {
		if err := s.networkObserver.Run(s.done); err != nil {
			log.Printf("[ERROR] Network observer failed: %v", err)
		}
	}()

	// Load snapshots from disk
	if err := s.state.LoadSnapshotsFromDisk(); err != nil {
		log.Printf("[WARN] Failed to load snapshots: %v", err)
	} else {
		snapshots := s.state.ListSnapshots()
		log.Printf("[INFO] Loaded %d snapshot(s) from disk", len(snapshots))
	}

	// Load interfaces config (auto-generates default config on first boot)
	interfacesConfig, err := state.LoadInterfacesConfig()
	if err != nil {
		return fmt.Errorf("failed to load interfaces config: %w\n\nTip: Run 'jack validate' to check for configuration errors", err)
	}
	s.state.LoadCommittedInterfaces(interfacesConfig)
	log.Printf("Loaded %d interfaces", len(interfacesConfig.Interfaces))

	// Load routes config (required for core functionality)
	var routesConfig types.RoutesConfig
	if err := state.LoadConfig("routes", &routesConfig); err != nil {
		log.Printf("[WARN] Failed to load routes config: %v", err)
		// Initialize empty routes config so it can be modified via set command
		routesConfig = types.RoutesConfig{
			Routes: make(map[string]types.Route),
		}
	}
	// Always register routes config (even if empty)
	s.state.LoadCommittedRoutes(&routesConfig)
	log.Printf("Loaded %d routes", len(routesConfig.Routes))

	// Load plugin configs generically for all registered plugins
	for _, namespace := range s.registry.List() {
		// Get plugin name for config file (use plugin name, not namespace)
		pluginName, found := s.registry.GetPluginNameForNamespace(namespace)
		if !found {
			log.Printf("[WARN] No plugin name found for namespace '%s', skipping config load", namespace)
			continue
		}

		var config map[string]interface{}
		if err := state.LoadConfig(pluginName, &config); err != nil {
			// Config file doesn't exist - check if plugin provides defaults
			if plugin, exists := s.registry.Get(namespace); exists {
				metadata := plugin.Metadata()
				if metadata.DefaultConfig != nil {
					log.Printf("[INFO] No %s.json config found, using plugin defaults", pluginName)
					config = metadata.DefaultConfig
				} else {
					log.Printf("[INFO] No %s.json config found and no defaults available", pluginName)
					// Still register empty config so 'set' command works
					config = make(map[string]interface{})
				}
			} else {
				log.Printf("[INFO] No %s.json config found", pluginName)
				config = make(map[string]interface{})
			}
		}
		// Store in state using generic method (still use namespace as key internally)
		s.state.LoadCommitted(namespace, config)
		if len(config) > 0 {
			log.Printf("Loaded %s.json config for %s plugin", pluginName, namespace)
		}
	}

	// Apply config on startup if requested
	if applyOnStartup {
		// Mark that Jack is making changes
		if s.networkObserver != nil {
			s.networkObserver.MarkChange()
		}

		log.Println("Applying configuration on startup...")

		// Apply interfaces first
		orderedNames := orderInterfaces(interfacesConfig.Interfaces)
		for _, name := range orderedNames {
			iface := interfacesConfig.Interfaces[name]
			if err := system.ApplyInterfaceConfig(name, iface); err != nil {
				log.Printf("[ERROR] Failed to configure %s: %v", name, err)
			}
		}

		// Apply plugin configs (creates VPN interfaces, etc.)
		for _, namespace := range s.registry.List() {
			if plugin, exists := s.registry.Get(namespace); exists {
				config, err := s.state.GetCurrent(namespace)
				if err != nil || config == nil {
					continue
				}
				if err := plugin.ApplyConfig(config); err != nil {
					log.Printf("[WARN] Failed to apply %s config: %v", namespace, err)
				} else {
					log.Printf("[%s] Plugin started with config", strings.ToUpper(namespace))
				}
			}
		}

		// Apply routes AFTER plugins (so VPN interfaces exist)
		if err := system.ApplyRoutesConfig(&routesConfig); err != nil {
			log.Printf("[ERROR] Failed to apply routes: %v", err)
		}

		log.Println("Configuration applied")
	} else {
		// If not applying on startup, still start plugins with their config
		for _, namespace := range s.registry.List() {
			if plugin, exists := s.registry.Get(namespace); exists {
				config, err := s.state.GetCurrent(namespace)
				if err != nil || config == nil {
					continue
				}
				if err := plugin.ApplyConfig(config); err != nil {
					log.Printf("[WARN] Failed to apply %s config: %v", namespace, err)
				} else {
					log.Printf("[%s] Plugin started with config", strings.ToUpper(namespace))
				}
			}
		}
	}

	log.Printf("Listening on %s", GetSocketPath())

	// Accept connections
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			// Check if we're shutting down
			select {
			case <-s.done:
				return nil
			default:
				log.Printf("[ERROR] Failed to accept connection: %v", err)
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
	os.Remove(GetSocketPath())
	return nil
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	data, err := reader.ReadBytes('\n')
	if err != nil {
		s.sendResponse(conn, Response{
			Success: false,
			Error:   fmt.Sprintf("failed to read request: %v", err),
		})
		return
	}

	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		s.sendResponse(conn, Response{
			Success: false,
			Error:   fmt.Sprintf("invalid request: %v", err),
		})
		return
	}

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

func (s *Server) handleCommit() Response {
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
		log.Printf("[COMMIT] Saved %s.json config", filename)
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
	log.Printf("[INFO] Starting apply operation")

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
		log.Printf("[WARN] Failed to save snapshot: %v", err)
		// Continue anyway - don't block apply
	} else {
		log.Printf("[INFO] Created checkpoint: %s", checkpointID)
	}

	// Execute apply with automatic rollback on error
	if err := s.executeApplyWithRollback(snapshot); err != nil {
		return Response{
			Success: false,
			Error:   fmt.Sprintf("apply failed, rolled back to checkpoint %s: %v", checkpointID, err),
		}
	}

	// Success - prune old checkpoints (keep last 10)
	s.state.PruneOldSnapshots(10)

	return Response{
		Success: true,
		Message: fmt.Sprintf("Configuration applied (interfaces + %s + routes)",
			strings.Join(s.registry.List(), " + ")),
	}
}

// executeApplyWithRollback performs the actual apply operations with automatic rollback on failure.
func (s *Server) executeApplyWithRollback(snapshot *system.SystemSnapshot) error {
	// Mark that Jack is making changes to prevent observer from treating them as external
	if s.networkObserver != nil {
		s.networkObserver.MarkChange()
	}

	// Track what we've applied for granular rollback
	appliedSteps := []string{}

	// Step 1: Enable IP forwarding
	if err := system.EnableIPForwarding(); err != nil {
		return fmt.Errorf("ip forwarding: %w", err)
	}
	appliedSteps = append(appliedSteps, "ipforward")

	// Step 2: Apply interface configuration in correct order
	interfaces := s.state.GetCommittedInterfaces()
	orderedNames := orderInterfaces(interfaces.Interfaces)

	for _, name := range orderedNames {
		iface := interfaces.Interfaces[name]
		if err := system.ApplyInterfaceConfig(name, iface); err != nil {
			// Rollback interfaces and IP forwarding
			log.Printf("[ERROR] Interface %s failed: %v, rolling back", name, err)
			if rollbackErr := system.RestoreSnapshot(snapshot, appliedSteps); rollbackErr != nil {
				log.Printf("[ERROR] Rollback failed: %v", rollbackErr)
			}
			return fmt.Errorf("interface %s: %w", name, err)
		}
	}
	appliedSteps = append(appliedSteps, "interfaces")

	// Step 3: Apply plugin configurations
	for _, namespace := range s.registry.List() {
		plugin, _ := s.registry.Get(namespace)

		// Get plugin name for config file
		pluginName, found := s.registry.GetPluginNameForNamespace(namespace)
		if !found {
			log.Printf("[WARN] No plugin name found for namespace '%s', skipping", namespace)
			continue
		}

		// Try to get committed config from state first (from set/commit commands)
		var config map[string]interface{}
		if committedConfig, err := s.state.GetCommitted(namespace); err == nil {
			// Use committed config from state
			if cfgMap, ok := committedConfig.(map[string]interface{}); ok {
				config = cfgMap
				log.Printf("[APPLY] Using committed %s configuration from state", namespace)
			}
		} else {
			// No committed config - try to load from file
			if err := state.LoadConfig(pluginName, &config); err != nil {
				// No config file exists - try to use default config from plugin metadata
				metadata := plugin.Metadata()
				if metadata.DefaultConfig != nil {
					log.Printf("[INFO] No %s.json config found, using plugin defaults for %s", pluginName, namespace)
					config = metadata.DefaultConfig
				} else {
					log.Printf("[INFO] No %s.json config found and no defaults available, skipping %s", pluginName, namespace)
					continue
				}
			}
		}

		// Check if config has changed since last apply
		lastApplied, err := s.state.GetLastApplied(namespace)
		if err != nil {
			log.Printf("[WARN] Failed to get last applied config for %s: %v", namespace, err)
		} else if ConfigsEqual(config, lastApplied) {
			log.Printf("[SKIP] %s configuration unchanged, skipping apply", namespace)
			continue
		}

		// Apply config through the plugin
		log.Printf("[APPLY] Applying %s configuration from %s.json", namespace, pluginName)
		if err := plugin.ApplyConfig(config); err != nil {
			// Rollback everything including plugins
			log.Printf("[ERROR] Plugin %s failed: %v, rolling back", namespace, err)
			s.rollbackPlugins(snapshot)
			if rollbackErr := system.RestoreSnapshot(snapshot, appliedSteps); rollbackErr != nil {
				log.Printf("[ERROR] Rollback failed: %v", rollbackErr)
			}
			return fmt.Errorf("plugin %s: %w", namespace, err)
		}

		// Track this config as successfully applied
		s.state.SetLastApplied(namespace, config)
		log.Printf("[OK] Applied %s configuration", namespace)
	}
	appliedSteps = append(appliedSteps, "plugins")

	// Step 4: Apply static routes
	var routesConfig types.RoutesConfig
	if err := state.LoadConfig("routes", &routesConfig); err != nil {
		log.Printf("[WARN] No routes config found: %v", err)
	} else {
		if err := system.ApplyRoutesConfig(&routesConfig); err != nil {
			// Full rollback
			log.Printf("[ERROR] Routes failed: %v, rolling back", err)
			s.rollbackPlugins(snapshot)
			if rollbackErr := system.RestoreSnapshot(snapshot, []string{"all"}); rollbackErr != nil {
				log.Printf("[ERROR] Rollback failed: %v", rollbackErr)
			}
			return fmt.Errorf("routes: %w", err)
		}
	}
	appliedSteps = append(appliedSteps, "routes")

	log.Printf("[INFO] Apply completed successfully")
	return nil
}

// rollbackPlugins rolls back all plugins to their snapshot state.
func (s *Server) rollbackPlugins(snapshot *system.SystemSnapshot) {
	log.Printf("[INFO] Rolling back plugins")

	for _, namespace := range s.registry.List() {
		plugin, _ := s.registry.Get(namespace)

		// Flush current state
		if err := plugin.Flush(); err != nil {
			log.Printf("[WARN] Failed to flush plugin %s: %v", namespace, err)
		}

		// Get plugin name
		pluginName, found := s.registry.GetPluginNameForNamespace(namespace)
		if !found {
			continue
		}

		// Try to re-apply old config from LastApplied
		if oldConfig, err := s.state.GetLastApplied(namespace); err == nil && oldConfig != nil {
			log.Printf("[INFO] Restoring %s to last known good config", namespace)
			if err := plugin.ApplyConfig(oldConfig); err != nil {
				log.Printf("[WARN] Failed to restore plugin %s: %v", namespace, err)
			}
		} else {
			// Try loading from file as fallback
			var config map[string]interface{}
			if err := state.LoadConfig(pluginName, &config); err == nil {
				log.Printf("[INFO] Restoring %s from config file", namespace)
				if err := plugin.ApplyConfig(config); err != nil {
					log.Printf("[WARN] Failed to restore plugin %s: %v", namespace, err)
				}
			}
		}
	}
}

func (s *Server) handleShow(path string) Response {
	// If no path specified, return all configs
	if path == "" {
		allConfigs := map[string]interface{}{
			"interfaces": s.state.GetCurrentInterfaces(),
			"routes":     s.state.GetCurrentRoutes(),
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
	// If no path provided, return list of available namespaces grouped by category
	if path == "" {
		categories := make(map[string][]string)

		// Add core namespaces to "core" category
		categories["core"] = []string{"interfaces", "routes"}

		// Add plugin namespaces grouped by their categories
		pluginCategories := s.registry.ListByCategory()
		for category, namespaces := range pluginCategories {
			categories[category] = namespaces
		}

		return Response{
			Success: true,
			Data:    categories,
			Message: "Available configuration namespaces grouped by category",
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

	// Convert routes map to slice for API compatibility when getting entire routes collection
	if path == "routes" {
		if routesMap, ok := value.(map[string]types.Route); ok {
			routesSlice := make([]types.Route, 0, len(routesMap))
			for _, route := range routesMap {
				routesSlice = append(routesSlice, route)
			}
			value = routesSlice
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
		// Try to convert to routes structure (slice of Route)
		var routesSlice []types.Route
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return Response{
				Success: false,
				Error:   fmt.Sprintf("failed to marshal routes: %v", err),
			}
		}
		if err := json.Unmarshal(jsonBytes, &routesSlice); err != nil {
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
		log.Printf("[ERROR] Failed to marshal response: %v", err)
		return
	}

	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		log.Printf("[ERROR] Failed to write response: %v", err)
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

	// Load and apply config if it exists (use plugin name for config file)
	var pluginConfig interface{}
	if err := state.LoadConfig(pluginName, &pluginConfig); err == nil {
		if err := plugin.ApplyConfig(pluginConfig); err != nil {
			log.Printf("[WARN] Failed to apply config from %s.json for plugin '%s': %v", pluginName, pluginName, err)
		} else {
			log.Printf("[INFO] Applied config from %s.json for plugin '%s'", pluginName, pluginName)
		}
	} else {
		// No config file exists - use default config from plugin metadata
		if metadata.DefaultConfig != nil {
			log.Printf("[INFO] No %s.json config file found for plugin '%s', using default config", pluginName, pluginName)
			if err := plugin.ApplyConfig(metadata.DefaultConfig); err != nil {
				log.Printf("[WARN] Failed to apply default config for plugin '%s': %v", pluginName, err)
			} else {
				log.Printf("[INFO] Applied default config for plugin '%s'", pluginName)
			}
		} else {
			log.Printf("[INFO] No %s.json config file found for plugin '%s' and no default config available", pluginName, pluginName)
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

	// Unregister from registry
	s.registry.Unregister(namespace)

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

	log.Printf("[INFO] Rolling back to checkpoint: %s", checkpointID)

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

	// Restore nftables rules if present
	if snapshot.NftablesRules != "" {
		if err := system.RestoreNftablesRules(snapshot.NftablesRules); err != nil {
			log.Printf("[WARN] Failed to restore nftables rules: %v", err)
		}
	}

	log.Printf("[INFO] Successfully rolled back to checkpoint: %s", checkpointID)

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
	log.Printf("[INFO] Creating manual checkpoint")

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

	log.Printf("[INFO] Created checkpoint: %s", snapshot.CheckpointID)

	return Response{
		Success: true,
		Message: fmt.Sprintf("Created checkpoint: %s", snapshot.CheckpointID),
		Data:    snapshot.CheckpointID,
	}
}
