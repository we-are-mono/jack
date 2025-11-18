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
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/we-are-mono/jack/daemon"
	"github.com/we-are-mono/jack/daemon/logger"
)

var applyOnStart bool

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run Jack as a daemon",
	Long:  `Starts the Jack daemon which listens for commands on a Unix socket.`,
	Run:   runDaemon,
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.Flags().BoolVar(&applyOnStart, "apply", false, "Apply configuration on startup")
}

func runDaemon(cmd *cobra.Command, args []string) {
	// Check for existing daemon via PID file
	pidFile := os.Getenv("JACK_PID_FILE")
	if pidFile == "" {
		pidFile = "/var/run/jack.pid"
	}
	if err := checkExistingDaemon(pidFile); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
		os.Exit(1)
	}

	// Write our PID to file
	if err := writePIDFile(pidFile); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to write PID file: %v\n", err)
		os.Exit(1)
	}
	defer os.Remove(pidFile)

	// Initialize structured logger
	if err := initializeLogger(); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	server, err := daemon.NewServer()
	if err != nil {
		logger.Error("Failed to create server", logger.Field{Key: "error", Value: err.Error()})
		os.Exit(1)
	}

	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Shutting down...")
		if err := server.Stop(); err != nil {
			logger.Error("Failed to stop server", logger.Field{Key: "error", Value: err.Error()})
		}
		os.Exit(0)
	}()

	// Start server (with optional apply on startup)
	if err := server.Start(applyOnStart); err != nil {
		logger.Error("Server failed", logger.Field{Key: "error", Value: err.Error()})
		os.Exit(1)
	}
}

// checkExistingDaemon checks if another daemon is already running
func checkExistingDaemon(pidFile string) error {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No PID file exists, we're good to start
			return nil
		}
		// PID file exists but can't be read - warn but allow start
		return fmt.Errorf("PID file exists but cannot be read: %w (remove %s manually if daemon is not running)", err, pidFile)
	}

	// Parse PID from file
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		// Invalid PID in file - warn but allow start
		return fmt.Errorf("invalid PID in %s: %s (remove file manually if daemon is not running)", pidFile, pidStr)
	}

	// Check if process with this PID exists
	process, err := os.FindProcess(pid)
	if err != nil {
		// Process doesn't exist, safe to remove stale PID file
		os.Remove(pidFile)
		return nil
	}

	// Try to signal the process to see if it's actually running
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		// Process doesn't exist or we can't signal it, remove stale PID file
		os.Remove(pidFile)
		return nil
	}

	// Process exists and is running
	return fmt.Errorf("daemon already running with PID %d (stop it first or remove %s if it's stale)", pid, pidFile)
}

// writePIDFile writes the current process PID to a file
func writePIDFile(pidFile string) error {
	pid := os.Getpid()
	return os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", pid)), 0600)
}

// initializeLogger sets up the structured logger with default configuration
func initializeLogger() error {
	// Default configuration
	config := logger.Config{
		Level:     "info",
		Format:    "json",
		Component: "daemon",
	}

	// Determine outputs: try journald first, fall back to file
	useJournald := false

	// Check if systemd-cat is available
	if _, err := exec.LookPath("systemd-cat"); err == nil {
		useJournald = true
	}

	// Create backends
	var backends []logger.Backend
	emitter := logger.NewEmitter()

	if useJournald {
		journaldBackend, err := logger.NewJournaldBackend(config.Format)
		if err != nil {
			// Fall back to file if journald fails
			log.Printf("[WARN] Could not initialize journald backend: %v, falling back to file", err)
			useJournald = false
		} else {
			backends = append(backends, journaldBackend)
		}
	}

	if !useJournald {
		// Use file backend
		logFile := "/var/log/jack/jack.log"
		fileBackend, err := logger.NewFileBackend(logFile, config.Format)
		if err != nil {
			return fmt.Errorf("failed to initialize file backend: %w", err)
		}
		backends = append(backends, fileBackend)
	}

	// Initialize global logger
	logger.Init(config, backends, emitter)

	// Log initialization message
	if useJournald {
		logger.Info("Logging initialized",
			logger.Field{Key: "backend", Value: "journald"},
			logger.Field{Key: "format", Value: config.Format})
	} else {
		logger.Info("Logging initialized",
			logger.Field{Key: "backend", Value: "file"},
			logger.Field{Key: "file", Value: "/var/log/jack/jack.log"},
			logger.Field{Key: "format", Value: config.Format})
	}

	return nil
}
