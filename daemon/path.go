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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/we-are-mono/jack/types"
)

// ParseConfigType extracts the config type from a path
func ParseConfigType(path string) (string, error) {
	parts := strings.Split(path, ".")
	if len(parts) < 1 {
		return "", fmt.Errorf("invalid path: %s", path)
	}

	configType := parts[0]
	if configType == "" {
		return "", fmt.Errorf("invalid path: %s", path)
	}

	return configType, nil
}

// SetValue sets a value in any config type
func SetValue(config interface{}, path string, value interface{}) error {
	parts := strings.Split(path, ".")

	if len(parts) < 1 {
		return fmt.Errorf("invalid path: %s", path)
	}

	configType := parts[0]

	// Special handling for core types with complex structures
	switch configType {
	case "interfaces":
		interfacesConfig, ok := config.(*types.InterfacesConfig)
		if !ok {
			return fmt.Errorf("config is not InterfacesConfig")
		}
		return setInterfacesValue(interfacesConfig, parts, value)
	case "routes":
		routesConfig, ok := config.(*types.RoutesConfig)
		if !ok {
			return fmt.Errorf("config is not RoutesConfig")
		}
		return setRoutesValue(routesConfig, parts, value)
	default:
		// Generic path navigation for plugin configs (map[string]interface{})
		configMap, ok := config.(map[string]interface{})
		if !ok {
			return fmt.Errorf("config must be a map for generic path access")
		}

		// If path is just the config type (e.g., "vpn"), replace entire config
		if len(parts) == 1 {
			// Replace entire config map with new value
			valueMap, ok := value.(map[string]interface{})
			if !ok {
				return fmt.Errorf("value must be a map when setting entire config")
			}
			// Clear existing map and copy new values
			for k := range configMap {
				delete(configMap, k)
			}
			for k, v := range valueMap {
				configMap[k] = v
			}
			return nil
		}

		return setGenericValue(configMap, parts[1:], value)
	}
}

// GetValue gets a value from any config type
func GetValue(config interface{}, path string) (interface{}, error) {
	parts := strings.Split(path, ".")

	if len(parts) < 1 {
		return nil, fmt.Errorf("invalid path: %s", path)
	}

	configType := parts[0]

	// Special handling for core types with complex structures
	switch configType {
	case "interfaces":
		interfacesConfig, ok := config.(*types.InterfacesConfig)
		if !ok {
			return nil, fmt.Errorf("config is not InterfacesConfig")
		}
		return getInterfacesValue(interfacesConfig, parts)
	case "routes":
		routesConfig, ok := config.(*types.RoutesConfig)
		if !ok {
			return nil, fmt.Errorf("config is not RoutesConfig")
		}
		return getRoutesValue(routesConfig, parts)
	default:
		// Generic path navigation for plugin configs (map[string]interface{})
		configMap, ok := config.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("config must be a map for generic path access")
		}
		return getGenericValue(configMap, parts[1:])
	}
}

// setGenericValue sets a value in a generic map[string]interface{} using path navigation
func setGenericValue(config map[string]interface{}, path []string, value interface{}) error {
	if len(path) == 0 {
		return fmt.Errorf("empty path")
	}

	if len(path) == 1 {
		// Leaf node - set the value
		config[path[0]] = value
		return nil
	}

	// Navigate deeper
	key := path[0]
	next, exists := config[key]
	if !exists {
		return fmt.Errorf("key '%s' not found", key)
	}

	nextMap, ok := next.(map[string]interface{})
	if !ok {
		return fmt.Errorf("cannot navigate into non-map value at '%s'", key)
	}

	return setGenericValue(nextMap, path[1:], value)
}

// getGenericValue gets a value from a generic map[string]interface{} using path navigation
func getGenericValue(config map[string]interface{}, path []string) (interface{}, error) {
	if len(path) == 0 {
		return config, nil
	}

	key := path[0]
	value, exists := config[key]
	if !exists {
		return nil, fmt.Errorf("key '%s' not found", key)
	}

	if len(path) == 1 {
		// Leaf node - return the value
		return value, nil
	}

	// Navigate deeper
	nextMap, ok := value.(map[string]interface{})
	if !ok {
		// Could be an array or primitive - return it as is
		return value, nil
	}

	return getGenericValue(nextMap, path[1:])
}

func setInterfacesValue(config *types.InterfacesConfig, parts []string, value interface{}) error {
	// Handle setting entire interfaces map
	if len(parts) == 1 && parts[0] == "interfaces" {
		// Try direct type assertion first
		interfacesMap, ok := value.(map[string]types.Interface)
		if !ok {
			// Value came from JSON unmarshaling, need to convert it
			// Marshal and unmarshal to convert map[string]interface{} to map[string]types.Interface
			jsonBytes, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("failed to marshal interfaces value: %w", err)
			}
			if err := json.Unmarshal(jsonBytes, &interfacesMap); err != nil {
				return fmt.Errorf("value must be map[string]types.Interface when setting interfaces: %w", err)
			}
		}
		config.Interfaces = interfacesMap
		return nil
	}

	if len(parts) < 2 {
		return fmt.Errorf("invalid path for interfaces")
	}

	ifaceName := parts[1]

	// Handle setting entire interface struct
	if len(parts) == 2 {
		// Try direct type assertion first
		ifaceStruct, ok := value.(types.Interface)
		if !ok {
			// Value came from JSON unmarshaling, need to convert it
			jsonBytes, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("failed to marshal interface value: %w", err)
			}
			if err := json.Unmarshal(jsonBytes, &ifaceStruct); err != nil {
				return fmt.Errorf("value must be types.Interface when setting interface: %w", err)
			}
		}
		config.Interfaces[ifaceName] = ifaceStruct
		return nil
	}

	// Handle setting individual fields
	if len(parts) < 3 {
		return fmt.Errorf("invalid path for interfaces: need at least interfaces.name.field")
	}

	iface, exists := config.Interfaces[ifaceName]
	if !exists {
		return fmt.Errorf("interface '%s' not found", ifaceName)
	}

	if err := setInterfaceField(&iface, parts[2:], value); err != nil {
		return err
	}

	config.Interfaces[ifaceName] = iface
	return nil
}

