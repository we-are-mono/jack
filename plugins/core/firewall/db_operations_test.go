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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockDatabaseService implements plugins.DaemonService for testing
type MockDatabaseService struct {
	calls         []MockCall
	responses     map[string][]byte
	errorOnCall   map[string]error
	callCount     map[string]int
	customHandler func(ctx context.Context, serviceName string, method string, argsJSON []byte) ([]byte, error)
}

type MockCall struct {
	ServiceName string
	Method      string
	ArgsJSON    []byte
}

func NewMockDatabaseService() *MockDatabaseService {
	return &MockDatabaseService{
		calls:       []MockCall{},
		responses:   make(map[string][]byte),
		errorOnCall: make(map[string]error),
		callCount:   make(map[string]int),
	}
}

func (m *MockDatabaseService) WaitForService(ctx context.Context, serviceName string) error {
	return nil
}

func (m *MockDatabaseService) WaitForServices(ctx context.Context, serviceNames []string) error {
	return nil
}

func (m *MockDatabaseService) IsServiceReady(serviceName string) bool {
	return true
}

func (m *MockDatabaseService) CallService(ctx context.Context, serviceName string, method string, argsJSON []byte) ([]byte, error) {
	// Use custom handler if provided
	if m.customHandler != nil {
		return m.customHandler(ctx, serviceName, method, argsJSON)
	}

	m.calls = append(m.calls, MockCall{
		ServiceName: serviceName,
		Method:      method,
		ArgsJSON:    argsJSON,
	})

	key := fmt.Sprintf("%s.%s", serviceName, method)
	m.callCount[key]++

	if err, exists := m.errorOnCall[key]; exists {
		return nil, err
	}

	if response, exists := m.responses[key]; exists {
		return response, nil
	}

	return nil, fmt.Errorf("no mock response configured for %s", key)
}

func (m *MockDatabaseService) SetResponse(serviceName, method string, response []byte) {
	key := fmt.Sprintf("%s.%s", serviceName, method)
	m.responses[key] = response
}

func (m *MockDatabaseService) SetError(serviceName, method string, err error) {
	key := fmt.Sprintf("%s.%s", serviceName, method)
	m.errorOnCall[key] = err
}

func (m *MockDatabaseService) GetCallCount(serviceName, method string) int {
	key := fmt.Sprintf("%s.%s", serviceName, method)
	return m.callCount[key]
}

func (m *MockDatabaseService) GetLastCall() *MockCall {
	if len(m.calls) == 0 {
		return nil
	}
	return &m.calls[len(m.calls)-1]
}

func (m *MockDatabaseService) GetCalls() []MockCall {
	return m.calls
}

func TestNewFirewallDatabase(t *testing.T) {
	mock := NewMockDatabaseService()
	db := NewFirewallDatabase(mock)

	require.NotNil(t, db)
	assert.False(t, db.IsInitialized())
}

func TestFirewallDatabase_IsInitialized(t *testing.T) {
	mock := NewMockDatabaseService()
	db := NewFirewallDatabase(mock)

	// Initially not initialized
	assert.False(t, db.IsInitialized())

	// Set initialized flag
	db.schemaInit = true
	assert.True(t, db.IsInitialized())

	// Reset
	db.ResetInitialization()
	assert.False(t, db.IsInitialized())
}

func TestFirewallDatabase_InitSchema_Success(t *testing.T) {
	mock := NewMockDatabaseService()
	db := NewFirewallDatabase(mock)

	// Mock successful Exec responses
	successResponse, _ := json.Marshal(map[string]interface{}{"rows_affected": 0})
	mock.SetResponse("database", "Exec", successResponse)

	ctx := context.Background()
	err := db.InitSchema(ctx)

	require.NoError(t, err)
	assert.True(t, db.IsInitialized())

	// Verify all schema statements were executed (1 table + 5 indexes = 6 calls)
	assert.Equal(t, 6, mock.GetCallCount("database", "Exec"))

	// Verify calls were made to database service
	calls := mock.GetCalls()
	require.Len(t, calls, 6)

	// Check first call creates the table
	var firstArgs map[string]interface{}
	err = json.Unmarshal(calls[0].ArgsJSON, &firstArgs)
	require.NoError(t, err)
	assert.Contains(t, firstArgs["query"], "CREATE TABLE IF NOT EXISTS firewall_logs")

	// Check index creation calls
	for i := 1; i < 6; i++ {
		var args map[string]interface{}
		err = json.Unmarshal(calls[i].ArgsJSON, &args)
		require.NoError(t, err)
		assert.Contains(t, args["query"], "CREATE INDEX IF NOT EXISTS")
	}
}

