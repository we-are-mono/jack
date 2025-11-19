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

func TestParseConfigType(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		want      string
		wantError bool
	}{
		{
			name:      "simple config type",
			path:      "interfaces",
			want:      "interfaces",
			wantError: false,
		},
		{
			name:      "nested path",
			path:      "interfaces.br-lan.enabled",
			want:      "interfaces",
			wantError: false,
		},
		{
			name:      "plugin config",
			path:      "firewall.zones.wan",
			want:      "firewall",
			wantError: false,
		},
		{
			name:      "routes config",
			path:      "routes.default.gateway",
			want:      "routes",
			wantError: false,
		},
		{
			name:      "empty path",
			path:      "",
			want:      "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseConfigType(tt.path)
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestSetValue_Interfaces(t *testing.T) {
	config := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"br-lan": {
				Type:     "bridge",
				Enabled:  true,
				Protocol: "static",
				IPAddr:   "10.0.0.1",
				Netmask:  "255.255.255.0",
				MTU:      1500,
			},
		},
	}

	tests := []struct {
		name      string
		path      string
		value     interface{}
		wantError bool
		check     func(*testing.T, *types.InterfacesConfig)
	}{
		{
			name:      "set enabled",
			path:      "interfaces.br-lan.enabled",
			value:     false,
			wantError: false,
			check: func(t *testing.T, c *types.InterfacesConfig) {
				if c.Interfaces["br-lan"].Enabled != false {
					t.Errorf("enabled not set correctly")
				}
			},
		},
		{
			name:      "set protocol",
			path:      "interfaces.br-lan.protocol",
			value:     "dhcp",
			wantError: false,
			check: func(t *testing.T, c *types.InterfacesConfig) {
				if c.Interfaces["br-lan"].Protocol != "dhcp" {
					t.Errorf("protocol not set correctly")
				}
			},
		},
		{
			name:      "set ipaddr",
			path:      "interfaces.br-lan.ipaddr",
			value:     "192.168.1.1",
			wantError: false,
			check: func(t *testing.T, c *types.InterfacesConfig) {
				if c.Interfaces["br-lan"].IPAddr != "192.168.1.1" {
					t.Errorf("ipaddr not set correctly")
				}
			},
		},
		{
			name:      "set netmask",
			path:      "interfaces.br-lan.netmask",
			value:     "255.255.0.0",
			wantError: false,
			check: func(t *testing.T, c *types.InterfacesConfig) {
				if c.Interfaces["br-lan"].Netmask != "255.255.0.0" {
					t.Errorf("netmask not set correctly")
				}
			},
		},
		{
			name:      "set gateway",
			path:      "interfaces.br-lan.gateway",
			value:     "10.0.0.254",
			wantError: false,
			check: func(t *testing.T, c *types.InterfacesConfig) {
				if c.Interfaces["br-lan"].Gateway != "10.0.0.254" {
					t.Errorf("gateway not set correctly")
				}
			},
		},
		{
			name:      "set mtu as float64",
			path:      "interfaces.br-lan.mtu",
			value:     float64(9000),
			wantError: false,
			check: func(t *testing.T, c *types.InterfacesConfig) {
				if c.Interfaces["br-lan"].MTU != 9000 {
					t.Errorf("mtu not set correctly")
				}
			},
		},
		{
			name:      "set mtu as int",
			path:      "interfaces.br-lan.mtu",
			value:     1400,
			wantError: false,
			check: func(t *testing.T, c *types.InterfacesConfig) {
				if c.Interfaces["br-lan"].MTU != 1400 {
					t.Errorf("mtu not set correctly")
				}
			},
		},
		{
			name:      "set comment",
			path:      "interfaces.br-lan.comment",
			value:     "LAN bridge",
			wantError: false,
			check: func(t *testing.T, c *types.InterfacesConfig) {
				if c.Interfaces["br-lan"].Comment != "LAN bridge" {
					t.Errorf("comment not set correctly")
				}
			},
		},
		{
			name:      "invalid path - too short",
			path:      "interfaces.br-lan",
			value:     "value",
			wantError: true,
		},
		{
			name:      "interface not found",
			path:      "interfaces.nonexistent.enabled",
			value:     true,
			wantError: true,
		},
		{
			name:      "unknown field",
			path:      "interfaces.br-lan.unknownfield",
			value:     "value",
			wantError: true,
		},
		{
			name:      "wrong type for enabled",
			path:      "interfaces.br-lan.enabled",
			value:     "not-a-bool",
			wantError: true,
		},
		{
			name:      "wrong type for mtu",
			path:      "interfaces.br-lan.mtu",
			value:     "not-a-number",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy for each test
			testConfig := &types.InterfacesConfig{
				Interfaces: map[string]types.Interface{
					"br-lan": config.Interfaces["br-lan"],
				},
			}

			err := SetValue(testConfig, tt.path, tt.value)
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if !tt.wantError && tt.check != nil {
				tt.check(t, testConfig)
			}
		})
	}
}

