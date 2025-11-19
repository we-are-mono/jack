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

// Package system provides low-level system integration for network, firewall, DHCP, and routing.
package system

import (
	"fmt"
	"net"
	"os"
	"os/exec"

	"github.com/vishvananda/netlink"
)

// NetlinkClient abstracts netlink operations for testability.
// This interface allows mocking of all netlink system calls.
type NetlinkClient interface {
	// Link operations
	LinkByName(name string) (netlink.Link, error)
	LinkByIndex(index int) (netlink.Link, error)
	LinkList() ([]netlink.Link, error)
	LinkAdd(link netlink.Link) error
	LinkDel(link netlink.Link) error
	LinkSetUp(link netlink.Link) error
	LinkSetDown(link netlink.Link) error
	LinkSetMTU(link netlink.Link, mtu int) error
	LinkSetMaster(link netlink.Link, master netlink.Link) error
	LinkSetNoMaster(link netlink.Link) error
	LinkSetName(link netlink.Link, name string) error

	// Address operations
	AddrList(link netlink.Link, family int) ([]netlink.Addr, error)
	AddrAdd(link netlink.Link, addr *netlink.Addr) error
	AddrDel(link netlink.Link, addr *netlink.Addr) error

	// Route operations
	RouteAdd(route *netlink.Route) error
	RouteDel(route *netlink.Route) error
	RouteList(link netlink.Link, family int) ([]netlink.Route, error)
	RouteListFiltered(family int, filter *netlink.Route, filterMask uint64) ([]netlink.Route, error)

	// Neighbor operations (for gateway resolution)
	NeighList(linkIndex, family int) ([]netlink.Neigh, error)
}

// SysctlClient abstracts sysctl operations for testability.
type SysctlClient interface {
	// Get reads a sysctl value
	Get(key string) (string, error)
	// Set writes a sysctl value
	Set(key, value string) error
}

// FilesystemClient abstracts filesystem operations for testability.
type FilesystemClient interface {
	// ReadFile reads the entire file content
	ReadFile(filename string) ([]byte, error)
	// WriteFile writes data to a file
	WriteFile(filename string, data []byte, perm uint32) error
}

// CommandRunner abstracts command execution for testability.
type CommandRunner interface {
	// Run executes a command and returns its combined output
	Run(name string, args ...string) ([]byte, error)
}

// DefaultNetlinkClient implements NetlinkClient using real netlink calls.
type DefaultNetlinkClient struct{}

// NewDefaultNetlinkClient creates a new DefaultNetlinkClient.
func NewDefaultNetlinkClient() *DefaultNetlinkClient {
	return &DefaultNetlinkClient{}
}

func (c *DefaultNetlinkClient) LinkByName(name string) (netlink.Link, error) {
	return netlink.LinkByName(name)
}

func (c *DefaultNetlinkClient) LinkByIndex(index int) (netlink.Link, error) {
	return netlink.LinkByIndex(index)
}

func (c *DefaultNetlinkClient) LinkList() ([]netlink.Link, error) {
	return netlink.LinkList()
}

func (c *DefaultNetlinkClient) LinkAdd(link netlink.Link) error {
	return netlink.LinkAdd(link)
}

func (c *DefaultNetlinkClient) LinkDel(link netlink.Link) error {
	return netlink.LinkDel(link)
}

func (c *DefaultNetlinkClient) LinkSetUp(link netlink.Link) error {
	return netlink.LinkSetUp(link)
}

func (c *DefaultNetlinkClient) LinkSetDown(link netlink.Link) error {
	return netlink.LinkSetDown(link)
}

func (c *DefaultNetlinkClient) LinkSetMTU(link netlink.Link, mtu int) error {
	return netlink.LinkSetMTU(link, mtu)
}

func (c *DefaultNetlinkClient) LinkSetMaster(link netlink.Link, master netlink.Link) error {
	return netlink.LinkSetMaster(link, master)
}

func (c *DefaultNetlinkClient) LinkSetNoMaster(link netlink.Link) error {
	return netlink.LinkSetNoMaster(link)
}

func (c *DefaultNetlinkClient) LinkSetName(link netlink.Link, name string) error {
	return netlink.LinkSetName(link, name)
}

func (c *DefaultNetlinkClient) AddrList(link netlink.Link, family int) ([]netlink.Addr, error) {
	return netlink.AddrList(link, family)
}

func (c *DefaultNetlinkClient) AddrAdd(link netlink.Link, addr *netlink.Addr) error {
	return netlink.AddrAdd(link, addr)
}

func (c *DefaultNetlinkClient) AddrDel(link netlink.Link, addr *netlink.Addr) error {
	return netlink.AddrDel(link, addr)
}

func (c *DefaultNetlinkClient) RouteAdd(route *netlink.Route) error {
	return netlink.RouteAdd(route)
}

func (c *DefaultNetlinkClient) RouteDel(route *netlink.Route) error {
	return netlink.RouteDel(route)
}

func (c *DefaultNetlinkClient) RouteList(link netlink.Link, family int) ([]netlink.Route, error) {
	return netlink.RouteList(link, family)
}

