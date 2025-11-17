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

package plugins

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewPluginManager tests PluginManager creation
func TestNewPluginManager(t *testing.T) {
	pm := NewPluginManager()

	require.NotNil(t, pm)
	assert.Len(t, pm.pluginDirs, 3)
	assert.Contains(t, pm.pluginDirs, "./bin")
	assert.Contains(t, pm.pluginDirs, "/usr/lib/jack/plugins")
	assert.Contains(t, pm.pluginDirs, "/opt/jack/plugins")
}

// TestPluginManager_FindPlugin tests plugin discovery
func TestPluginManager_FindPlugin(t *testing.T) {
	// Create temporary directory for test plugins
	tmpDir := t.TempDir()

	// Create a custom plugin manager with test directory
	pm := &PluginManager{
		pluginDirs: []string{tmpDir},
	}

	tests := []struct {
		name          string
		pluginName    string
		createPlugin  bool
		makeExecutable bool
		expectError   bool
		errorMsg      string
	}{
		{
			name:          "plugin exists and is executable",
			pluginName:    "nftables",
			createPlugin:  true,
			makeExecutable: true,
		},
		{
			name:          "plugin does not exist",
			pluginName:    "nonexistent",
			createPlugin:  false,
			expectError:   true,
			errorMsg:      "plugin not found",
		},
		{
			name:          "plugin exists but not executable",
			pluginName:    "notexec",
			createPlugin:  true,
			makeExecutable: false,
			expectError:   true,
			errorMsg:      "plugin not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup: create plugin file if needed
			if tt.createPlugin {
				pluginPath := filepath.Join(tmpDir, "jack-plugin-"+tt.pluginName)
				err := os.WriteFile(pluginPath, []byte("#!/bin/bash\necho test"), 0644)
				require.NoError(t, err)

				if tt.makeExecutable {
					err = os.Chmod(pluginPath, 0755)
					require.NoError(t, err)
				}
			}

			// Test
			path, err := pm.FindPlugin(tt.pluginName)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Contains(t, path, "jack-plugin-"+tt.pluginName)
				assert.FileExists(t, path)
			}
		})
	}
}

// TestPluginManager_FindPlugin_MultipleDirectories tests searching multiple directories
func TestPluginManager_FindPlugin_MultipleDirectories(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	pm := &PluginManager{
		pluginDirs: []string{tmpDir1, tmpDir2},
	}

	// Create plugin in second directory
	pluginPath := filepath.Join(tmpDir2, "jack-plugin-test")
	err := os.WriteFile(pluginPath, []byte("#!/bin/bash"), 0755)
	require.NoError(t, err)

	// Should find it even though it's not in first directory
	found, err := pm.FindPlugin("test")
	require.NoError(t, err)
	assert.Equal(t, pluginPath, found)
}

// TestPluginManager_FindPlugin_FirstMatchWins tests that first directory takes precedence
func TestPluginManager_FindPlugin_FirstMatchWins(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	pm := &PluginManager{
		pluginDirs: []string{tmpDir1, tmpDir2},
	}

	// Create same plugin in both directories
	plugin1 := filepath.Join(tmpDir1, "jack-plugin-dup")
	plugin2 := filepath.Join(tmpDir2, "jack-plugin-dup")

	err := os.WriteFile(plugin1, []byte("#!/bin/bash\necho version1"), 0755)
	require.NoError(t, err)

	err = os.WriteFile(plugin2, []byte("#!/bin/bash\necho version2"), 0755)
	require.NoError(t, err)

	// Should find the one in first directory
	found, err := pm.FindPlugin("dup")
	require.NoError(t, err)
	assert.Equal(t, plugin1, found)
}

// TestPluginManager_ListPlugins tests listing all available plugins
func TestPluginManager_ListPlugins(t *testing.T) {
	tmpDir := t.TempDir()

	pm := &PluginManager{
		pluginDirs: []string{tmpDir},
	}

	// Create multiple plugins
	plugins := []string{"nftables", "wireguard", "dnsmasq", "monitoring"}
	for _, name := range plugins {
		pluginPath := filepath.Join(tmpDir, "jack-plugin-"+name)
		err := os.WriteFile(pluginPath, []byte("#!/bin/bash"), 0755)
		require.NoError(t, err)
	}

	// Create non-plugin files (should be ignored)
	err := os.WriteFile(filepath.Join(tmpDir, "not-a-plugin"), []byte("test"), 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "jack-plugin-notexec"), []byte("test"), 0644)
	require.NoError(t, err)

	// List plugins
	found, err := pm.ListPlugins()
	require.NoError(t, err)

	// Should find all 4 plugins
	assert.Len(t, found, 4)
	for _, name := range plugins {
		assert.Contains(t, found, name)
	}

	// Should not include non-plugins
	assert.NotContains(t, found, "not-a-plugin")
	assert.NotContains(t, found, "notexec")
}

// TestPluginManager_ListPlugins_EmptyDirectory tests listing with no plugins
func TestPluginManager_ListPlugins_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	pm := &PluginManager{
		pluginDirs: []string{tmpDir},
	}

	plugins, err := pm.ListPlugins()
	require.NoError(t, err)
	assert.Empty(t, plugins)
}

