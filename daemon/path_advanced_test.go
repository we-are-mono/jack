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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/we-are-mono/jack/types"
)

// TestSetValue_EntireInterfacesMap tests setting the entire interfaces map
func TestSetValue_EntireInterfacesMap(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		wantError bool
		check     func(*testing.T, *types.InterfacesConfig)
	}{
		{
			name: "set entire map - direct type",
			value: map[string]types.Interface{
				"eth0": {
					Type:     "physical",
					Enabled:  true,
					Protocol: "static",
					IPAddr:   "10.0.1.1",
					Netmask:  "255.255.255.0",
				},
				"wlan0": {
					Type:     "wireless",
					Enabled:  false,
					Protocol: "dhcp",
				},
			},
			wantError: false,
			check: func(t *testing.T, c *types.InterfacesConfig) {
				assert.Len(t, c.Interfaces, 2)
				assert.Equal(t, "physical", c.Interfaces["eth0"].Type)
				assert.Equal(t, "wireless", c.Interfaces["wlan0"].Type)
			},
		},
		{
			name: "set entire map - from JSON (map[string]interface{})",
			value: map[string]interface{}{
				"br-lan": map[string]interface{}{
					"type":     "bridge",
					"enabled":  true,
					"protocol": "static",
					"ipaddr":   "192.168.1.1",
					"netmask":  "255.255.255.0",
				},
			},
			wantError: false,
			check: func(t *testing.T, c *types.InterfacesConfig) {
				assert.Len(t, c.Interfaces, 1)
				assert.Equal(t, "bridge", c.Interfaces["br-lan"].Type)
				assert.Equal(t, "192.168.1.1", c.Interfaces["br-lan"].IPAddr)
			},
		},
		{
			name: "set entire map - invalid value",
			value: "not-a-map",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &types.InterfacesConfig{
				Interfaces: make(map[string]types.Interface),
			}

			err := SetValue(config, "interfaces", tt.value)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.check != nil {
					tt.check(t, config)
				}
			}
		})
	}
}

// TestSetValue_EntireInterfaceStruct tests setting an entire interface struct
func TestSetValue_EntireInterfaceStruct(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		value     interface{}
		wantError bool
		check     func(*testing.T, *types.InterfacesConfig)
	}{
		{
			name: "set entire interface - direct type",
			path: "interfaces.eth1",
			value: types.Interface{
				Type:     "physical",
				Enabled:  true,
				Protocol: "static",
				IPAddr:   "10.0.2.1",
				Netmask:  "255.255.255.0",
				MTU:      1500,
			},
			wantError: false,
			check: func(t *testing.T, c *types.InterfacesConfig) {
				assert.Equal(t, "physical", c.Interfaces["eth1"].Type)
				assert.Equal(t, "10.0.2.1", c.Interfaces["eth1"].IPAddr)
				assert.Equal(t, 1500, c.Interfaces["eth1"].MTU)
			},
		},
		{
			name: "set entire interface - from JSON (map[string]interface{})",
			path: "interfaces.wg0",
			value: map[string]interface{}{
				"type":     "wireguard",
				"enabled":  true,
				"protocol": "none",
				"mtu":      float64(1420),
			},
			wantError: false,
			check: func(t *testing.T, c *types.InterfacesConfig) {
				assert.Equal(t, "wireguard", c.Interfaces["wg0"].Type)
				assert.Equal(t, 1420, c.Interfaces["wg0"].MTU)
			},
		},
		{
			name:      "set entire interface - invalid value",
			path:      "interfaces.invalid",
			value:     "not-an-interface",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &types.InterfacesConfig{
				Interfaces: make(map[string]types.Interface),
			}

			err := SetValue(config, tt.path, tt.value)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.check != nil {
					tt.check(t, config)
				}
			}
		})
	}
}

