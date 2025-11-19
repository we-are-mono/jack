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

package daemon

import (
	"context"
	"fmt"
	"sync"

	"github.com/we-are-mono/jack/daemon/logger"
	"github.com/we-are-mono/jack/plugins"
)

// ServiceRegistry manages plugin services and allows cross-plugin service calls
type ServiceRegistry struct {
	mu sync.RWMutex

	// serviceName -> provider plugin namespace
	serviceProviders map[string]string

	// pluginNamespace -> []serviceName (services provided by this plugin)
	pluginServices map[string][]string

	// pluginNamespace -> Plugin
	plugins map[string]plugins.Plugin

	// serviceName -> ready state
	serviceReady map[string]bool

	// serviceName -> channel that closes when service becomes ready
	readyChannels map[string]chan struct{}
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		serviceProviders: make(map[string]string),
		pluginServices:   make(map[string][]string),
		plugins:          make(map[string]plugins.Plugin),
		serviceReady:     make(map[string]bool),
		readyChannels:    make(map[string]chan struct{}),
	}
}

// RegisterPlugin registers a plugin and its services
func (r *ServiceRegistry) RegisterPlugin(namespace string, plugin plugins.Plugin, services []plugins.ServiceDescriptor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Store plugin reference
	r.plugins[namespace] = plugin

	// Register each service
	for _, service := range services {
		// Check if service is already provided by another plugin
		if existingProvider, exists := r.serviceProviders[service.Name]; exists {
			return fmt.Errorf("service '%s' is already provided by plugin '%s'", service.Name, existingProvider)
		}

		// Register service
		r.serviceProviders[service.Name] = namespace
		r.pluginServices[namespace] = append(r.pluginServices[namespace], service.Name)

		// Initialize ready state (not ready yet)
		r.serviceReady[service.Name] = false
		r.readyChannels[service.Name] = make(chan struct{})

		logger.Info("Registered service",
			logger.Field{Key: "service", Value: service.Name},
			logger.Field{Key: "provider", Value: namespace},
			logger.Field{Key: "methods", Value: fmt.Sprintf("%v", service.Methods)})
	}

	return nil
}

// UnregisterPlugin removes a plugin and its services
func (r *ServiceRegistry) UnregisterPlugin(namespace string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Remove all services provided by this plugin
	services := r.pluginServices[namespace]
	for _, serviceName := range services {
		delete(r.serviceProviders, serviceName)
		delete(r.serviceReady, serviceName)

		// Close ready channel if it exists
		if ch, exists := r.readyChannels[serviceName]; exists {
			close(ch)
			delete(r.readyChannels, serviceName)
		}
	}

	// Remove plugin
	delete(r.pluginServices, namespace)
	delete(r.plugins, namespace)
}

// GetServiceProvider returns the namespace of the plugin providing the given service
func (r *ServiceRegistry) GetServiceProvider(serviceName string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.serviceProviders[serviceName]
	return provider, exists
}

// CallService calls a service method on a provider plugin
func (r *ServiceRegistry) CallService(ctx context.Context, serviceName string, method string, argsJSON []byte) ([]byte, error) {
	// Find provider
	r.mu.RLock()
	providerNamespace, exists := r.serviceProviders[serviceName]
	if !exists {
		r.mu.RUnlock()
		return nil, fmt.Errorf("service '%s' not found", serviceName)
	}

	plugin, pluginExists := r.plugins[providerNamespace]
	r.mu.RUnlock()

	if !pluginExists {
		return nil, fmt.Errorf("provider plugin '%s' for service '%s' not found", providerNamespace, serviceName)
	}

	// Get the provider (the plugin implements Provider interface)
	provider, ok := plugin.(*PluginLoader)
	if !ok {
		return nil, fmt.Errorf("plugin '%s' does not support service calls", providerNamespace)
	}

	// Call service via RPC
	return provider.provider.CallService(ctx, serviceName, method, argsJSON)
}

// ValidateServiceDependencies checks if all required services are available
func (r *ServiceRegistry) ValidateServiceDependencies(requiredServices []string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, serviceName := range requiredServices {
		if _, exists := r.serviceProviders[serviceName]; !exists {
			return fmt.Errorf("required service '%s' is not available", serviceName)
		}
	}

	return nil
}

// ListServices returns all available services
func (r *ServiceRegistry) ListServices() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]string, len(r.serviceProviders))
	for service, provider := range r.serviceProviders {
		result[service] = provider
	}

	return result
}

// MarkServiceReady marks a service as ready for use
// This should be called by service providers after initialization is complete
func (r *ServiceRegistry) MarkServiceReady(serviceName string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.serviceProviders[serviceName]; !exists {
		return // Service doesn't exist
	}

	if !r.serviceReady[serviceName] {
		r.serviceReady[serviceName] = true
		// Close the channel to signal all waiters
		close(r.readyChannels[serviceName])
		logger.Info("Service is ready",
			logger.Field{Key: "service", Value: serviceName})
	}
}

// WaitForService waits for a service to become ready
// Returns error if context is canceled or service doesn't exist
func (r *ServiceRegistry) WaitForService(ctx context.Context, serviceName string) error {
	r.mu.RLock()
	readyCh, exists := r.readyChannels[serviceName]
	isReady := r.serviceReady[serviceName]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("service '%s' not found", serviceName)
	}

	// If already ready, return immediately
	if isReady {
		return nil
	}

	// Wait for ready signal or context cancellation
	select {
	case <-readyCh:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for service '%s': %w", serviceName, ctx.Err())
	}
}

// WaitForServices waits for multiple services to become ready
func (r *ServiceRegistry) WaitForServices(ctx context.Context, serviceNames []string) error {
	for _, serviceName := range serviceNames {
		if err := r.WaitForService(ctx, serviceName); err != nil {
			return err
		}
	}
	return nil
}

// IsServiceReady checks if a service is ready without waiting
// Returns false if service doesn't exist
func (r *ServiceRegistry) IsServiceReady(serviceName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.serviceReady[serviceName]
}

// AreServicesReady checks if all given services are ready
func (r *ServiceRegistry) AreServicesReady(serviceNames []string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, serviceName := range serviceNames {
		if !r.serviceReady[serviceName] {
			return false
		}
	}
	return true
}

// DaemonServiceImpl implements plugins.DaemonService for bidirectional RPC
// This allows plugins to call services on other plugins through the daemon
type DaemonServiceImpl struct {
	registry *ServiceRegistry
}

// NewDaemonServiceImpl creates a new daemon service implementation
func NewDaemonServiceImpl(registry *ServiceRegistry) *DaemonServiceImpl {
	return &DaemonServiceImpl{registry: registry}
}

// Ping verifies the daemon service connection is responsive
func (d *DaemonServiceImpl) Ping(ctx context.Context) error {
	// Simply return nil - if we can execute this method, the RPC connection is working
	return nil
}

// CallService routes a service call from a plugin to another plugin's service
func (d *DaemonServiceImpl) CallService(ctx context.Context, serviceName string, method string, argsJSON []byte) ([]byte, error) {
	return d.registry.CallService(ctx, serviceName, method, argsJSON)
}
