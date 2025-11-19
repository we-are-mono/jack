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

package validation

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorCollector_NoErrors(t *testing.T) {
	v := NewCollector()

	v.Check(nil)
	v.Check(nil)
	v.CheckMsg(nil, "some message")

	assert.NoError(t, v.Error())
}

func TestErrorCollector_SingleError(t *testing.T) {
	v := NewCollector()

	v.Check(fmt.Errorf("test error"))

	err := v.Error()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "test error")
}

func TestErrorCollector_MultipleErrors(t *testing.T) {
	v := NewCollector()

	v.Check(fmt.Errorf("first error"))
	v.Check(fmt.Errorf("second error"))
	v.Check(fmt.Errorf("third error"))

	err := v.Error()
	require.Error(t, err)

	// All errors should be present in the combined error
	assert.Contains(t, err.Error(), "first error")
	assert.Contains(t, err.Error(), "second error")
	assert.Contains(t, err.Error(), "third error")
}

func TestErrorCollector_WithContext(t *testing.T) {
	v := NewCollector().WithContext("route default")

	v.Check(fmt.Errorf("invalid metric"))
	v.Check(fmt.Errorf("invalid table"))

	err := v.Error()
	require.Error(t, err)

	// Context should be prepended to each error
	assert.Contains(t, err.Error(), "route default: invalid metric")
	assert.Contains(t, err.Error(), "route default: invalid table")
}

func TestErrorCollector_CheckMsg(t *testing.T) {
	v := NewCollector()

	v.CheckMsg(fmt.Errorf("192.168.1.256"), "invalid IP")

	err := v.Error()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid IP: 192.168.1.256")
}

func TestErrorCollector_CheckMsg_WithContext(t *testing.T) {
	v := NewCollector().WithContext("interface eth0")

	v.CheckMsg(fmt.Errorf("192.168.1.256"), "invalid IP address")
	v.CheckMsg(fmt.Errorf("port 99999 out of range"), "invalid listen port")

	err := v.Error()
	require.Error(t, err)

	// Context + message + error should all be present
	assert.Contains(t, err.Error(), "interface eth0: invalid IP address: 192.168.1.256")
	assert.Contains(t, err.Error(), "interface eth0: invalid listen port: port 99999 out of range")
}

func TestErrorCollector_MixedChecks(t *testing.T) {
	v := NewCollector().WithContext("firewall zone wan")

	v.Check(fmt.Errorf("no interfaces defined"))
	v.CheckMsg(fmt.Errorf("must be ACCEPT, DROP, or REJECT"), "invalid input policy")
	v.Check(nil) // This should be ignored
	v.CheckMsg(fmt.Errorf("must be ACCEPT, DROP, or REJECT"), "invalid forward policy")

	err := v.Error()
	require.Error(t, err)

	// Verify all three errors are present
	errStr := err.Error()
	assert.Contains(t, errStr, "firewall zone wan: no interfaces defined")
	assert.Contains(t, errStr, "firewall zone wan: invalid input policy")
	assert.Contains(t, errStr, "firewall zone wan: invalid forward policy")
}

func TestErrorCollector_NestedValidation(t *testing.T) {
	// Simulate nested validation where a parent validator collects errors from child validators
	// This is how Jack actually works - child validators have their own context
	validateChild1 := func() error {
		v := NewCollector().WithContext("route default")
		v.Check(fmt.Errorf("invalid metric"))
		return v.Error()
	}

	validateChild2 := func() error {
		v := NewCollector().WithContext("interface eth0")
		v.Check(fmt.Errorf("invalid MTU"))
		v.Check(fmt.Errorf("invalid MAC"))
		return v.Error()
	}

	parentValidator := func() error {
		v := NewCollector()

		// Validate child 1
		v.Check(validateChild1())

		// Validate child 2
		v.Check(validateChild2())

		return v.Error()
	}

	err := parentValidator()
	require.Error(t, err)

	// All nested errors should be present (without additional parent context)
	errStr := err.Error()
	assert.Contains(t, errStr, "route default: invalid metric")
	assert.Contains(t, errStr, "interface eth0: invalid MTU")
	assert.Contains(t, errStr, "interface eth0: invalid MAC")
}

func TestErrorCollector_ErrorFormat(t *testing.T) {
	v := NewCollector()

	v.Check(fmt.Errorf("error 1"))
	v.Check(fmt.Errorf("error 2"))
	v.Check(fmt.Errorf("error 3"))

	err := v.Error()
	require.Error(t, err)

	// errors.Join uses newlines to separate errors
	errStr := err.Error()
	lines := strings.Split(errStr, "\n")

	// Should have 3 lines (one for each error)
	assert.Len(t, lines, 3)
	assert.Contains(t, lines[0], "error 1")
	assert.Contains(t, lines[1], "error 2")
	assert.Contains(t, lines[2], "error 3")
}

func TestErrorCollector_ErrorUnwrap(t *testing.T) {
	v := NewCollector()

	originalErr := fmt.Errorf("original error")
	v.Check(originalErr)

	err := v.Error()
	require.Error(t, err)

	// Should be able to unwrap to the original error
	assert.True(t, errors.Is(err, originalErr))
}

func TestErrorCollector_RealWorldScenario(t *testing.T) {
	// Simulate a real validation scenario with multiple field validations
	type Route struct {
		Name        string
		Destination string
		Metric      int
		Table       int
		Gateway     string
	}

	validateRoute := func(r *Route) error {
		v := NewCollector().WithContext(fmt.Sprintf("route %s", r.Name))

		// Validate metric
		if r.Metric < 0 {
			v.Check(fmt.Errorf("metric %d cannot be negative", r.Metric))
		}

		// Validate table ID
		if r.Table < 0 || r.Table > 4294967295 {
			v.Check(fmt.Errorf("table ID %d out of valid range [0, 4294967295]", r.Table))
		}

		// Validate destination
		if r.Destination == "" {
			v.Check(fmt.Errorf("destination cannot be empty"))
		}

		// Validate gateway (if specified)
		if r.Gateway == "invalid" {
			v.CheckMsg(fmt.Errorf("invalid IP address: %s", r.Gateway), "invalid gateway")
		}

		return v.Error()
	}

	// Test with invalid route
	invalidRoute := &Route{
		Name:        "default",
		Destination: "", // Invalid: empty
		Metric:      -5, // Invalid: negative
		Table:       999999999999, // Invalid: out of range
		Gateway:     "invalid", // Invalid: not an IP
	}

	err := validateRoute(invalidRoute)
	require.Error(t, err)

	// All four errors should be present
	errStr := err.Error()
	assert.Contains(t, errStr, "route default: metric -5 cannot be negative")
	assert.Contains(t, errStr, "route default: table ID")
	assert.Contains(t, errStr, "route default: destination cannot be empty")
	assert.Contains(t, errStr, "route default: invalid gateway: invalid IP address: invalid")

	// Test with valid route
	validRoute := &Route{
		Name:        "lan",
		Destination: "192.168.1.0/24",
		Metric:      100,
		Table:       254,
		Gateway:     "192.168.1.1",
	}

	err = validateRoute(validRoute)
	assert.NoError(t, err)
}