func TestSetValue_Routes(t *testing.T) {
	config := &types.RoutesConfig{
		Routes: map[string]types.Route{
			"default": {
				Destination: "0.0.0.0/0",
				Gateway:     "10.0.0.254",
				Interface:   "br-lan",
				Enabled:     true,
				Metric:      100,
				Table:       254,
			},
		},
	}

	tests := []struct {
		name      string
		path      string
		value     interface{}
		wantError bool
		check     func(*testing.T, *types.RoutesConfig)
	}{
		{
			name:      "set destination",
			path:      "routes.default.destination",
			value:     "192.168.0.0/24",
			wantError: false,
			check: func(t *testing.T, c *types.RoutesConfig) {
				if c.Routes["default"].Destination != "192.168.0.0/24" {
					t.Errorf("destination not set correctly")
				}
			},
		},
		{
			name:      "set gateway",
			path:      "routes.default.gateway",
			value:     "10.0.0.1",
			wantError: false,
			check: func(t *testing.T, c *types.RoutesConfig) {
				if c.Routes["default"].Gateway != "10.0.0.1" {
					t.Errorf("gateway not set correctly")
				}
			},
		},
		{
			name:      "set interface",
			path:      "routes.default.interface",
			value:     "wan",
			wantError: false,
			check: func(t *testing.T, c *types.RoutesConfig) {
				if c.Routes["default"].Interface != "wan" {
					t.Errorf("interface not set correctly")
				}
			},
		},
		{
			name:      "set metric as float64",
			path:      "routes.default.metric",
			value:     float64(200),
			wantError: false,
			check: func(t *testing.T, c *types.RoutesConfig) {
				if c.Routes["default"].Metric != 200 {
					t.Errorf("metric not set correctly")
				}
			},
		},
		{
			name:      "set metric as int",
			path:      "routes.default.metric",
			value:     300,
			wantError: false,
			check: func(t *testing.T, c *types.RoutesConfig) {
				if c.Routes["default"].Metric != 300 {
					t.Errorf("metric not set correctly")
				}
			},
		},
		{
			name:      "set table as float64",
			path:      "routes.default.table",
			value:     float64(100),
			wantError: false,
			check: func(t *testing.T, c *types.RoutesConfig) {
				if c.Routes["default"].Table != 100 {
					t.Errorf("table not set correctly")
				}
			},
		},
		{
			name:      "set table as int",
			path:      "routes.default.table",
			value:     200,
			wantError: false,
			check: func(t *testing.T, c *types.RoutesConfig) {
				if c.Routes["default"].Table != 200 {
					t.Errorf("table not set correctly")
				}
			},
		},
		{
			name:      "set enabled",
			path:      "routes.default.enabled",
			value:     false,
			wantError: false,
			check: func(t *testing.T, c *types.RoutesConfig) {
				if c.Routes["default"].Enabled != false {
					t.Errorf("enabled not set correctly")
				}
			},
		},
		{
			name:      "set comment",
			path:      "routes.default.comment",
			value:     "Default route",
			wantError: false,
			check: func(t *testing.T, c *types.RoutesConfig) {
				if c.Routes["default"].Comment != "Default route" {
					t.Errorf("comment not set correctly")
				}
			},
		},
		{
			name:      "create new route",
			path:      "routes.newroute.gateway",
			value:     "10.0.0.10",
			wantError: false,
			check: func(t *testing.T, c *types.RoutesConfig) {
				route, exists := c.Routes["newroute"]
				if !exists {
					t.Errorf("new route not created")
				}
				if route.Gateway != "10.0.0.10" {
					t.Errorf("gateway not set correctly in new route")
				}
			},
		},
		{
			name:      "invalid path - too short",
			path:      "routes.default",
			value:     "value",
			wantError: true,
		},
		{
			name:      "unknown field",
			path:      "routes.default.unknownfield",
			value:     "value",
			wantError: true,
		},
		{
			name:      "wrong type for metric",
			path:      "routes.default.metric",
			value:     "not-a-number",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy for each test
			testConfig := &types.RoutesConfig{
				Routes: map[string]types.Route{
					"default": config.Routes["default"],
				},
			}

			err := SetValue(testConfig, tt.path, tt.value)
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if !tt.wantError && tt.check != nil {
				tt.check(t, testConfig)
			}
		})
	}
}

