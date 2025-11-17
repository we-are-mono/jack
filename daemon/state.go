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
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/we-are-mono/jack/system"
	"github.com/we-are-mono/jack/types"
)

// ConfigEntry holds committed and pending config for a specific type
type ConfigEntry struct {
	Committed   interface{}
	Pending     interface{}
	LastApplied interface{} // Last config successfully applied to system
}

// State holds the daemon's in-memory state using a registry pattern
type State struct {
	registry     map[string]*ConfigEntry
	mu           sync.RWMutex

	// Snapshot management for rollback
	snapshots    map[string]*system.SystemSnapshot
	snapshotList []string // Ordered list of checkpoint IDs (oldest first)
}

// NewState creates a new state manager with an empty registry
// Config types are registered dynamically when first loaded
func NewState() *State {
	return &State{
		registry:  make(map[string]*ConfigEntry),
		snapshots: make(map[string]*system.SystemSnapshot),
	}
}

// LoadCommitted loads the committed config for a specific type
// Automatically registers the config type if it doesn't exist
func (s *State) LoadCommitted(configType string, config interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.registry[configType]
	if !exists {
		// Auto-register new config types
		entry = &ConfigEntry{}
		s.registry[configType] = entry
	}

	entry.Committed = config
	entry.Pending = nil // Clear any pending changes
}

// Type-specific load methods for convenience and backward compatibility
func (s *State) LoadCommittedInterfaces(config *types.InterfacesConfig) {
	s.LoadCommitted("interfaces", config)
}

func (s *State) LoadCommittedRoutes(config *types.RoutesConfig) {
	s.LoadCommitted("routes", config)
}

func (s *State) LoadCommittedJackConfig(config *types.JackConfig) {
	s.LoadCommitted("jack", config)
}

// GetCurrent returns the current effective config (pending if exists, otherwise committed)
func (s *State) GetCurrent(configType string) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.registry[configType]
	if !exists {
		return nil, fmt.Errorf("unknown config type: %s", configType)
	}

	if entry.Pending != nil {
		return entry.Pending, nil
	}
	return entry.Committed, nil
}

// Type-specific get current methods for convenience and backward compatibility
func (s *State) GetCurrentInterfaces() *types.InterfacesConfig {
	config, _ := s.GetCurrent("interfaces") //nolint:errcheck // Intentionally return nil on error
	if config == nil {
		return nil
	}
	if typed, ok := config.(*types.InterfacesConfig); ok {
		return typed
	}
	return nil
}

func (s *State) GetCurrentRoutes() *types.RoutesConfig {
	config, _ := s.GetCurrent("routes") //nolint:errcheck // Intentionally return nil on error
	if config == nil {
		return nil
	}
	if typed, ok := config.(*types.RoutesConfig); ok {
		return typed
	}
	return nil
}

func (s *State) GetCurrentJackConfig() *types.JackConfig {
	config, _ := s.GetCurrent("jack") //nolint:errcheck // Intentionally return nil on error
	if config == nil {
		return nil
	}
	if typed, ok := config.(*types.JackConfig); ok {
		return typed
	}
	return nil
}

// GetCommitted returns the committed config for a specific type
func (s *State) GetCommitted(configType string) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.registry[configType]
	if !exists {
		return nil, fmt.Errorf("unknown config type: %s", configType)
	}

	return entry.Committed, nil
}

// Type-specific get committed methods for convenience and backward compatibility
func (s *State) GetCommittedInterfaces() *types.InterfacesConfig {
	config, _ := s.GetCommitted("interfaces") //nolint:errcheck // Intentionally return nil on error
	if config == nil {
		return nil
	}
	if typed, ok := config.(*types.InterfacesConfig); ok {
		return typed
	}
	return nil
}

// HasPending returns true if there are any pending changes across all config types
func (s *State) HasPending() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, entry := range s.registry {
		if entry.Pending != nil {
			return true
		}
	}
	return false
}

// HasPendingFor returns true if there are pending changes for a specific config type
func (s *State) HasPendingFor(configType string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.registry[configType]
	if !exists {
		return false
	}

	return entry.Pending != nil
}

// SetPending sets the pending config for a specific type
// Automatically registers the config type if it doesn't exist
func (s *State) SetPending(configType string, config interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.registry[configType]
	if !exists {
		// Auto-register new config types
		entry = &ConfigEntry{}
		s.registry[configType] = entry
	}

	entry.Pending = config
}

// CommitPending moves all pending configs to committed (caller must save to disk)
func (s *State) CommitPending() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hasPending := false
	for _, entry := range s.registry {
		if entry.Pending != nil {
			hasPending = true
			entry.Committed = entry.Pending
			entry.Pending = nil
		}
	}

	if !hasPending {
		return fmt.Errorf("no pending changes to commit")
	}

	return nil
}

// RevertPending discards all pending changes
func (s *State) RevertPending() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear all pending changes (idempotent - succeeds even if no changes)
	for _, entry := range s.registry {
		entry.Pending = nil
	}

	return nil
}

// GetPendingTypes returns a list of config types that have pending changes
func (s *State) GetPendingTypes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var types []string
	for configType, entry := range s.registry {
		if entry.Pending != nil {
			types = append(types, configType)
		}
	}
	return types
}

