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

// Package daemon implements the Jack daemon server and IPC protocol.
package daemon

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/we-are-mono/jack/daemon/logger"
	"github.com/we-are-mono/jack/types"
	"golang.org/x/sys/unix"
)

// NetworkObserver monitors network configuration changes via netlink
// and detects when external processes modify interfaces, addresses, or routes.
type NetworkObserver struct {
	server            *Server
	linkCh            chan netlink.LinkUpdate
	addrCh            chan netlink.AddrUpdate
	routeCh           chan netlink.RouteUpdate
	lastChange        time.Time
	changeMutex       sync.RWMutex
	lastReconcile     time.Time
	reconcileMutex    sync.RWMutex
	autoReconcile     bool
	reconcileInterval time.Duration
}

// NewNetworkObserver creates a new network observer for the given server
func NewNetworkObserver(s *Server) *NetworkObserver {
	// Default configuration: observer enabled, auto-reconcile disabled
	autoReconcile := false
	reconcileInterval := 60 * time.Second // Default: 1 minute between reconciliations

	// Read observer config from jack.json if available
	jackConfig := s.state.GetCurrentJackConfig()
	if jackConfig != nil && jackConfig.Observer != nil {
		autoReconcile = jackConfig.Observer.AutoReconcile
		if jackConfig.Observer.ReconcileIntervalMS > 0 {
			reconcileInterval = time.Duration(jackConfig.Observer.ReconcileIntervalMS) * time.Millisecond
		}
	}

	if autoReconcile {
		logger.Info("Auto-reconciliation enabled",
			logger.Field{Key: "interval", Value: reconcileInterval.String()})
	}

	return &NetworkObserver{
		server:            s,
		linkCh:            make(chan netlink.LinkUpdate),
		addrCh:            make(chan netlink.AddrUpdate),
		routeCh:           make(chan netlink.RouteUpdate),
		lastChange:        time.Now().Add(-2 * time.Second), // Start with no recent changes
		lastReconcile:     time.Now().Add(-reconcileInterval),
		autoReconcile:     autoReconcile,
		reconcileInterval: reconcileInterval,
	}
}

// Run starts the network observer and blocks until done is closed
func (o *NetworkObserver) Run(done chan struct{}) error {
	monitorDone := make(chan struct{})

	// Subscribe to netlink events
	if err := netlink.LinkSubscribe(o.linkCh, monitorDone); err != nil {
		logger.Error("Failed to subscribe to link events",
			logger.Field{Key: "error", Value: err.Error()})
		return err
	}
	logger.Info("Subscribed to link events successfully")

	if err := netlink.AddrSubscribe(o.addrCh, monitorDone); err != nil {
		logger.Error("Failed to subscribe to address events",
			logger.Field{Key: "error", Value: err.Error()})
		close(monitorDone)
		return err
	}
	logger.Info("Subscribed to address events successfully")

	if err := netlink.RouteSubscribe(o.routeCh, monitorDone); err != nil {
		logger.Error("Failed to subscribe to route events",
			logger.Field{Key: "error", Value: err.Error()})
		close(monitorDone)
		return err
	}
	logger.Info("Subscribed to route events successfully")

	logger.Info("Network observer started - monitoring for external configuration changes")

	// Event loop
	for {
		select {
		case update := <-o.linkCh:
			o.handleLinkUpdate(update)
		case update := <-o.addrCh:
			o.handleAddrUpdate(update)
		case update := <-o.routeCh:
			o.handleRouteUpdate(update)
		case <-done:
			close(monitorDone)
			logger.Info("Network observer stopped")
			return nil
		}
	}
}

// handleLinkUpdate processes link (interface) change events
func (o *NetworkObserver) handleLinkUpdate(update netlink.LinkUpdate) {
	link := update.Link
	attrs := link.Attrs()

	// Ignore events from Jack's own changes (within 1 second)
	if o.isRecentChange() {
		return
	}

	flags := update.IfInfomsg.Flags

	// Determine state
	isUp := flags&unix.IFF_UP != 0
	isRunning := flags&unix.IFF_RUNNING != 0

	// Log the change
	logger.Info("Link change detected",
		logger.Field{Key: "interface", Value: attrs.Name},
		logger.Field{Key: "index", Value: attrs.Index},
		logger.Field{Key: "up", Value: isUp},
		logger.Field{Key: "running", Value: isRunning},
		logger.Field{Key: "mtu", Value: attrs.MTU},
		logger.Field{Key: "type", Value: link.Type()})

	// Phase 2: Compare with Jack's desired state
	if drift := o.checkLinkDrift(attrs.Name, isUp, attrs.MTU); drift != "" {
		logger.Warn("Configuration drift detected",
			logger.Field{Key: "drift", Value: drift})
		// Phase 3: Trigger reconciliation if enabled
		o.maybeReconcile()
	}
}