func TestSetValue_Generic(t *testing.T) {
	tests := []struct {
		name      string
		config    map[string]interface{}
		path      string
		value     interface{}
		wantError bool
		check     func(*testing.T, map[string]interface{})
	}{
		{
			name: "set simple value",
			config: map[string]interface{}{
				"enabled": false,
			},
			path:      "firewall.enabled",
			value:     true,
			wantError: false,
			check: func(t *testing.T, c map[string]interface{}) {
				if c["enabled"] != true {
					t.Errorf("enabled not set correctly")
				}
			},
		},
		{
			name: "set nested value",
			config: map[string]interface{}{
				"zones": map[string]interface{}{
					"wan": map[string]interface{}{
						"policy": "reject",
					},
				},
			},
			path:      "firewall.zones.wan.policy",
			value:     "accept",
			wantError: false,
			check: func(t *testing.T, c map[string]interface{}) {
				zones := c["zones"].(map[string]interface{})
				wan := zones["wan"].(map[string]interface{})
				if wan["policy"] != "accept" {
					t.Errorf("policy not set correctly")
				}
			},
		},
		{
			name: "key not found",
			config: map[string]interface{}{
				"enabled": true,
			},
			path:      "firewall.nonexistent.value",
			value:     "test",
			wantError: true,
		},
		{
			name: "cannot navigate into non-map",
			config: map[string]interface{}{
				"enabled": true,
			},
			path:      "firewall.enabled.nested",
			value:     "test",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetValue(tt.config, tt.path, tt.value)
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if !tt.wantError && tt.check != nil {
				tt.check(t, tt.config)
			}
		})
	}
}

func TestGetValue_Interfaces(t *testing.T) {
	config := &types.InterfacesConfig{
		Interfaces: map[string]types.Interface{
			"br-lan": {
				Type:       "bridge",
				Device:     "br0",
				DeviceName: "br-lan",
				Enabled:    true,
				Protocol:   "static",
				IPAddr:     "10.0.0.1",
				Netmask:    "255.255.255.0",
				Gateway:    "10.0.0.254",
				MTU:        1500,
				Comment:    "LAN bridge",
			},
		},
	}

	tests := []struct {
		name      string
		path      string
		want      interface{}
		wantError bool
	}{
		{
			name:      "get specific interface",
			path:      "interfaces.br-lan",
			want:      config.Interfaces["br-lan"],
			wantError: false,
		},
		{
			name:      "get type",
			path:      "interfaces.br-lan.type",
			want:      "bridge",
			wantError: false,
		},
		{
			name:      "get device",
			path:      "interfaces.br-lan.device",
			want:      "br0",
			wantError: false,
		},
		{
			name:      "get device_name",
			path:      "interfaces.br-lan.device_name",
			want:      "br-lan",
			wantError: false,
		},
		{
			name:      "get enabled",
			path:      "interfaces.br-lan.enabled",
			want:      true,
			wantError: false,
		},
		{
			name:      "get protocol",
			path:      "interfaces.br-lan.protocol",
			want:      "static",
			wantError: false,
		},
		{
			name:      "get ipaddr",
			path:      "interfaces.br-lan.ipaddr",
			want:      "10.0.0.1",
			wantError: false,
		},
		{
			name:      "get netmask",
			path:      "interfaces.br-lan.netmask",
			want:      "255.255.255.0",
			wantError: false,
		},
		{
			name:      "get gateway",
			path:      "interfaces.br-lan.gateway",
			want:      "10.0.0.254",
			wantError: false,
		},
		{
			name:      "get mtu",
			path:      "interfaces.br-lan.mtu",
			want:      1500,
			wantError: false,
		},
		{
			name:      "get comment",
			path:      "interfaces.br-lan.comment",
			want:      "LAN bridge",
			wantError: false,
		},
		{
			name:      "interface not found",
			path:      "interfaces.nonexistent",
			wantError: true,
		},
		{
			name:      "unknown field",
			path:      "interfaces.br-lan.unknownfield",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetValue(config, tt.path)
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if !tt.wantError {
				// For complex types, just check they're not nil
				if tt.path == "interfaces" || tt.path == "interfaces.br-lan" {
					assert.NotNil(t, got, "GetValue() returned nil")
				} else {
					assert.Equal(t, tt.want, got)
				}
			}
		})
	}
}

