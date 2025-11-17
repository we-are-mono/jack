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
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNetworkObserver_MarkChange(t *testing.T) {
	// Use temporary socket for testing
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "jack.sock")
	os.Setenv("JACK_SOCKET_PATH", socketPath)
	defer os.Unsetenv("JACK_SOCKET_PATH")

	server, err := NewServer()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Stop()

	observer := NewNetworkObserver(server)

	// Initially, no recent change
	if observer.isRecentChange() {
		t.Error("Expected no recent change on new observer")
	}

	// Mark a change
	observer.MarkChange()

	// Should now have a recent change
	if !observer.isRecentChange() {
		t.Error("Expected recent change after MarkChange()")
	}

	// Wait for debounce window to expire
	time.Sleep(1100 * time.Millisecond)

	// Should no longer have a recent change
	if observer.isRecentChange() {
		t.Error("Expected no recent change after debounce window expired")
	}
}

func TestNetworkObserver_NewObserver(t *testing.T) {
	// Use temporary socket for testing
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "jack.sock")
	os.Setenv("JACK_SOCKET_PATH", socketPath)
	defer os.Unsetenv("JACK_SOCKET_PATH")

	server, err := NewServer()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Stop()

	observer := NewNetworkObserver(server)

	if observer.server != server {
		t.Error("Observer server reference incorrect")
	}

	if observer.linkCh == nil {
		t.Error("linkCh not initialized")
	}

	if observer.addrCh == nil {
		t.Error("addrCh not initialized")
	}

	if observer.routeCh == nil {
		t.Error("routeCh not initialized")
	}
}

func TestNetworkObserver_isRecentChange_ThreadSafety(t *testing.T) {
	// Use temporary socket for testing
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "jack.sock")
	os.Setenv("JACK_SOCKET_PATH", socketPath)
	defer os.Unsetenv("JACK_SOCKET_PATH")

	server, err := NewServer()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Stop()

	observer := NewNetworkObserver(server)

	// Run concurrent reads and writes to test mutex safety
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			observer.MarkChange()
			time.Sleep(time.Millisecond)
		}
		close(done)
	}()

	for i := 0; i < 100; i++ {
		_ = observer.isRecentChange()
		time.Sleep(time.Millisecond)
	}

	<-done // Wait for writes to complete
}
