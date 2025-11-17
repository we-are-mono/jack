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

// Package cmd implements the CLI commands for Jack using cobra.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/we-are-mono/jack/state"
	"github.com/we-are-mono/jack/types"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration files without applying them",
	Long:  `Checks all JSON configuration files for syntax errors and validates their structure.`,
	Run:   runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) {
	configDir := state.GetConfigDir()
	hasErrors := false

	fmt.Printf("Validating configuration files in %s...\n\n", configDir)

	// Validate interfaces.json
	if err := validateInterfaces(); err != nil {
		fmt.Printf("❌ interfaces.json: %v\n", err)
		hasErrors = true
	} else {
		fmt.Printf("✓ interfaces.json: valid\n")
	}

	// Validate routes.json
	if err := validateRoutes(); err != nil {
		fmt.Printf("❌ routes.json: %v\n", err)
		hasErrors = true
	} else {
		fmt.Printf("✓ routes.json: valid\n")
	}

	// Validate jack.json
	if err := validateJackConfig(); err != nil {
		fmt.Printf("❌ jack.json: %v\n", err)
		hasErrors = true
	} else {
		fmt.Printf("✓ jack.json: valid\n")
	}

	// Validate plugin configs
	pluginConfigs := []string{"nftables.json", "dnsmasq.json", "wireguard.json"}
	for _, pluginConfig := range pluginConfigs {
		path := filepath.Join(configDir, pluginConfig)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Printf("⊘ %s: not found (optional)\n", pluginConfig)
			continue
		}

		if err := validateGenericJSON(path); err != nil {
			fmt.Printf("❌ %s: %v\n", pluginConfig, err)
			hasErrors = true
		} else {
			fmt.Printf("✓ %s: valid\n", pluginConfig)
		}
	}

	fmt.Println()
	if hasErrors {
		fmt.Println("❌ Validation failed - please fix the errors above")
		os.Exit(1)
	} else {
		fmt.Println("✓ All configuration files are valid")
	}
}

func validateInterfaces() error {
	var interfacesConfig types.InterfacesConfig
	if err := state.LoadConfig("interfaces", &interfacesConfig); err != nil {
		return err
	}

	// Additional validation
	if len(interfacesConfig.Interfaces) == 0 {
		return fmt.Errorf("no interfaces defined")
	}

	return nil
}

func validateRoutes() error {
	var routesConfig types.RoutesConfig
	if err := state.LoadConfig("routes", &routesConfig); err != nil {
		// Routes config is optional
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return nil
}

func validateJackConfig() error {
	jackConfig, err := state.LoadJackConfig()
	if err != nil {
		return err
	}

	if len(jackConfig.Plugins) == 0 {
		return fmt.Errorf("no plugins configured")
	}

	return nil
}

func validateGenericJSON(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read: %w", err)
	}

	// Just validate JSON syntax by unmarshaling to interface{}
	var config interface{}
	if err := state.UnmarshalJSON(data, &config); err != nil {
		return err
	}

	return nil
}