func getInterfacesValue(config *types.InterfacesConfig, parts []string) (interface{}, error) {
	// If path is just "interfaces", return the entire interfaces map
	if len(parts) == 1 && parts[0] == "interfaces" {
		return config.Interfaces, nil
	}

	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid path for interfaces")
	}

	ifaceName := parts[1]
	iface, exists := config.Interfaces[ifaceName]
	if !exists {
		return nil, fmt.Errorf("interface '%s' not found", ifaceName)
	}

	if len(parts) == 2 {
		return iface, nil
	}

	return getInterfaceField(iface, parts[2:])
}

func setInterfaceField(iface *types.Interface, parts []string, value interface{}) error {
	if len(parts) > 1 {
		return fmt.Errorf("nested paths not yet supported")
	}

	field := parts[0]
	return setStructField(iface, field, value)
}

func getInterfaceField(iface types.Interface, parts []string) (interface{}, error) {
	if len(parts) > 1 {
		return nil, fmt.Errorf("nested paths not yet supported")
	}

	field := parts[0]
	return getStructField(iface, field)
}

// Routes config get/set operations
func getRoutesValue(config *types.RoutesConfig, parts []string) (interface{}, error) {
	// If path is just "routes", return the entire routes map
	if len(parts) == 1 && parts[0] == "routes" {
		return config.Routes, nil
	}

	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid path for routes")
	}

	// parts[1] is the route name (e.g., routes.default-via-vpn.enabled)
	routeName := parts[1]

	// Direct map access
	route, exists := config.Routes[routeName]
	if !exists {
		return nil, fmt.Errorf("route '%s' not found", routeName)
	}

	if len(parts) == 2 {
		return route, nil
	}
	return getRoute(&route, parts[2:])
}

func getRoute(route *types.Route, parts []string) (interface{}, error) {
	if len(parts) == 0 {
		return route, nil
	}

	field := parts[0]
	return getStructField(route, field)
}

func setRoutesValue(config *types.RoutesConfig, parts []string, value interface{}) error {
	// Handle setting entire routes map
	if len(parts) == 1 && parts[0] == "routes" {
		// Try direct type assertion first
		routesMap, ok := value.(map[string]types.Route)
		if !ok {
			// Check if it's a slice (some tests send []types.Route)
			if routesSlice, ok := value.([]types.Route); ok {
				// Convert slice to map using route names or indices
				routesMap = make(map[string]types.Route)
				for i, route := range routesSlice {
					// Use route name if available, otherwise use index
					key := route.Name
					if key == "" {
						key = fmt.Sprintf("route-%d", i)
					}
					routesMap[key] = route
				}
			} else {
				// Value came from JSON unmarshaling, need to convert it
				// First try to unmarshal as map
				jsonBytes, err := json.Marshal(value)
				if err != nil {
					return fmt.Errorf("failed to marshal routes value: %w", err)
				}

				// Try unmarshaling as map first
				if err := json.Unmarshal(jsonBytes, &routesMap); err != nil {
					// Try as slice
					var routesSlice []types.Route
					if err2 := json.Unmarshal(jsonBytes, &routesSlice); err2 != nil {
						return fmt.Errorf("value must be map[string]types.Route or []types.Route when setting routes: %w", err)
					}
					// Convert slice to map
					routesMap = make(map[string]types.Route)
					for i, route := range routesSlice {
						key := route.Name
						if key == "" {
							key = fmt.Sprintf("route-%d", i)
						}
						routesMap[key] = route
					}
				}
			}
		}
		config.Routes = routesMap
		return nil
	}

	if len(parts) < 2 {
		return fmt.Errorf("invalid path for routes")
	}

	// parts[1] is the route name (e.g., routes.default-via-vpn.enabled)
	routeName := parts[1]

	// Handle setting entire route struct
	if len(parts) == 2 {
		// Try direct type assertion first
		routeStruct, ok := value.(types.Route)
		if !ok {
			// Value came from JSON unmarshaling, need to convert it
			jsonBytes, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("failed to marshal route value: %w", err)
			}
			if err := json.Unmarshal(jsonBytes, &routeStruct); err != nil {
				return fmt.Errorf("value must be types.Route when setting route: %w", err)
			}
		}
		config.Routes[routeName] = routeStruct
		return nil
	}

	// Handle setting individual fields
	if len(parts) < 3 {
		return fmt.Errorf("need to specify route name and field (e.g., routes.vpn-endpoint.enabled)")
	}

	// Get route from map
	route, exists := config.Routes[routeName]
	if !exists {
		// Create new route if it doesn't exist
		route = types.Route{
			Enabled: false,
		}
	}

	// Set the field
	if err := setRoute(&route, parts[2:], value); err != nil {
		return err
	}

	// Store back to map
	config.Routes[routeName] = route
	return nil
}

func setRoute(route *types.Route, parts []string, value interface{}) error {
	if len(parts) == 0 {
		return fmt.Errorf("need to specify route field")
	}

	field := parts[0]

	// Special check for read-only "name" field
	if field == "name" {
		return fmt.Errorf("unknown route field: %s (read-only field: name)", field)
	}

	return setStructField(route, field, value)
}