func TestFirewallDatabase_InitSchema_AlreadyInitialized(t *testing.T) {
	mock := NewMockDatabaseService()
	db := NewFirewallDatabase(mock)
	db.schemaInit = true // Already initialized

	ctx := context.Background()
	err := db.InitSchema(ctx)

	require.NoError(t, err)
	assert.Equal(t, 0, mock.GetCallCount("database", "Exec"), "Should not call database if already initialized")
}

func TestFirewallDatabase_InitSchema_NoDaemonService(t *testing.T) {
	db := NewFirewallDatabase(nil)

	ctx := context.Background()
	err := db.InitSchema(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "daemon service not available")
	assert.False(t, db.IsInitialized())
}

func TestFirewallDatabase_InitSchema_DatabaseError(t *testing.T) {
	mock := NewMockDatabaseService()
	db := NewFirewallDatabase(mock)

	// Mock error on first Exec call
	mock.SetError("database", "Exec", fmt.Errorf("database locked"))

	ctx := context.Background()
	err := db.InitSchema(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create schema")
	assert.False(t, db.IsInitialized())
}

func TestFirewallDatabase_QueryStats_Success(t *testing.T) {
	mock := NewMockDatabaseService()
	db := NewFirewallDatabase(mock)

	// Mock QueryRow responses
	totalResponse, _ := json.Marshal(map[string]interface{}{
		"columns": []string{"COUNT(*)"},
		"values":  []interface{}{float64(100)},
	})
	mock.SetResponse("database", "QueryRow", totalResponse)

	ctx := context.Background()
	stats, err := db.QueryStats(ctx)

	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, int64(100), stats.Total)

	// Verify calls were made
	assert.Equal(t, 2, mock.GetCallCount("database", "QueryRow"), "Should query total and drops")
}

func TestFirewallDatabase_QueryStats_ParsesDropsCorrectly(t *testing.T) {
	mock := NewMockDatabaseService()
	db := NewFirewallDatabase(mock)

	callCount := 0
	// Mock different responses for total and drops using custom handler
	mock.customHandler = func(ctx context.Context, serviceName string, method string, argsJSON []byte) ([]byte, error) {
		callCount++
		if callCount == 1 {
			// First call: total logs
			return json.Marshal(map[string]interface{}{
				"columns": []string{"COUNT(*)"},
				"values":  []interface{}{float64(150)},
			})
		}
		// Second call: drops
		return json.Marshal(map[string]interface{}{
			"columns": []string{"COUNT(*)"},
			"values":  []interface{}{float64(45)},
		})
	}

	ctx := context.Background()
	stats, err := db.QueryStats(ctx)

	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, int64(150), stats.Total)
	assert.Equal(t, int64(45), stats.Drops)
	assert.Equal(t, int64(105), stats.Accepts) // 150 - 45
}

func TestFirewallDatabase_QueryStats_EmptyResult(t *testing.T) {
	mock := NewMockDatabaseService()
	db := NewFirewallDatabase(mock)

	// Mock empty QueryRow responses
	emptyResponse, _ := json.Marshal(map[string]interface{}{
		"columns": []string{"COUNT(*)"},
		"values":  []interface{}{},
	})
	mock.SetResponse("database", "QueryRow", emptyResponse)

	ctx := context.Background()
	stats, err := db.QueryStats(ctx)

	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, int64(0), stats.Total)
	assert.Equal(t, int64(0), stats.Drops)
	assert.Equal(t, int64(0), stats.Accepts)
}

func TestFirewallDatabase_QueryStats_DatabaseError(t *testing.T) {
	mock := NewMockDatabaseService()
	db := NewFirewallDatabase(mock)

	mock.SetError("database", "QueryRow", fmt.Errorf("connection timeout"))

	ctx := context.Background()
	stats, err := db.QueryStats(ctx)

	require.Error(t, err)
	assert.Nil(t, stats)
	assert.Contains(t, err.Error(), "failed to query total logs")
}