// TestPluginManager_ListPlugins_NonexistentDirectory tests listing with nonexistent directory
func TestPluginManager_ListPlugins_NonexistentDirectory(t *testing.T) {
	pm := &PluginManager{
		pluginDirs: []string{"/nonexistent/directory/that/does/not/exist"},
	}

	plugins, err := pm.ListPlugins()
	require.NoError(t, err) // Should not error, just skip missing directories
	assert.Empty(t, plugins)
}

// TestPluginManager_ListPlugins_MultipleDirectories tests listing from multiple directories
func TestPluginManager_ListPlugins_MultipleDirectories(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	pm := &PluginManager{
		pluginDirs: []string{tmpDir1, tmpDir2},
	}

	// Create plugins in first directory
	plugin1 := filepath.Join(tmpDir1, "jack-plugin-plugin1")
	err := os.WriteFile(plugin1, []byte("#!/bin/bash"), 0755)
	require.NoError(t, err)

	// Create plugins in second directory
	plugin2 := filepath.Join(tmpDir2, "jack-plugin-plugin2")
	err = os.WriteFile(plugin2, []byte("#!/bin/bash"), 0755)
	require.NoError(t, err)

	// List should include plugins from both directories
	plugins, err := pm.ListPlugins()
	require.NoError(t, err)
	assert.Len(t, plugins, 2)
	assert.Contains(t, plugins, "plugin1")
	assert.Contains(t, plugins, "plugin2")
}

// TestPluginManager_ListPlugins_Deduplication tests that duplicate plugins are handled
func TestPluginManager_ListPlugins_Deduplication(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	pm := &PluginManager{
		pluginDirs: []string{tmpDir1, tmpDir2},
	}

	// Create same plugin in both directories
	plugin1 := filepath.Join(tmpDir1, "jack-plugin-duplicate")
	plugin2 := filepath.Join(tmpDir2, "jack-plugin-duplicate")

	err := os.WriteFile(plugin1, []byte("#!/bin/bash"), 0755)
	require.NoError(t, err)

	err = os.WriteFile(plugin2, []byte("#!/bin/bash"), 0755)
	require.NoError(t, err)

	// List should deduplicate
	plugins, err := pm.ListPlugins()
	require.NoError(t, err)
	assert.Len(t, plugins, 1)
	assert.Contains(t, plugins, "duplicate")
}

// TestPluginManager_ListPlugins_DirectoryAsPlugin tests that directories are ignored
func TestPluginManager_ListPlugins_DirectoryAsPlugin(t *testing.T) {
	tmpDir := t.TempDir()

	pm := &PluginManager{
		pluginDirs: []string{tmpDir},
	}

	// Create a directory with plugin naming pattern (should be ignored)
	dirPath := filepath.Join(tmpDir, "jack-plugin-notafile")
	err := os.Mkdir(dirPath, 0755)
	require.NoError(t, err)

	// Create a real plugin
	pluginPath := filepath.Join(tmpDir, "jack-plugin-realfile")
	err = os.WriteFile(pluginPath, []byte("#!/bin/bash"), 0755)
	require.NoError(t, err)

	// List should only include the file, not the directory
	plugins, err := pm.ListPlugins()
	require.NoError(t, err)
	assert.Len(t, plugins, 1)
	assert.Contains(t, plugins, "realfile")
	assert.NotContains(t, plugins, "notafile")
}

// TestPluginManager_FindPlugin_SymlinkSupport tests that symlinks are followed
func TestPluginManager_FindPlugin_SymlinkSupport(t *testing.T) {
	tmpDir := t.TempDir()

	pm := &PluginManager{
		pluginDirs: []string{tmpDir},
	}

	// Create actual plugin in subdirectory
	subDir := filepath.Join(tmpDir, "plugins")
	err := os.Mkdir(subDir, 0755)
	require.NoError(t, err)

	realPlugin := filepath.Join(subDir, "real-plugin")
	err = os.WriteFile(realPlugin, []byte("#!/bin/bash"), 0755)
	require.NoError(t, err)

	// Create symlink in tmpDir
	symlinkPath := filepath.Join(tmpDir, "jack-plugin-symlinked")
	err = os.Symlink(realPlugin, symlinkPath)
	require.NoError(t, err)

	// Should find the plugin via symlink
	found, err := pm.FindPlugin("symlinked")
	require.NoError(t, err)
	assert.Equal(t, symlinkPath, found)
}

// TestPluginManager_NamingConvention tests plugin naming convention
func TestPluginManager_NamingConvention(t *testing.T) {
	tmpDir := t.TempDir()

	pm := &PluginManager{
		pluginDirs: []string{tmpDir},
	}

	// Create plugin with correct naming
	correctName := filepath.Join(tmpDir, "jack-plugin-myplugin")
	err := os.WriteFile(correctName, []byte("#!/bin/bash"), 0755)
	require.NoError(t, err)

	// Create file with incorrect naming (should be ignored)
	wrongName := filepath.Join(tmpDir, "myplugin")
	err = os.WriteFile(wrongName, []byte("#!/bin/bash"), 0755)
	require.NoError(t, err)

	// List should only include correctly named plugin
	plugins, err := pm.ListPlugins()
	require.NoError(t, err)
	assert.Len(t, plugins, 1)
	assert.Contains(t, plugins, "myplugin")

	// Find should work with just the plugin name
	found, err := pm.FindPlugin("myplugin")
	require.NoError(t, err)
	assert.Equal(t, correctName, found)
}