// SetLastApplied records the last successfully applied config for a type
func (s *State) SetLastApplied(configType string, config interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.registry[configType]
	if !exists {
		entry = &ConfigEntry{}
		s.registry[configType] = entry
	}

	entry.LastApplied = config
}

// GetLastApplied returns the last successfully applied config for a type
func (s *State) GetLastApplied(configType string) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.registry[configType]
	if !exists {
		return nil, fmt.Errorf("unknown config type: %s", configType)
	}

	return entry.LastApplied, nil
}

// ConfigsEqual compares two configs for equality using JSON serialization
// This allows deep comparison of map[string]interface{} configs
func ConfigsEqual(a, b interface{}) bool {
	// Handle nil cases
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Try direct equality first (fast path)
	if reflect.DeepEqual(a, b) {
		return true
	}

	// Fall back to JSON comparison for map[string]interface{} types
	aJSON, err := json.Marshal(a)
	if err != nil {
		return false
	}

	bJSON, err := json.Marshal(b)
	if err != nil {
		return false
	}

	return string(aJSON) == string(bJSON)
}

// SaveSnapshot stores a system snapshot for potential rollback.
func (s *State) SaveSnapshot(checkpointID string, snapshot *system.SystemSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshots[checkpointID] = snapshot
	s.snapshotList = append(s.snapshotList, checkpointID)

	// Persist to disk
	return s.persistSnapshot(checkpointID, snapshot)
}

// LoadSnapshot retrieves a snapshot by checkpoint ID.
// If checkpointID is "latest", returns the most recent snapshot.
func (s *State) LoadSnapshot(checkpointID string) (*system.SystemSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if checkpointID == "latest" {
		if len(s.snapshotList) == 0 {
			return nil, fmt.Errorf("no snapshots available")
		}
		checkpointID = s.snapshotList[len(s.snapshotList)-1]
	}

	snapshot, exists := s.snapshots[checkpointID]
	if !exists {
		// Try loading from disk
		return s.loadSnapshotFromDisk(checkpointID)
	}

	return snapshot, nil
}

// ListSnapshots returns information about all available snapshots.
func (s *State) ListSnapshots() []SnapshotInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var snapshots []SnapshotInfo
	for _, checkpointID := range s.snapshotList {
		if snapshot, exists := s.snapshots[checkpointID]; exists {
			snapshots = append(snapshots, SnapshotInfo{
				CheckpointID: checkpointID,
				Timestamp:    snapshot.Timestamp,
				Trigger:      getTriggerType(checkpointID),
			})
		}
	}

	return snapshots
}

// PruneOldSnapshots removes old snapshots, keeping only the most recent N.
func (s *State) PruneOldSnapshots(keepCount int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.snapshotList) <= keepCount {
		return
	}

	// Remove oldest snapshots
	toRemove := s.snapshotList[:len(s.snapshotList)-keepCount]
	for _, checkpointID := range toRemove {
		delete(s.snapshots, checkpointID)
		// Remove from disk
		snapshotPath := getSnapshotPath(checkpointID)
		_ = os.Remove(snapshotPath) // Ignore errors
	}

	s.snapshotList = s.snapshotList[len(s.snapshotList)-keepCount:]
}

// persistSnapshot saves a snapshot to disk.
func (s *State) persistSnapshot(checkpointID string, snapshot *system.SystemSnapshot) error {
	snapshotDir := "/var/lib/jack/snapshots"
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	path := getSnapshotPath(checkpointID)
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write snapshot file: %w", err)
	}

	return nil
}

// loadSnapshotFromDisk loads a snapshot from disk.
func (s *State) loadSnapshotFromDisk(checkpointID string) (*system.SystemSnapshot, error) {
	path := getSnapshotPath(checkpointID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("snapshot not found: %w", err)
	}

	var snapshot system.SystemSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	return &snapshot, nil
}

// LoadSnapshotsFromDisk loads all available snapshots from disk on daemon startup.
func (s *State) LoadSnapshotsFromDisk() error {
	snapshotDir := "/var/lib/jack/snapshots"

	// Create directory if it doesn't exist
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		return fmt.Errorf("failed to read snapshot directory: %w", err)
	}

	var checkpointIDs []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}

		checkpointID := name[:len(name)-5] // Remove .json extension
		checkpointIDs = append(checkpointIDs, checkpointID)
	}

	// Sort by checkpoint ID (which includes timestamp)
	sort.Strings(checkpointIDs)

	s.mu.Lock()
	s.snapshotList = checkpointIDs
	s.mu.Unlock()

	return nil
}

// SnapshotInfo contains metadata about a snapshot.
type SnapshotInfo struct {
	CheckpointID string    `json:"checkpoint_id"`
	Timestamp    time.Time `json:"timestamp"`
	Trigger      string    `json:"trigger"`
}

// getSnapshotPath returns the file path for a checkpoint ID.
func getSnapshotPath(checkpointID string) string {
	return filepath.Join("/var/lib/jack/snapshots", checkpointID+".json")
}

// getTriggerType determines the trigger type from the checkpoint ID.
func getTriggerType(checkpointID string) string {
	if len(checkpointID) > 5 && checkpointID[:5] == "auto-" {
		return "auto"
	}
	if len(checkpointID) > 7 && checkpointID[:7] == "manual-" {
		return "manual"
	}
	return "unknown"
}