func TestFirewallDatabase_QueryLogs_Success(t *testing.T) {
	mock := NewMockDatabaseService()
	db := NewFirewallDatabase(mock)

	// Mock Query response with sample logs
	queryResponse, _ := json.Marshal(map[string]interface{}{
		"columns": []string{"timestamp", "action", "src_ip", "dst_ip", "protocol", "src_port", "dst_port"},
		"rows": [][]interface{}{
			{"2025-01-15T10:00:00Z", "DROP", "192.168.1.100", "8.8.8.8", "TCP", float64(54321), float64(80)},
			{"2025-01-15T10:01:00Z", "ACCEPT", "10.0.0.50", "1.1.1.1", "UDP", float64(12345), float64(53)},
		},
	})
	mock.SetResponse("database", "Query", queryResponse)

	ctx := context.Background()
	logs, err := db.QueryLogs(ctx, 100)

	require.NoError(t, err)
	require.Len(t, logs, 2)

	// Verify first log entry
	assert.Equal(t, "2025-01-15T10:00:00Z", logs[0].Timestamp)
	assert.Equal(t, "DROP", logs[0].Action)
	assert.Equal(t, "192.168.1.100", logs[0].SrcIP)
	assert.Equal(t, "8.8.8.8", logs[0].DstIP)
	assert.Equal(t, "TCP", logs[0].Protocol)
	assert.Equal(t, int64(54321), logs[0].SrcPort)
	assert.Equal(t, int64(80), logs[0].DstPort)

	// Verify second log entry
	assert.Equal(t, "2025-01-15T10:01:00Z", logs[1].Timestamp)
	assert.Equal(t, "ACCEPT", logs[1].Action)
}

func TestFirewallDatabase_QueryLogs_EmptyResult(t *testing.T) {
	mock := NewMockDatabaseService()
	db := NewFirewallDatabase(mock)

	// Mock empty Query response
	emptyResponse, _ := json.Marshal(map[string]interface{}{
		"columns": []string{"timestamp", "action", "src_ip", "dst_ip", "protocol", "src_port", "dst_port"},
		"rows":    [][]interface{}{},
	})
	mock.SetResponse("database", "Query", emptyResponse)

	ctx := context.Background()
	logs, err := db.QueryLogs(ctx, 100)

	require.NoError(t, err)
	assert.Len(t, logs, 0)
}

func TestFirewallDatabase_QueryLogs_IncompleteRow(t *testing.T) {
	mock := NewMockDatabaseService()
	db := NewFirewallDatabase(mock)

	// Mock Query response with incomplete row (missing columns)
	queryResponse, _ := json.Marshal(map[string]interface{}{
		"columns": []string{"timestamp", "action", "src_ip"},
		"rows": [][]interface{}{
			{"2025-01-15T10:00:00Z", "DROP", "192.168.1.100"},                                              // Only 3 columns, need 7
			{"2025-01-15T10:01:00Z", "ACCEPT", "10.0.0.50", "1.1.1.1", "UDP", float64(12345), float64(53)}, // Complete
		},
	})
	mock.SetResponse("database", "Query", queryResponse)

	ctx := context.Background()
	logs, err := db.QueryLogs(ctx, 100)

	require.NoError(t, err)
	// Should skip incomplete row and only return the complete one
	require.Len(t, logs, 1)
	assert.Equal(t, "ACCEPT", logs[0].Action)
}

func TestFirewallDatabase_QueryLogs_DatabaseError(t *testing.T) {
	mock := NewMockDatabaseService()
	db := NewFirewallDatabase(mock)

	mock.SetError("database", "Query", fmt.Errorf("table does not exist"))

	ctx := context.Background()
	logs, err := db.QueryLogs(ctx, 100)

	require.Error(t, err)
	assert.Nil(t, logs)
	assert.Contains(t, err.Error(), "failed to query logs")
}

