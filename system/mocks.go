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

package system

import (
	"fmt"
	"sync"

	"github.com/vishvananda/netlink"
)

// MockNetlinkClient is a mock implementation of NetlinkClient for testing.
type MockNetlinkClient struct {
	mu sync.Mutex

	// State
	Links     map[string]netlink.Link
	Addresses map[string][]netlink.Addr
	Routes    []netlink.Route

	// Call counters for verification
	LinkByNameCalls     int
	LinkListCalls       int
	LinkAddCalls        int
	LinkDelCalls        int
	LinkSetUpCalls      int
	LinkSetDownCalls    int
	LinkSetMTUCalls     int
	LinkSetMasterCalls  int
	LinkSetNoMasterCalls int
	LinkSetNameCalls    int
	AddrListCalls       int
	AddrAddCalls        int
	AddrDelCalls        int
	RouteAddCalls       int
	RouteDelCalls       int
	RouteListCalls      int
	RouteListFilteredCalls int
	NeighListCalls      int

	// Error injection for testing error paths
	LinkByNameError     error
	LinkListError       error
	LinkAddError        error
	LinkDelError        error
	LinkSetUpError      error
	LinkSetDownError    error
	LinkSetMTUError     error
	LinkSetMasterError  error
	LinkSetNoMasterError error
	LinkSetNameError    error
	AddrListError       error
	AddrAddError        error
	AddrDelError        error
	RouteAddError       error
	RouteDelError       error
	RouteListError      error
	RouteListFilteredError error
	NeighListError      error
}

// NewMockNetlinkClient creates a new MockNetlinkClient.
func NewMockNetlinkClient() *MockNetlinkClient {
	return &MockNetlinkClient{
		Links:     make(map[string]netlink.Link),
		Addresses: make(map[string][]netlink.Addr),
		Routes:    make([]netlink.Route, 0),
	}
}

func (m *MockNetlinkClient) LinkByName(name string) (netlink.Link, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LinkByNameCalls++

	if m.LinkByNameError != nil {
		return nil, m.LinkByNameError
	}

	link, ok := m.Links[name]
	if !ok {
		return nil, fmt.Errorf("Link not found")
	}
	return link, nil
}

func (m *MockNetlinkClient) LinkByIndex(index int) (netlink.Link, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, link := range m.Links {
		if link.Attrs().Index == index {
			return link, nil
		}
	}
	return nil, fmt.Errorf("Link not found")
}

func (m *MockNetlinkClient) LinkList() ([]netlink.Link, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LinkListCalls++

	if m.LinkListError != nil {
		return nil, m.LinkListError
	}

	links := make([]netlink.Link, 0, len(m.Links))
	for _, link := range m.Links {
		links = append(links, link)
	}
	return links, nil
}

func (m *MockNetlinkClient) LinkAdd(link netlink.Link) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LinkAddCalls++

	if m.LinkAddError != nil {
		return m.LinkAddError
	}

	m.Links[link.Attrs().Name] = link
	return nil
}

func (m *MockNetlinkClient) LinkDel(link netlink.Link) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LinkDelCalls++

	if m.LinkDelError != nil {
		return m.LinkDelError
	}

	delete(m.Links, link.Attrs().Name)
	return nil
}

func (m *MockNetlinkClient) LinkSetUp(link netlink.Link) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LinkSetUpCalls++

	if m.LinkSetUpError != nil {
		return m.LinkSetUpError
	}

	return nil
}

func (m *MockNetlinkClient) LinkSetDown(link netlink.Link) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LinkSetDownCalls++

	if m.LinkSetDownError != nil {
		return m.LinkSetDownError
	}

	return nil
}

func (m *MockNetlinkClient) LinkSetMTU(link netlink.Link, mtu int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LinkSetMTUCalls++

	if m.LinkSetMTUError != nil {
		return m.LinkSetMTUError
	}

	return nil
}

func (m *MockNetlinkClient) LinkSetMaster(link netlink.Link, master netlink.Link) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LinkSetMasterCalls++

	if m.LinkSetMasterError != nil {
		return m.LinkSetMasterError
	}

	return nil
}

func (m *MockNetlinkClient) LinkSetNoMaster(link netlink.Link) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LinkSetNoMasterCalls++

	if m.LinkSetNoMasterError != nil {
		return m.LinkSetNoMasterError
	}

	return nil
}

func (m *MockNetlinkClient) LinkSetName(link netlink.Link, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.LinkSetNameCalls++

	if m.LinkSetNameError != nil {
		return m.LinkSetNameError
	}

	return nil
}

func (m *MockNetlinkClient) AddrList(link netlink.Link, family int) ([]netlink.Addr, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AddrListCalls++

	if m.AddrListError != nil {
		return nil, m.AddrListError
	}

	addrs, ok := m.Addresses[link.Attrs().Name]
	if !ok {
		return []netlink.Addr{}, nil
	}
	return addrs, nil
}

func (m *MockNetlinkClient) AddrAdd(link netlink.Link, addr *netlink.Addr) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AddrAddCalls++

	if m.AddrAddError != nil {
		return m.AddrAddError
	}

	name := link.Attrs().Name
	m.Addresses[name] = append(m.Addresses[name], *addr)
	return nil
}

