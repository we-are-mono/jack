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
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/we-are-mono/jack/daemon"
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
	pidFile := "/var/run/jack.pid"
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

	// Set up file logging
	logDir := "/var/log/jack"
	logFile := filepath.Join(logDir, "jack.log")

	// Create log directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("[WARN] Could not create log directory %s: %v", logDir, err)
		log.Println("[INFO] Logging to stdout only")
	} else {
		// Open log file
		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Printf("[WARN] Could not open log file %s: %v", logFile, err)
			log.Println("[INFO] Logging to stdout only")
		} else {
			// Redirect both log and stdout to both file and original stdout
			mw := io.MultiWriter(os.Stdout, file)
			log.SetOutput(mw)

			// IMPORTANT: Also redirect os.Stdout so fmt.Println() goes to log file
			// Save original stdout first
			originalStdout := os.Stdout
			r, w, err := os.Pipe()
			if err != nil {
				log.Printf("[WARN] Could not create pipe for stdout redirection: %v", err)
			} else {
				os.Stdout = w

				// Copy from pipe to multiwriter
				go func() {
					_, _ = io.Copy(mw, r) //nolint:errcheck // Error intentionally ignored - logging background task
				}()
			}

			log.Printf("[INFO] Logging to stdout and %s", logFile)

			// Ensure file and stdout are restored on exit
			defer func() {
				file.Close()
				os.Stdout = originalStdout
			}()
		}
	}

	server, err := daemon.NewServer()
	if err != nil {
		log.Fatalf("[ERROR] Failed to create server: %v", err)
	}

	// Handle shutdown gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down...")
		if err := server.Stop(); err != nil {
			log.Printf("[ERROR] Failed to stop server: %v", err)
		}
		os.Exit(0)
	}()

	// Start server (with optional apply on startup)
	if err := server.Start(applyOnStart); err != nil {
		log.Fatalf("[ERROR] Server failed: %v", err)
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