// handleAddrUpdate processes IP address change events
func (o *NetworkObserver) handleAddrUpdate(update netlink.AddrUpdate) {
	// Ignore events from Jack's own changes
	if o.isRecentChange() {
		return
	}

	// Get link name for better logging
	link, err := netlink.LinkByIndex(update.LinkIndex)
	linkName := "unknown"
	if err == nil {
		linkName = link.Attrs().Name
	}

	action := "modified"
	if update.NewAddr {
		action = "added"
	}

	logger.Info("Address change detected",
		logger.Field{Key: "action", Value: action},
		logger.Field{Key: "interface", Value: linkName},
		logger.Field{Key: "index", Value: update.LinkIndex},
		logger.Field{Key: "address", Value: update.LinkAddress.String()})

	// Phase 2: Compare with Jack's desired addresses
	if linkName != "unknown" && update.LinkAddress.IP != nil {
		if drift := o.checkAddressDrift(linkName, update.LinkAddress.IP.String(), update.NewAddr); drift != "" {
			logger.Warn("Configuration drift detected",
				logger.Field{Key: "drift", Value: drift})
			// Phase 3: Trigger reconciliation if enabled
			o.maybeReconcile()
		}
	}
}

// handleRouteUpdate processes routing table change events
func (o *NetworkObserver) handleRouteUpdate(update netlink.RouteUpdate) {
	// Ignore events from Jack's own changes
	if o.isRecentChange() {
		return
	}

	route := update.Route
	action := "modified"
	switch update.Type {
	case unix.RTM_NEWROUTE:
		action = "added"
	case unix.RTM_DELROUTE:
		action = "deleted"
	}

	// Get link name if available
	linkName := "unknown"
	if route.LinkIndex > 0 {
		if link, err := netlink.LinkByIndex(route.LinkIndex); err == nil {
			linkName = link.Attrs().Name
		}
	}

	logger.Info("Route change detected",
		logger.Field{Key: "action", Value: action},
		logger.Field{Key: "dst", Value: route.Dst},
		logger.Field{Key: "via", Value: route.Gw},
		logger.Field{Key: "dev", Value: linkName},
		logger.Field{Key: "table", Value: route.Table})

	// Phase 2: Compare with Jack's desired routes
	if drift := o.checkRouteDrift(&route, action); drift != "" {
		logger.Warn("Configuration drift detected",
			logger.Field{Key: "drift", Value: drift})
		// Phase 3: Trigger reconciliation if enabled
		o.maybeReconcile()
	}
}

// MarkChange records that Jack is about to make a network change.
// This prevents the observer from treating Jack's own changes as external modifications.
func (o *NetworkObserver) MarkChange() {
	o.changeMutex.Lock()
	defer o.changeMutex.Unlock()
	o.lastChange = time.Now()
}

// isRecentChange checks if a change was made by Jack recently (within 1 second)
func (o *NetworkObserver) isRecentChange() bool {
	o.changeMutex.RLock()
	defer o.changeMutex.RUnlock()
	return time.Since(o.lastChange) < time.Second
}

// checkLinkDrift compares the actual link state with Jack's desired configuration
// Returns a drift description if mismatch detected, empty string otherwise
func (o *NetworkObserver) checkLinkDrift(linkName string, isUp bool, actualMTU int) string {
	// Get Jack's desired interface configuration
	config := o.server.state.GetCurrentInterfaces()
	if config == nil || config.Interfaces == nil {
		return "" // No config to compare against
	}

	// Find the interface in Jack's config by matching device name
	var desiredIface *types.Interface
	var configName string
	for name, iface := range config.Interfaces {
		if iface.DeviceName == linkName || iface.Device == linkName {
			desiredIface = &iface
			configName = name
			break
		}
	}

	if desiredIface == nil {
		// Interface not managed by Jack, ignore
		return ""
	}

	// Check if interface should be enabled
	if desiredIface.Enabled && !isUp {
		return fmt.Sprintf("Interface %s (%s) is down but should be up", linkName, configName)
	}
	if !desiredIface.Enabled && isUp {
		return fmt.Sprintf("Interface %s (%s) is up but should be down", linkName, configName)
	}

	// Check MTU if specified
	if desiredIface.MTU > 0 && actualMTU != desiredIface.MTU {
		return fmt.Sprintf("Interface %s (%s) has MTU %d but should have %d", linkName, configName, actualMTU, desiredIface.MTU)
	}

	return "" // No drift detected
}