func (m *MockNetlinkClient) AddrDel(link netlink.Link, addr *netlink.Addr) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.AddrDelCalls++

	if m.AddrDelError != nil {
		return m.AddrDelError
	}

	name := link.Attrs().Name
	addrs := m.Addresses[name]
	for i, a := range addrs {
		if a.IPNet.String() == addr.IPNet.String() {
			m.Addresses[name] = append(addrs[:i], addrs[i+1:]...)
			break
		}
	}
	return nil
}

func (m *MockNetlinkClient) RouteAdd(route *netlink.Route) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RouteAddCalls++

	if m.RouteAddError != nil {
		return m.RouteAddError
	}

	m.Routes = append(m.Routes, *route)
	return nil
}

func (m *MockNetlinkClient) RouteDel(route *netlink.Route) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RouteDelCalls++

	if m.RouteDelError != nil {
		return m.RouteDelError
	}

	for i, r := range m.Routes {
		if routesEqual(&r, route) {
			m.Routes = append(m.Routes[:i], m.Routes[i+1:]...)
			break
		}
	}
	return nil
}

func (m *MockNetlinkClient) RouteList(link netlink.Link, family int) ([]netlink.Route, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RouteListCalls++

	if m.RouteListError != nil {
		return nil, m.RouteListError
	}

	return m.Routes, nil
}

func (m *MockNetlinkClient) RouteListFiltered(family int, filter *netlink.Route, filterMask uint64) ([]netlink.Route, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RouteListFilteredCalls++

	if m.RouteListFilteredError != nil {
		return nil, m.RouteListFilteredError
	}

	return m.Routes, nil
}

func (m *MockNetlinkClient) NeighList(linkIndex, family int) ([]netlink.Neigh, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.NeighListCalls++

	if m.NeighListError != nil {
		return nil, m.NeighListError
	}

	return []netlink.Neigh{}, nil
}

// Helper function to compare routes
func routesEqual(r1, r2 *netlink.Route) bool {
	if r1.Dst == nil && r2.Dst == nil {
		return true
	}
	if r1.Dst == nil || r2.Dst == nil {
		return false
	}
	return r1.Dst.String() == r2.Dst.String() &&
		r1.Gw.Equal(r2.Gw) &&
		r1.LinkIndex == r2.LinkIndex
}

// MockSysctlClient is a mock implementation of SysctlClient for testing.
type MockSysctlClient struct {
	mu sync.Mutex

	// State
	Values map[string]string

	// Call counters
	GetCalls int
	SetCalls int

	// Error injection
	GetError error
	SetError error
}

// NewMockSysctlClient creates a new MockSysctlClient.
func NewMockSysctlClient() *MockSysctlClient {
	return &MockSysctlClient{
		Values: make(map[string]string),
	}
}

func (m *MockSysctlClient) Get(key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetCalls++

	if m.GetError != nil {
		return "", m.GetError
	}

	value, ok := m.Values[key]
	if !ok {
		return "", fmt.Errorf("sysctl key not found: %s", key)
	}
	return value, nil
}

func (m *MockSysctlClient) Set(key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SetCalls++

	if m.SetError != nil {
		return m.SetError
	}

	m.Values[key] = value
	return nil
}

// MockFilesystemClient is a mock implementation of FilesystemClient for testing.
type MockFilesystemClient struct {
	mu sync.Mutex

	// State
	Files map[string][]byte

	// Call counters
	ReadFileCalls  int
	WriteFileCalls int

	// Error injection
	ReadFileError  error
	WriteFileError error
}

// NewMockFilesystemClient creates a new MockFilesystemClient.
func NewMockFilesystemClient() *MockFilesystemClient {
	return &MockFilesystemClient{
		Files: make(map[string][]byte),
	}
}

func (m *MockFilesystemClient) ReadFile(filename string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ReadFileCalls++

	if m.ReadFileError != nil {
		return nil, m.ReadFileError
	}

	data, ok := m.Files[filename]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", filename)
	}
	return data, nil
}

func (m *MockFilesystemClient) WriteFile(filename string, data []byte, perm uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.WriteFileCalls++

	if m.WriteFileError != nil {
		return m.WriteFileError
	}

	m.Files[filename] = data
	return nil
}

// MockCommandRunner is a mock implementation of CommandRunner for testing.
type MockCommandRunner struct {
	mu sync.Mutex

	// State
	CommandOutputs map[string][]byte

	// Call tracking
	Commands [][]string
	RunCalls int

	// Error injection
	RunError error
}

// NewMockCommandRunner creates a new MockCommandRunner.
func NewMockCommandRunner() *MockCommandRunner {
	return &MockCommandRunner{
		CommandOutputs: make(map[string][]byte),
		Commands:       make([][]string, 0),
	}
}

func (m *MockCommandRunner) Run(name string, args ...string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RunCalls++

	// Track the command that was run
	cmd := append([]string{name}, args...)
	m.Commands = append(m.Commands, cmd)

	if m.RunError != nil {
		return nil, m.RunError
	}

	// Build command string for lookup
	cmdStr := name
	for _, arg := range args {
		cmdStr += " " + arg
	}

	output, ok := m.CommandOutputs[cmdStr]
	if !ok {
		return []byte{}, nil
	}
	return output, nil
}

// SetOutput sets the output for a specific command.
func (m *MockCommandRunner) SetOutput(name string, args []string, output []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cmdStr := name
	for _, arg := range args {
		cmdStr += " " + arg
	}
	m.CommandOutputs[cmdStr] = output
}
