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
	"fmt"
	"reflect"
	"strings"

	"github.com/we-are-mono/jack/types"
)

type DiffResult struct {
	Path       string      `json:"path"`
	Old        interface{} `json:"old"`
	New        interface{} `json:"new"`
	ChangeType string      `json:"change_type"` // "modified", "added", "removed"
}

// DiffConfigs diffs any two configs of the same type generically
func DiffConfigs(configType string, committed, pending interface{}) ([]DiffResult, error) {
	// Special handling for interfaces which has map-based structure
	if configType == "interfaces" {
		committedIface, ok1 := committed.(*types.InterfacesConfig)
		pendingIface, ok2 := pending.(*types.InterfacesConfig)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("invalid InterfacesConfig types")
		}
		return DiffInterfaces(committedIface, pendingIface), nil
	}

	// For all other config types (including routes and plugin configs), use generic reflection-based diff
	// This works because diffStructs uses reflection to compare any struct fields
	committedVal := reflect.ValueOf(committed)
	pendingVal := reflect.ValueOf(pending)

	// Dereference pointers if needed
	if committedVal.Kind() == reflect.Ptr {
		committedVal = committedVal.Elem()
	}
	if pendingVal.Kind() == reflect.Ptr {
		pendingVal = pendingVal.Elem()
	}

	return diffStructs(configType, committedVal.Interface(), pendingVal.Interface()), nil
}

func DiffInterfaces(committed, pending *types.InterfacesConfig) []DiffResult {
	var diffs []DiffResult

	for name, pendingIface := range pending.Interfaces {
		committedIface, exists := committed.Interfaces[name]

		if !exists {
			diffs = append(diffs, DiffResult{
				Path:       fmt.Sprintf("interfaces.%s", name),
				Old:        nil,
				New:        pendingIface,
				ChangeType: "added",
			})
			continue
		}

		diffs = append(diffs, diffStructs(fmt.Sprintf("interfaces.%s", name), committedIface, pendingIface)...)
	}

	for name := range committed.Interfaces {
		if _, exists := pending.Interfaces[name]; !exists {
			diffs = append(diffs, DiffResult{
				Path:       fmt.Sprintf("interfaces.%s", name),
				Old:        committed.Interfaces[name],
				New:        nil,
				ChangeType: "removed",
			})
		}
	}

	return diffs
}

// diffStructs performs a generic reflection-based diff on any struct
func diffStructs(basePath string, old, new interface{}) []DiffResult {
	var diffs []DiffResult

	oldVal := reflect.ValueOf(old)
	newVal := reflect.ValueOf(new)
	oldType := oldVal.Type()

	for i := 0; i < oldVal.NumField(); i++ {
		field := oldType.Field(i)

		if !field.IsExported() {
			continue
		}

		fieldName := field.Tag.Get("json")
		if fieldName == "" || fieldName == "-" {
			fieldName = field.Name
		}

		if idx := strings.Index(fieldName, ","); idx != -1 {
			fieldName = fieldName[:idx]
		}

		oldFieldVal := oldVal.Field(i)
		newFieldVal := newVal.Field(i)

		if !reflect.DeepEqual(oldFieldVal.Interface(), newFieldVal.Interface()) {
			if isZeroValue(oldFieldVal) && isZeroValue(newFieldVal) {
				continue
			}

			diffs = append(diffs, DiffResult{
				Path:       fmt.Sprintf("%s.%s", basePath, fieldName),
				Old:        oldFieldVal.Interface(),
				New:        newFieldVal.Interface(),
				ChangeType: "modified",
			})
		}
	}

	return diffs
}

func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}

func FormatDiff(diffs []DiffResult) string {
	if len(diffs) == 0 {
		return "No changes"
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Found %d change(s):\n", len(diffs)))

	for _, diff := range diffs {
		switch diff.ChangeType {
		case "added":
			lines = append(lines, fmt.Sprintf("  + %s (added)", diff.Path))
		case "removed":
			lines = append(lines, fmt.Sprintf("  - %s (removed)", diff.Path))
		case "modified":
			lines = append(lines, fmt.Sprintf("  ~ %s: %v → %v", diff.Path, formatValue(diff.Old), formatValue(diff.New)))
		}
	}

	return strings.Join(lines, "\n")
}

func formatValue(v interface{}) string {
	if v == nil {
		return "(none)"
	}

	val := reflect.ValueOf(v)
	if isZeroValue(val) {
		return "(empty)"
	}

	switch v := v.(type) {
	case string:
		return fmt.Sprintf("%q", v)
	case []string:
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