func TestGetValue_Routes(t *testing.T) {
	config := &types.RoutesConfig{
		Routes: map[string]types.Route{
			"default": {
				Name:        "default",
				Destination: "0.0.0.0/0",
				Gateway:     "10.0.0.254",
				Interface:   "br-lan",
				Enabled:     true,
				Metric:      100,
				Table:       254,
				Comment:     "Default route",
			},
		},
	}

	tests := []struct {
		name      string
		path      string
		want      interface{}
		wantError bool
	}{
		{
			name:      "get specific route",
			path:      "routes.default",
			want:      config.Routes["default"],
			wantError: false,
		},
		{
			name:      "get name",
			path:      "routes.default.name",
			want:      "default",
			wantError: false,
		},
		{
			name:      "get destination",
			path:      "routes.default.destination",
			want:      "0.0.0.0/0",
			wantError: false,
		},
		{
			name:      "get gateway",
			path:      "routes.default.gateway",
			want:      "10.0.0.254",
			wantError: false,
		},
		{
			name:      "get interface",
			path:      "routes.default.interface",
			want:      "br-lan",
			wantError: false,
		},
		{
			name:      "get metric",
			path:      "routes.default.metric",
			want:      100,
			wantError: false,
		},
		{
			name:      "get table",
			path:      "routes.default.table",
			want:      254,
			wantError: false,
		},
		{
			name:      "get enabled",
			path:      "routes.default.enabled",
			want:      true,
			wantError: false,
		},
		{
			name:      "get comment",
			path:      "routes.default.comment",
			want:      "Default route",
			wantError: false,
		},
		{
			name:      "route not found",
			path:      "routes.nonexistent",
			wantError: true,
		},
		{
			name:      "unknown field",
			path:      "routes.default.unknownfield",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetValue(config, tt.path)
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if !tt.wantError {
				// For complex types, just check they're not nil
				if tt.path == "routes.default" {
					assert.NotNil(t, got, "GetValue() returned nil")
				} else {
					assert.Equal(t, tt.want, got)
				}
			}
		})
	}
}

func TestGetValue_Generic(t *testing.T) {
	config := map[string]interface{}{
		"enabled": true,
		"zones": map[string]interface{}{
			"wan": map[string]interface{}{
				"policy": "reject",
			},
		},
		"rules": []string{"rule1", "rule2"},
	}

	tests := []struct {
		name      string
		path      string
		want      interface{}
		wantError bool
	}{
		{
			name:      "get simple value",
			path:      "firewall.enabled",
			want:      true,
			wantError: false,
		},
		{
			name:      "get nested value",
			path:      "firewall.zones.wan.policy",
			want:      "reject",
			wantError: false,
		},
		{
			name:      "get intermediate map",
			path:      "firewall.zones.wan",
			wantError: false,
		},
		{
			name:      "get array",
			path:      "firewall.rules",
			wantError: false,
		},
		{
			name:      "key not found",
			path:      "firewall.nonexistent",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetValue(config, tt.path)
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if !tt.wantError && tt.want != nil && got != tt.want {
				t.Errorf("GetValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetValue_WrongConfigType(t *testing.T) {
	// Test with wrong config type for interfaces path
	wrongConfig := map[string]interface{}{}
	err := SetValue(wrongConfig, "interfaces.br-lan.enabled", true)
	require.Error(t, err, "SetValue() expected error for wrong config type")

	// Test with wrong config type for routes path
	err = SetValue(wrongConfig, "routes.default.enabled", true)
	require.Error(t, err, "SetValue() expected error for wrong config type")
}

func TestGetValue_WrongConfigType(t *testing.T) {
	// Test with wrong config type for interfaces path
	wrongConfig := map[string]interface{}{}
	_, err := GetValue(wrongConfig, "interfaces.br-lan.enabled")
	require.Error(t, err, "GetValue() expected error for wrong config type")

	// Test with wrong config type for routes path
	_, err = GetValue(wrongConfig, "routes.default.enabled")
	require.Error(t, err, "GetValue() expected error for wrong config type")
}