// TestSetValue_EntireRoutesMap tests setting the entire routes map
func TestSetValue_EntireRoutesMap(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		wantError bool
		check     func(*testing.T, *types.RoutesConfig)
	}{
		{
			name: "set entire map - direct type",
			value: map[string]types.Route{
				"default": {
					Name:        "default",
					Destination: "0.0.0.0/0",
					Gateway:     "10.0.0.1",
					Interface:   "eth0",
					Enabled:     true,
					Metric:      100,
				},
				"vpn": {
					Name:        "vpn",
					Destination: "10.8.0.0/24",
					Interface:   "wg0",
					Enabled:     true,
				},
			},
			wantError: false,
			check: func(t *testing.T, c *types.RoutesConfig) {
				assert.Len(t, c.Routes, 2)
				assert.Equal(t, "0.0.0.0/0", c.Routes["default"].Destination)
				assert.Equal(t, "10.8.0.0/24", c.Routes["vpn"].Destination)
			},
		},
		{
			name: "set entire map - from slice (direct type)",
			value: []types.Route{
				{
					Name:        "route-1",
					Destination: "192.168.1.0/24",
					Gateway:     "10.0.0.254",
					Enabled:     true,
				},
				{
					Name:        "route-2",
					Destination: "192.168.2.0/24",
					Gateway:     "10.0.0.253",
					Enabled:     true,
				},
			},
			wantError: false,
			check: func(t *testing.T, c *types.RoutesConfig) {
				assert.Len(t, c.Routes, 2)
				assert.Equal(t, "192.168.1.0/24", c.Routes["route-1"].Destination)
				assert.Equal(t, "192.168.2.0/24", c.Routes["route-2"].Destination)
			},
		},
		{
			name: "set entire map - from slice without names",
			value: []types.Route{
				{
					Destination: "10.10.0.0/24",
					Gateway:     "10.0.0.1",
					Enabled:     true,
				},
			},
			wantError: false,
			check: func(t *testing.T, c *types.RoutesConfig) {
				assert.Len(t, c.Routes, 1)
				// Should use auto-generated key "route-0"
				assert.NotNil(t, c.Routes["route-0"])
				assert.Equal(t, "10.10.0.0/24", c.Routes["route-0"].Destination)
			},
		},
		{
			name: "set entire map - from JSON map",
			value: map[string]interface{}{
				"custom": map[string]interface{}{
					"destination": "172.16.0.0/16",
					"gateway":     "10.0.0.100",
					"enabled":     true,
					"metric":      float64(200),
				},
			},
			wantError: false,
			check: func(t *testing.T, c *types.RoutesConfig) {
				assert.Len(t, c.Routes, 1)
				assert.Equal(t, "172.16.0.0/16", c.Routes["custom"].Destination)
				assert.Equal(t, 200, c.Routes["custom"].Metric)
			},
		},
		{
			name: "set entire map - from JSON slice",
			value: []interface{}{
				map[string]interface{}{
					"name":        "json-route",
					"destination": "192.168.100.0/24",
					"gateway":     "10.0.0.1",
					"enabled":     true,
				},
			},
			wantError: false,
			check: func(t *testing.T, c *types.RoutesConfig) {
				assert.Len(t, c.Routes, 1)
				assert.Equal(t, "192.168.100.0/24", c.Routes["json-route"].Destination)
			},
		},
		{
			name:      "set entire map - invalid value",
			value:     "not-a-map",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &types.RoutesConfig{
				Routes: make(map[string]types.Route),
			}

			err := SetValue(config, "routes", tt.value)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.check != nil {
					tt.check(t, config)
				}
			}
		})
	}
}

// TestSetValue_EntireRouteStruct tests setting an entire route struct
func TestSetValue_EntireRouteStruct(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		value     interface{}
		wantError bool
		check     func(*testing.T, *types.RoutesConfig)
	}{
		{
			name: "set entire route - direct type",
			path: "routes.my-route",
			value: types.Route{
				Name:        "my-route",
				Destination: "10.20.0.0/16",
				Gateway:     "10.0.0.50",
				Interface:   "eth1",
				Enabled:     true,
				Metric:      150,
				Table:       200,
			},
			wantError: false,
			check: func(t *testing.T, c *types.RoutesConfig) {
				assert.Equal(t, "10.20.0.0/16", c.Routes["my-route"].Destination)
				assert.Equal(t, 150, c.Routes["my-route"].Metric)
				assert.Equal(t, 200, c.Routes["my-route"].Table)
			},
		},
		{
			name: "set entire route - from JSON (map[string]interface{})",
			path: "routes.json-route",
			value: map[string]interface{}{
				"destination": "192.168.50.0/24",
				"gateway":     "10.0.0.1",
				"interface":   "wg0",
				"enabled":     true,
				"metric":      float64(300),
				"table":       float64(100),
			},
			wantError: false,
			check: func(t *testing.T, c *types.RoutesConfig) {
				assert.Equal(t, "192.168.50.0/24", c.Routes["json-route"].Destination)
				assert.Equal(t, 300, c.Routes["json-route"].Metric)
				assert.Equal(t, 100, c.Routes["json-route"].Table)
			},
		},
		{
			name:      "set entire route - invalid value",
			path:      "routes.invalid",
			value:     "not-a-route",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &types.RoutesConfig{
				Routes: make(map[string]types.Route),
			}

			err := SetValue(config, tt.path, tt.value)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.check != nil {
					tt.check(t, config)
				}
			}
		})
	}
}

