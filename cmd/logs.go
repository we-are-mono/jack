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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/we-are-mono/jack/client"
	"github.com/we-are-mono/jack/daemon"
	"github.com/we-are-mono/jack/daemon/logger"
)

var (
	logsFollow bool
	logsLines  int
	logsSince  string
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show Jack daemon logs",
	Long:  `Display logs from the Jack daemon using journalctl (systemd) or tail (non-systemd).`,
	Run:   runLogs,
}

var logsWatchCmd = &cobra.Command{
	Use:   "watch [level]",
	Short: "Watch logs in real-time from Jack daemon",
	Long:  `Stream logs from Jack daemon in real-time. Optionally filter by log level (debug, info, warn, error, alert).`,
	Run:   runLogsWatch,
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.AddCommand(logsWatchCmd)
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output in real-time")
	logsCmd.Flags().IntVarP(&logsLines, "lines", "n", 100, "Number of lines to show")
	logsCmd.Flags().StringVar(&logsSince, "since", "", "Show logs since time (e.g., '1 hour ago', '2024-01-01')")

	// Watch subcommand flags
	logsWatchCmd.Flags().StringVar(&logsComponent, "component", "", "Filter by component name")
}

var logsComponent string

func runLogs(cmd *cobra.Command, args []string) {
	// Check if journalctl is available (systemd)
	if _, err := exec.LookPath("journalctl"); err == nil {
		// Use journalctl
		runJournalctlLogs()
	} else {
		// Fall back to tail on log file
		runTailLogs()
	}
}

func runJournalctlLogs() {
	jcmd := []string{"journalctl", "-u", "jack"}

	if logsFollow {
		jcmd = append(jcmd, "-f")
	}

	if logsLines > 0 && !logsFollow {
		jcmd = append(jcmd, "-n", fmt.Sprintf("%d", logsLines))
	}

	if logsSince != "" {
		jcmd = append(jcmd, "--since", logsSince)
	}

	// Add --no-pager to prevent paging when not following
	if !logsFollow {
		jcmd = append(jcmd, "--no-pager")
	}

	execCmd := exec.Command(jcmd[0], jcmd[1:]...) //nolint:gosec // Command built from hardcoded journalctl with validated flags
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin

	if err := execCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to run journalctl: %v\n", err)
		os.Exit(1)
	}
}

func runTailLogs() {
	logFile := "/var/log/jack/jack.log"

	// Check if log file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "[ERROR] Log file not found: %s\n", logFile)
		fmt.Fprintf(os.Stderr, "[INFO] Make sure jack daemon is running or has been run at least once.\n")
		os.Exit(1)
	}

	// Build tail command
	tailCmd := []string{"tail"}

	if logsFollow {
		tailCmd = append(tailCmd, "-f")
	}

	if logsLines > 0 {
		tailCmd = append(tailCmd, "-n", fmt.Sprintf("%d", logsLines))
	}

	tailCmd = append(tailCmd, logFile)

	// Note: logsSince is not supported with tail
	if logsSince != "" {
		fmt.Fprintf(os.Stderr, "[WARN] --since flag is not supported without journalctl, ignoring\n")
	}

	execCmd := exec.Command(tailCmd[0], tailCmd[1:]...) //nolint:gosec // Command built from hardcoded tail with validated flags
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr
	execCmd.Stdin = os.Stdin

	if err := execCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[ERROR] Failed to run tail: %v\n", err)
		os.Exit(1)
	}
}

func runLogsWatch(cmd *cobra.Command, args []string) {
	// Build filter from arguments and flags
	filter := &daemon.LogFilter{
		Component: logsComponent,
	}

	// If first argument is provided, use it as level filter
	if len(args) > 0 {
		filter.Level = args[0]
	}

	// Set up signal handling for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Channel to signal completion
	done := make(chan error, 1)

	// Start streaming logs in background
	go func() {
		err := client.StreamLogs(filter, func(logData []byte) error {
			// Parse log entry
			var entry logger.Entry
			if err := json.Unmarshal(logData, &entry); err != nil {
				return fmt.Errorf("failed to parse log entry: %w", err)
			}

			// Format and print log entry
			fmt.Printf("[%s] [%s] %s: %s\n",
				entry.Timestamp,
				entry.Level,
				entry.Component,
				entry.Message)

			return nil
		})
		done <- err
	}()

	// Wait for either completion or Ctrl+C
	select {
	case err := <-done:
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
			os.Exit(1)
		}
	case <-sigChan:
		fmt.Println("\nStopping log stream...")
		return
	}
}