func (c *DefaultNetlinkClient) RouteListFiltered(family int, filter *netlink.Route, filterMask uint64) ([]netlink.Route, error) {
	return netlink.RouteListFiltered(family, filter, filterMask)
}

func (c *DefaultNetlinkClient) NeighList(linkIndex, family int) ([]netlink.Neigh, error) {
	return netlink.NeighList(linkIndex, family)
}

// DefaultSysctlClient implements SysctlClient using real sysctl operations.
type DefaultSysctlClient struct {
	fs FilesystemClient
}

// NewDefaultSysctlClient creates a new DefaultSysctlClient.
func NewDefaultSysctlClient(fs FilesystemClient) *DefaultSysctlClient {
	return &DefaultSysctlClient{fs: fs}
}

func (c *DefaultSysctlClient) Get(key string) (string, error) {
	path := "/proc/sys/" + replaceDotsWithSlash(key)
	data, err := c.fs.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (c *DefaultSysctlClient) Set(key, value string) error {
	path := "/proc/sys/" + replaceDotsWithSlash(key)
	return c.fs.WriteFile(path, []byte(value), 0600)
}

func replaceDotsWithSlash(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			result[i] = '/'
		} else {
			result[i] = s[i]
		}
	}
	return string(result)
}

// DefaultFilesystemClient implements FilesystemClient using real filesystem operations.
type DefaultFilesystemClient struct{}

// NewDefaultFilesystemClient creates a new DefaultFilesystemClient.
func NewDefaultFilesystemClient() *DefaultFilesystemClient {
	return &DefaultFilesystemClient{}
}

func (c *DefaultFilesystemClient) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

func (c *DefaultFilesystemClient) WriteFile(filename string, data []byte, perm uint32) error {
	return os.WriteFile(filename, data, os.FileMode(perm))
}

// DefaultCommandRunner implements CommandRunner using real command execution.
type DefaultCommandRunner struct{}

// NewDefaultCommandRunner creates a new DefaultCommandRunner.
func NewDefaultCommandRunner() *DefaultCommandRunner {
	return &DefaultCommandRunner{}
}

func (c *DefaultCommandRunner) Run(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}

// NetworkManager provides high-level network configuration operations
// with dependency injection for testability.
type NetworkManager struct {
	netlink NetlinkClient
	sysctl  SysctlClient
}

// NewNetworkManager creates a new NetworkManager with the given clients.
func NewNetworkManager(nl NetlinkClient, sc SysctlClient) *NetworkManager {
	return &NetworkManager{
		netlink: nl,
		sysctl:  sc,
	}
}

// NewDefaultNetworkManager creates a NetworkManager with real system clients.
func NewDefaultNetworkManager() *NetworkManager {
	fs := NewDefaultFilesystemClient()
	return &NetworkManager{
		netlink: NewDefaultNetlinkClient(),
		sysctl:  NewDefaultSysctlClient(fs),
	}
}

// RouteManager provides high-level route management operations
// with dependency injection for testability.
type RouteManager struct {
	netlink NetlinkClient
}

// NewRouteManager creates a new RouteManager with the given clients.
func NewRouteManager(nl NetlinkClient) *RouteManager {
	return &RouteManager{
		netlink: nl,
	}
}

// NewDefaultRouteManager creates a RouteManager with real system clients.
func NewDefaultRouteManager() *RouteManager {
	return &RouteManager{
		netlink: NewDefaultNetlinkClient(),
	}
}

// SnapshotManager provides system snapshot operations
// with dependency injection for testability.
type SnapshotManager struct {
	netlink NetlinkClient
	fs      FilesystemClient
	cmd     CommandRunner
}

// NewSnapshotManager creates a new SnapshotManager with the given clients.
func NewSnapshotManager(nl NetlinkClient, fs FilesystemClient, cmd CommandRunner) *SnapshotManager {
	return &SnapshotManager{
		netlink: nl,
		fs:      fs,
		cmd:     cmd,
	}
}

// NewDefaultSnapshotManager creates a SnapshotManager with real system clients.
func NewDefaultSnapshotManager() *SnapshotManager {
	fs := NewDefaultFilesystemClient()
	return &SnapshotManager{
		netlink: NewDefaultNetlinkClient(),
		fs:      fs,
		cmd:     NewDefaultCommandRunner(),
	}
}

// findInterfaceForGateway finds the interface that can reach the given gateway
// by checking which interface has an IP address on the same subnet as the gateway.
func (rm *RouteManager) findInterfaceForGateway(gateway net.IP) (int, error) {
	links, err := rm.netlink.LinkList()
	if err != nil {
		return 0, fmt.Errorf("failed to list interfaces: %w", err)
	}

	for _, link := range links {
		// Get addresses for this interface
		addrs, err := rm.netlink.AddrList(link, netlink.FAMILY_V4)
		if err != nil {
			continue
		}

		// Check if any address on this interface is on the same subnet as the gateway
		for _, addr := range addrs {
			if addr.IPNet.Contains(gateway) {
				return link.Attrs().Index, nil
			}
		}
	}

	return 0, fmt.Errorf("no interface found with network containing gateway %s", gateway.String())
}
