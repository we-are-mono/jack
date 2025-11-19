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
)

// ErrorCollector accumulates validation errors for better user experience.
// It allows collecting multiple validation errors and returning them all at once
// instead of failing on the first error.
type ErrorCollector struct {
	errs []error
	ctx  string // Optional context prefix (e.g., "route default", "interface eth0")
}

// NewCollector creates a new error collector.
func NewCollector() *ErrorCollector {
	return &ErrorCollector{}
}

// WithContext sets a context prefix that will be prepended to all subsequent errors.
// This is useful for providing hierarchical error messages (e.g., "route default: ...").
func (ec *ErrorCollector) WithContext(ctx string) *ErrorCollector {
	ec.ctx = ctx
	return ec
}

// Check runs a validation and collects any error.
// If the error is nil, it is ignored. Otherwise, it is added to the collection
// with the configured context prefix (if any).
func (ec *ErrorCollector) Check(err error) {
	if err != nil {
		if ec.ctx != "" {
			ec.errs = append(ec.errs, fmt.Errorf("%s: %w", ec.ctx, err))
		} else {
			ec.errs = append(ec.errs, err)
		}
	}
}

// CheckMsg runs a validation and wraps any error with a custom message.
// This is useful for adding additional context to specific validation failures.
// The message is inserted between the context prefix and the original error.
func (ec *ErrorCollector) CheckMsg(err error, msg string) {
	if err != nil {
		if ec.ctx != "" {
			ec.errs = append(ec.errs, fmt.Errorf("%s: %s: %w", ec.ctx, msg, err))
		} else {
			ec.errs = append(ec.errs, fmt.Errorf("%s: %w", msg, err))
		}
	}
}

// Error returns all accumulated errors joined together, or nil if no errors were collected.
// Uses errors.Join (Go 1.20+) to combine multiple errors.
func (ec *ErrorCollector) Error() error {
	return errors.Join(ec.errs...)
}
