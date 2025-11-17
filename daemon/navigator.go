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
	"fmt"
	"reflect"
	"strings"
)

// fieldCache caches struct field lookups for performance
var fieldCache = make(map[reflect.Type]map[string]int)

// getStructFieldByJSONTag finds a struct field by its JSON tag name using reflection.
// Returns the field index or -1 if not found.
func getStructFieldByJSONTag(structType reflect.Type, jsonTag string) int {
	// Check cache first
	if cache, exists := fieldCache[structType]; exists {
		if idx, found := cache[jsonTag]; found {
			return idx
		}
	}

	// Build cache for this type
	cache := make(map[string]int)
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		tag := field.Tag.Get("json")
		if tag == "" {
			continue
		}

		// Parse tag (may include options like "omitempty")
		tagName := strings.Split(tag, ",")[0]
		cache[tagName] = i
	}

	// Store in global cache
	fieldCache[structType] = cache

	// Lookup
	if idx, found := cache[jsonTag]; found {
		return idx
	}
	return -1
}

// setStructField sets a field value on a struct using reflection.
// The struct must be passed as a pointer.
func setStructField(structPtr interface{}, fieldName string, value interface{}) error {
	structValue := reflect.ValueOf(structPtr)
	if structValue.Kind() != reflect.Ptr {
		return fmt.Errorf("setStructField requires a pointer to struct")
	}

	structValue = structValue.Elem()
	if structValue.Kind() != reflect.Struct {
		return fmt.Errorf("setStructField requires a struct, got %s", structValue.Kind())
	}

	// Find field by JSON tag
	fieldIdx := getStructFieldByJSONTag(structValue.Type(), fieldName)
	if fieldIdx == -1 {
		return fmt.Errorf("unknown field: %s", fieldName)
	}

	field := structValue.Field(fieldIdx)
	if !field.CanSet() {
		return fmt.Errorf("field %s cannot be set", fieldName)
	}

	// Convert and set value
	if err := setFieldValue(field, value); err != nil {
		// Prepend field name to error message for better context
		return fmt.Errorf("%s %s", fieldName, err.Error())
	}
	return nil
}

// getStructField gets a field value from a struct using reflection.
func getStructField(structVal interface{}, fieldName string) (interface{}, error) {
	structValue := reflect.ValueOf(structVal)

	// Handle pointer to struct
	if structValue.Kind() == reflect.Ptr {
		structValue = structValue.Elem()
	}

	if structValue.Kind() != reflect.Struct {
		return nil, fmt.Errorf("getStructField requires a struct, got %s", structValue.Kind())
	}

	// Find field by JSON tag
	fieldIdx := getStructFieldByJSONTag(structValue.Type(), fieldName)
	if fieldIdx == -1 {
		return nil, fmt.Errorf("unknown field: %s", fieldName)
	}

	field := structValue.Field(fieldIdx)
	return field.Interface(), nil
}

// setFieldValue sets a reflect.Value with type conversion
func setFieldValue(field reflect.Value, value interface{}) error {
	valueReflect := reflect.ValueOf(value)

	// Handle nil values
	if value == nil {
		field.Set(reflect.Zero(field.Type()))
		return nil
	}

	// Direct assignment if types match
	if valueReflect.Type().AssignableTo(field.Type()) {
		field.Set(valueReflect)
		return nil
	}

	// Type conversions
	switch field.Kind() {
	case reflect.String:
		if valueReflect.Kind() == reflect.String {
			field.SetString(valueReflect.String())
			return nil
		}
		return fmt.Errorf("must be a string")

	case reflect.Bool:
		if valueReflect.Kind() == reflect.Bool {
			field.SetBool(valueReflect.Bool())
			return nil
		}
		return fmt.Errorf("must be a boolean")

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch valueReflect.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			field.SetInt(valueReflect.Int())
			return nil
		case reflect.Float32, reflect.Float64:
			// JSON numbers are unmarshaled as float64
			field.SetInt(int64(valueReflect.Float()))
			return nil
		}
		return fmt.Errorf("must be a number")

	case reflect.Slice:
		if valueReflect.Kind() == reflect.Slice {
			field.Set(valueReflect)
			return nil
		}
		return fmt.Errorf("must be a slice")

	case reflect.Ptr:
		// Handle pointer fields
		if valueReflect.Type().AssignableTo(field.Type()) {
			field.Set(valueReflect)
			return nil
		}
		// Try setting the pointer's element
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		return setFieldValue(field.Elem(), value)

	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}
}
