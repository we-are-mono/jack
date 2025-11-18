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
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // Pure-Go SQLite3 driver
)

// DatabaseProvider manages the SQLite3 database
type DatabaseProvider struct {
	config *DatabaseConfig
	db     *sql.DB
}

// NewDatabaseProvider creates a new database provider
func NewDatabaseProvider(config *DatabaseConfig) (*DatabaseProvider, error) {
	provider := &DatabaseProvider{
		config: config,
	}

	if err := provider.connect(); err != nil {
		return nil, err
	}

	if err := provider.initializeSchema(); err != nil {
		provider.Close()
		return nil, err
	}

	return provider, nil
}

// connect establishes a connection to the SQLite database
func (p *DatabaseProvider) connect() error {
	// Ensure parent directory exists
	dbDir := filepath.Dir(p.config.DatabasePath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database connection
	db, err := sql.Open("sqlite", p.config.DatabasePath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	p.db = db
	log.Printf("[jack-plugin-sqlite3] Connected to database: %s", p.config.DatabasePath)
	return nil
}

// initializeSchema creates the database tables if they don't exist
func (p *DatabaseProvider) initializeSchema() error {
	// Create logs table
	logsTableSQL := `
		CREATE TABLE IF NOT EXISTS logs (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp  TEXT NOT NULL,
			level      TEXT NOT NULL,
			component  TEXT NOT NULL,
			message    TEXT NOT NULL,
			fields     TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON logs(timestamp);
		CREATE INDEX IF NOT EXISTS idx_logs_level ON logs(level);
		CREATE INDEX IF NOT EXISTS idx_logs_component ON logs(component);
	`

	if _, err := p.db.Exec(logsTableSQL); err != nil {
		return fmt.Errorf("failed to create logs table: %w", err)
	}

	// Create plugin_metadata table for extensibility
	metadataTableSQL := `
		CREATE TABLE IF NOT EXISTS plugin_metadata (
			plugin_name TEXT PRIMARY KEY,
			version     TEXT,
			last_update TEXT
		);
	`

	if _, err := p.db.Exec(metadataTableSQL); err != nil {
		return fmt.Errorf("failed to create plugin_metadata table: %w", err)
	}

	log.Println("[jack-plugin-sqlite3] Database schema initialized")
	return nil
}

// InsertLog inserts a log entry into the database
func (p *DatabaseProvider) InsertLog(timestamp, level, component, message, fields string) error {
	if !p.config.LogStorageEnabled {
		return nil
	}

	insertSQL := `INSERT INTO logs (timestamp, level, component, message, fields) VALUES (?, ?, ?, ?, ?)`
	_, err := p.db.Exec(insertSQL, timestamp, level, component, message, fields)
	if err != nil {
		return fmt.Errorf("failed to insert log: %w", err)
	}

	// TODO: Implement max_log_entries cleanup if configured
	return nil
}

// Close closes the database connection
func (p *DatabaseProvider) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// Stats returns database statistics
func (p *DatabaseProvider) Stats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get log count
	var logCount int
	err := p.db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&logCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count logs: %w", err)
	}
	stats["log_count"] = logCount

	// Get database file size
	fileInfo, err := os.Stat(p.config.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat database file: %w", err)
	}
	stats["database_size_bytes"] = fileInfo.Size()
	stats["database_path"] = p.config.DatabasePath

	return stats, nil
}

// LogEntry represents a single log entry from the database
type LogEntry struct {
	ID        int
	Timestamp string
	Level     string
	Component string
	Message   string
	Fields    string
}

// QueryLogs queries logs from the database with optional filters
func (p *DatabaseProvider) QueryLogs(level, component string, limit int) ([]LogEntry, error) {
	// Build query with filters
	query := "SELECT id, timestamp, level, component, message, fields FROM logs WHERE 1=1"
	args := []interface{}{}

	if level != "" {
		query += " AND level = ?"
		args = append(args, level)
	}

	if component != "" {
		query += " AND component = ?"
		args = append(args, component)
	}

	// Order by timestamp descending (newest first)
	query += " ORDER BY timestamp DESC"

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := p.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}
	defer rows.Close()

	var entries []LogEntry
	for rows.Next() {
		var entry LogEntry
		err := rows.Scan(&entry.ID, &entry.Timestamp, &entry.Level, &entry.Component, &entry.Message, &entry.Fields)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log entry: %w", err)
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating logs: %w", err)
	}

	return entries, nil
}

// Vacuum compacts the database to reclaim unused space
func (p *DatabaseProvider) Vacuum() error {
	_, err := p.db.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}
	return nil
}