// checkAddressDrift compares the actual IP address with Jack's desired configuration
// Returns a drift description if mismatch detected, empty string otherwise
func (o *NetworkObserver) checkAddressDrift(linkName, ipAddr string, isNew bool) string {
	// Get Jack's desired interface configuration
	config := o.server.state.GetCurrentInterfaces()
	if config == nil || config.Interfaces == nil {
		return ""
	}

	// Find the interface in Jack's config
	var desiredIface *types.Interface
	var configName string
	for name, iface := range config.Interfaces {
		if iface.DeviceName == linkName || iface.Device == linkName {
			desiredIface = &iface
			configName = name
			break
		}
	}

	if desiredIface == nil {
		// Interface not managed by Jack
		return ""
	}

	// Check if the IP address matches Jack's configuration
	if desiredIface.IPAddr != "" {
		// Strip CIDR suffix from both for comparison (e.g., "10.0.0.1/24" -> "10.0.0.1")
		desiredIP := strings.Split(desiredIface.IPAddr, "/")[0]
		actualIP := strings.Split(ipAddr, "/")[0]

		if isNew && actualIP != desiredIP {
			return fmt.Sprintf("Interface %s (%s) has unexpected IP %s (expected %s)", linkName, configName, actualIP, desiredIP)
		}
	}

	return ""
}

// checkRouteDrift compares the actual route with Jack's desired configuration
// Returns a drift description if mismatch detected, empty string otherwise
func (o *NetworkObserver) checkRouteDrift(route *netlink.Route, action string) string {
	// Get Jack's desired routes configuration
	config := o.server.state.GetCurrentRoutes()
	if config == nil || config.Routes == nil {
		return ""
	}

	// Convert netlink route to comparable format
	var routeDst string
	if route.Dst == nil {
		routeDst = "default"
	} else {
		routeDst = route.Dst.String()
	}

	var routeGw string
	if route.Gw != nil {
		routeGw = route.Gw.String()
	}

	// Check if this route is managed by Jack
	for name, desiredRoute := range config.Routes {
		if !desiredRoute.Enabled {
			continue
		}

		// Normalize destination (support "default" keyword)
		desiredDst := desiredRoute.Destination
		if desiredDst == "default" {
			desiredDst = "0.0.0.0/0"
		}

		// Compare destination
		if !routeDestinationsMatch(routeDst, desiredDst) {
			continue
		}

		// This is a Jack-managed route - check for drift
		if action == "deleted" {
			return fmt.Sprintf("Route %s (%s) was deleted externally", name, desiredDst)
		}

		// Compare gateway if specified
		if desiredRoute.Gateway != "" && desiredRoute.Gateway != routeGw {
			return fmt.Sprintf("Route %s (%s) has gateway %s but should have %s", name, desiredDst, routeGw, desiredRoute.Gateway)
		}

		// Compare table if specified
		if desiredRoute.Table > 0 && route.Table != desiredRoute.Table {
			return fmt.Sprintf("Route %s (%s) is in table %d but should be in table %d", name, desiredDst, route.Table, desiredRoute.Table)
		}

		// Route matches desired config
		return ""
	}

	// Route not managed by Jack
	return ""
}

// routeDestinationsMatch checks if two route destinations match
// Handles CIDR normalization and special cases like "default"
func routeDestinationsMatch(actual, desired string) bool {
	// Normalize "default" to 0.0.0.0/0
	if actual == "default" {
		actual = "0.0.0.0/0"
	}
	if desired == "default" {
		desired = "0.0.0.0/0"
	}

	// Parse both as CIDR to handle different formats
	_, actualNet, err1 := net.ParseCIDR(actual)
	_, desiredNet, err2 := net.ParseCIDR(desired)

	if err1 != nil || err2 != nil {
		// Fallback to string comparison if parsing fails
		return actual == desired
	}

	return actualNet.String() == desiredNet.String()
}

// maybeReconcile triggers reconciliation if auto-reconcile is enabled and rate limit allows
func (o *NetworkObserver) maybeReconcile() {
	if !o.autoReconcile {
		return
	}

	o.reconcileMutex.Lock()
	defer o.reconcileMutex.Unlock()

	// Check rate limit
	if time.Since(o.lastReconcile) < o.reconcileInterval {
		logger.Info("Reconciliation rate limited",
			logger.Field{Key: "last_reconcile_ago", Value: time.Since(o.lastReconcile).String()},
			logger.Field{Key: "interval", Value: o.reconcileInterval.String()})
		return
	}

	// Update last reconciliation time
	o.lastReconcile = time.Now()

	// Trigger reconciliation
	logger.Info("Triggering automatic reconciliation due to drift")
	go o.reconcile()
}

// reconcile applies the committed configuration to fix drift
func (o *NetworkObserver) reconcile() {
	// Mark that Jack is making changes (prevents observer from detecting its own fix)
	o.MarkChange()

	// Execute apply via internal server method
	resp := o.server.handleApply()

	if resp.Success {
		logger.Info("Automatic reconciliation completed successfully")
	} else {
		logger.Error("Automatic reconciliation failed",
			logger.Field{Key: "error", Value: resp.Error})
	}
}