func TestFirewallDatabase_InsertLog_Success(t *testing.T) {
	mock := NewMockDatabaseService()
	db := NewFirewallDatabase(mock)

	// Mock successful Exec response
	successResponse, _ := json.Marshal(map[string]interface{}{"rows_affected": 1})
	mock.SetResponse("database", "Exec", successResponse)

	entry := &FirewallLogEntry{
		Timestamp:    "2025-01-15T10:00:00Z",
		Action:       "DROP",
		SrcIP:        "192.168.1.100",
		DstIP:        "8.8.8.8",
		Protocol:     "TCP",
		SrcPort:      54321,
		DstPort:      80,
		InterfaceIn:  "eth0",
		InterfaceOut: "eth1",
		PacketLength: 60,
	}

	ctx := context.Background()
	err := db.InsertLog(ctx, entry)

	require.NoError(t, err)
	assert.Equal(t, 1, mock.GetCallCount("database", "Exec"))

	// Verify the call was made with correct parameters
	lastCall := mock.GetLastCall()
	require.NotNil(t, lastCall)
	assert.Equal(t, "database", lastCall.ServiceName)
	assert.Equal(t, "Exec", lastCall.Method)

	// Parse args and verify
	var args map[string]interface{}
	err = json.Unmarshal(lastCall.ArgsJSON, &args)
	require.NoError(t, err)
	assert.Contains(t, args["query"], "INSERT INTO firewall_logs")

	// Verify args contains all expected values
	argsList := args["args"].([]interface{})
	require.Len(t, argsList, 10)
	assert.Equal(t, "2025-01-15T10:00:00Z", argsList[0])
	assert.Equal(t, "DROP", argsList[1])
	assert.Equal(t, "192.168.1.100", argsList[2])
	assert.Equal(t, "8.8.8.8", argsList[3])
	assert.Equal(t, "TCP", argsList[4])
}

func TestFirewallDatabase_InsertLog_DatabaseError(t *testing.T) {
	mock := NewMockDatabaseService()
	db := NewFirewallDatabase(mock)

	mock.SetError("database", "Exec", fmt.Errorf("database is locked"))

	entry := &FirewallLogEntry{
		Timestamp: "2025-01-15T10:00:00Z",
		Action:    "DROP",
		SrcIP:     "192.168.1.100",
		DstIP:     "8.8.8.8",
		Protocol:  "TCP",
	}

	ctx := context.Background()
	err := db.InsertLog(ctx, entry)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to insert log")
}

func TestFirewallDatabase_ResetInitialization(t *testing.T) {
	mock := NewMockDatabaseService()
	db := NewFirewallDatabase(mock)

	// Initialize
	db.schemaInit = true
	assert.True(t, db.IsInitialized())

	// Reset
	db.ResetInitialization()
	assert.False(t, db.IsInitialized())
}

func TestFirewallDatabase_QueryLogsFiltered_WithFilters(t *testing.T) {
	mock := NewMockDatabaseService()
	db := NewFirewallDatabase(mock)

	// Mock Query response with filtered logs
	queryResponse, _ := json.Marshal(map[string]interface{}{
		"columns": []string{"timestamp", "action", "src_ip", "dst_ip", "protocol", "src_port", "dst_port"},
		"rows": [][]interface{}{
			{"2025-01-15T10:00:00Z", "DROP", "192.168.1.100", "8.8.8.8", "TCP", float64(54321), float64(80)},
		},
	})
	mock.SetResponse("database", "Query", queryResponse)

	filter := &FirewallLogQuery{
		Action: "DROP",
		SrcIP:  "192.168.1.100",
		Limit:  20,
	}

	ctx := context.Background()
	logs, err := db.QueryLogsFiltered(ctx, filter)

	require.NoError(t, err)
	require.Len(t, logs, 1)
	assert.Equal(t, "DROP", logs[0].Action)
	assert.Equal(t, "192.168.1.100", logs[0].SrcIP)
}