// TestSetValue_CreateNewRoute tests creating a new route that doesn't exist
func TestSetValue_CreateNewRoute(t *testing.T) {
	config := &types.RoutesConfig{
		Routes: map[string]types.Route{
			"existing": {
				Destination: "10.0.0.0/24",
				Gateway:     "10.0.0.1",
				Enabled:     true,
			},
		},
	}

	// Set field on non-existent route - should create it
	err := SetValue(config, "routes.new-route.destination", "192.168.1.0/24")
	require.NoError(t, err)

	// Verify route was created
	assert.Contains(t, config.Routes, "new-route")
	assert.Equal(t, "192.168.1.0/24", config.Routes["new-route"].Destination)
	assert.False(t, config.Routes["new-route"].Enabled) // Default value

	// Set another field on the new route
	err = SetValue(config, "routes.new-route.enabled", true)
	require.NoError(t, err)
	assert.True(t, config.Routes["new-route"].Enabled)
}

// TestSetValue_NestedPathErrors tests nested path error handling
func TestSetValue_NestedPathErrors(t *testing.T) {
	interfaceConfig := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"eth0": {
				Type:    "physical",
				Enabled: true,
			},
		},
	}

	// Nested paths not supported for interface fields
	err := SetValue(interfaceConfig, "interfaces.eth0.nested.path", "value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nested paths not yet supported")
}

// TestSetValue_InterfaceFieldEdgeCases tests edge cases for interface field setting
func TestSetValue_InterfaceFieldEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		value     interface{}
		wantError bool
		errorMsg  string
	}{
		{
			name:      "protocol - wrong type",
			field:     "protocol",
			value:     123,
			wantError: true,
			errorMsg:  "protocol must be a string",
		},
		{
			name:      "ipaddr - wrong type",
			field:     "ipaddr",
			value:     true,
			wantError: true,
			errorMsg:  "ipaddr must be a string",
		},
		{
			name:      "netmask - wrong type",
			field:     "netmask",
			value:     []string{"invalid"},
			wantError: true,
			errorMsg:  "netmask must be a string",
		},
		{
			name:      "gateway - wrong type",
			field:     "gateway",
			value:     123.456,
			wantError: true,
			errorMsg:  "gateway must be a string",
		},
		{
			name:      "comment - wrong type",
			field:     "comment",
			value:     123,
			wantError: true,
			errorMsg:  "comment must be a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &types.InterfacesConfig{
				Interfaces: map[string]types.Interface{
					"test": {},
				},
			}

			err := SetValue(config, "interfaces.test."+tt.field, tt.value)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestSetValue_RouteFieldEdgeCases tests edge cases for route field setting
func TestSetValue_RouteFieldEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		field     string
		value     interface{}
		wantError bool
		errorMsg  string
	}{
		{
			name:      "destination - wrong type",
			field:     "destination",
			value:     123,
			wantError: true,
			errorMsg:  "destination must be a string",
		},
		{
			name:      "gateway - wrong type",
			field:     "gateway",
			value:     true,
			wantError: true,
			errorMsg:  "gateway must be a string",
		},
		{
			name:      "interface - wrong type",
			field:     "interface",
			value:     []string{"invalid"},
			wantError: true,
			errorMsg:  "interface must be a string",
		},
		{
			name:      "metric - wrong type",
			field:     "metric",
			value:     "not-a-number",
			wantError: true,
			errorMsg:  "metric must be a number",
		},
		{
			name:      "table - wrong type",
			field:     "table",
			value:     "not-a-number",
			wantError: true,
			errorMsg:  "table must be a number",
		},
		{
			name:      "enabled - wrong type",
			field:     "enabled",
			value:     "not-a-bool",
			wantError: true,
			errorMsg:  "enabled must be a boolean",
		},
		{
			name:      "comment - wrong type",
			field:     "comment",
			value:     123,
			wantError: true,
			errorMsg:  "comment must be a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &types.RoutesConfig{
				Routes: map[string]types.Route{
					"test": {},
				},
			}

			err := SetValue(config, "routes.test."+tt.field, tt.value)

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestSetValue_InvalidPaths tests various invalid path scenarios
func TestSetValue_InvalidPaths(t *testing.T) {
	tests := []struct {
		name      string
		config    interface{}
		path      string
		value     interface{}
		errorMsg  string
	}{
		{
			name:     "interfaces - path too short",
			config:   &types.InterfacesConfig{Interfaces: make(map[string]types.Interface)},
			path:     "interfaces",
			value:    "invalid",
			errorMsg: "value must be map[string]types.Interface when setting interfaces",
		},
		{
			name:     "routes - path too short for field",
			config:   &types.RoutesConfig{Routes: make(map[string]types.Route)},
			path:     "routes",
			value:    "invalid",
			errorMsg: "value must be map[string]types.Route or []types.Route when setting routes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetValue(tt.config, tt.path, tt.value)
			assert.Error(t, err)
			if tt.errorMsg != "" {
				assert.Contains(t, err.Error(), tt.errorMsg)
			}
		})
	}
}
