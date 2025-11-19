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

package system

// RestoreSnapshot restores the system to the state captured in the snapshot.
// For testing, create a SnapshotManager with injected dependencies.
func RestoreSnapshot(snapshot *SystemSnapshot, scope []string) error {
	return defaultSnapshotManager.RestoreSnapshot(snapshot, scope)
}

// RestoreNftablesRules restores the nftables ruleset from a JSON dump.
// For testing, create a SnapshotManager with injected dependencies.
func RestoreNftablesRules(rulesJSON string) error {
	return defaultSnapshotManager.RestoreNftablesRules(rulesJSON)
}

// containsScope checks if a scope list contains the given scope.
func containsScope(scopes []string, scope string) bool {
	for _, s := range scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// containsString checks if a string slice contains the given string.
func containsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