func TestFirewallDatabase_QueryLogsFiltered_AllFilters(t *testing.T) {
	mock := NewMockDatabaseService()
	db := NewFirewallDatabase(mock)

	// Mock Query response
	queryResponse, _ := json.Marshal(map[string]interface{}{
		"columns": []string{"timestamp", "action", "src_ip", "dst_ip", "protocol", "src_port", "dst_port"},
		"rows":    [][]interface{}{},
	})
	mock.SetResponse("database", "Query", queryResponse)

	filter := &FirewallLogQuery{
		Action:       "ACCEPT",
		SrcIP:        "10.0.0.1",
		DstIP:        "8.8.8.8",
		Protocol:     "TCP",
		InterfaceIn:  "eth0",
		InterfaceOut: "eth1",
		Limit:        50,
	}

	ctx := context.Background()
	logs, err := db.QueryLogsFiltered(ctx, filter)

	require.NoError(t, err)
	assert.Len(t, logs, 0)

	// Verify query was called with correct arguments
	lastCall := mock.GetLastCall()
	require.NotNil(t, lastCall)

	var queryArgs map[string]interface{}
	err = json.Unmarshal(lastCall.ArgsJSON, &queryArgs)
	require.NoError(t, err)

	// Verify all filters were included in the query
	query := queryArgs["query"].(string)
	assert.Contains(t, query, "action = ?")
	assert.Contains(t, query, "src_ip = ?")
	assert.Contains(t, query, "dst_ip = ?")
	assert.Contains(t, query, "protocol = ?")
	assert.Contains(t, query, "interface_in = ?")
	assert.Contains(t, query, "interface_out = ?")
	assert.Contains(t, query, "LIMIT 50")

	// Verify args array contains all filter values
	args := queryArgs["args"].([]interface{})
	require.Len(t, args, 6)
	assert.Equal(t, "ACCEPT", args[0])
	assert.Equal(t, "10.0.0.1", args[1])
	assert.Equal(t, "8.8.8.8", args[2])
	assert.Equal(t, "TCP", args[3])
	assert.Equal(t, "eth0", args[4])
	assert.Equal(t, "eth1", args[5])
}

func TestFirewallDatabase_QueryLogsFiltered_DefaultLimit(t *testing.T) {
	mock := NewMockDatabaseService()
	db := NewFirewallDatabase(mock)

	// Mock Query response
	queryResponse, _ := json.Marshal(map[string]interface{}{
		"columns": []string{"timestamp", "action", "src_ip", "dst_ip", "protocol", "src_port", "dst_port"},
		"rows":    [][]interface{}{},
	})
	mock.SetResponse("database", "Query", queryResponse)

	filter := &FirewallLogQuery{
		Action: "DROP",
		Limit:  0, // Should default to 20
	}

	ctx := context.Background()
	_, err := db.QueryLogsFiltered(ctx, filter)

	require.NoError(t, err)

	// Verify default limit was applied
	lastCall := mock.GetLastCall()
	require.NotNil(t, lastCall)

	var queryArgs map[string]interface{}
	err = json.Unmarshal(lastCall.ArgsJSON, &queryArgs)
	require.NoError(t, err)

	query := queryArgs["query"].(string)
	assert.Contains(t, query, "LIMIT 20")
}

func TestFirewallDatabase_QueryLogsFiltered_EmptyFilter(t *testing.T) {
	mock := NewMockDatabaseService()
	db := NewFirewallDatabase(mock)

	// Mock Query response
	queryResponse, _ := json.Marshal(map[string]interface{}{
		"columns": []string{"timestamp", "action", "src_ip", "dst_ip", "protocol", "src_port", "dst_port"},
		"rows": [][]interface{}{
			{"2025-01-15T10:00:00Z", "DROP", "192.168.1.100", "8.8.8.8", "TCP", float64(54321), float64(80)},
			{"2025-01-15T10:01:00Z", "ACCEPT", "10.0.0.50", "1.1.1.1", "UDP", float64(12345), float64(53)},
		},
	})
	mock.SetResponse("database", "Query", queryResponse)

	// Empty filter should return all logs (up to limit)
	filter := &FirewallLogQuery{}

	ctx := context.Background()
	logs, err := db.QueryLogsFiltered(ctx, filter)

	require.NoError(t, err)
	require.Len(t, logs, 2)
}

func TestFirewallDatabase_QueryLogsFiltered_DatabaseError(t *testing.T) {
	mock := NewMockDatabaseService()
	db := NewFirewallDatabase(mock)

	mock.SetError("database", "Query", fmt.Errorf("connection timeout"))

	filter := &FirewallLogQuery{Action: "DROP"}

	ctx := context.Background()
	logs, err := db.QueryLogsFiltered(ctx, filter)

	require.Error(t, err)
	assert.Nil(t, logs)
	assert.Contains(t, err.Error(), "failed to query logs")
}
