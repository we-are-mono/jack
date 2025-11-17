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

// Package main provides a tool to merge multiple Go coverage profiles.
// It properly handles duplicate entries by combining coverage counts.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: mergecoverage <coverage1.out> <coverage2.out> ...")
		os.Exit(1)
	}

	// Map to store coverage: file:line -> count
	coverage := make(map[string]int)
	mode := ""

	// Read all coverage files
	for _, file := range os.Args[1:] {
		f, err := os.Open(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening %s: %v\n", file, err)
			os.Exit(1)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()

			// Skip mode line (we'll use the first one we see)
			if strings.HasPrefix(line, "mode:") {
				if mode == "" {
					mode = line
				}
				continue
			}

			// Parse coverage line: file.go:line.col,line.col statements count
			parts := strings.Fields(line)
			if len(parts) != 3 {
				continue
			}

			key := parts[0] + " " + parts[1] // file:line statements
			count := 0
			fmt.Sscanf(parts[2], "%d", &count)

			// Add coverage count (for atomic mode, any execution counts as covered)
			if count > 0 {
				coverage[key] = 1
			} else if _, exists := coverage[key]; !exists {
				coverage[key] = 0
			}
		}
	}

	// Output merged coverage
	fmt.Println(mode)
	for key, count := range coverage {
		fmt.Printf("%s %d\n", key, count)
	}
}
