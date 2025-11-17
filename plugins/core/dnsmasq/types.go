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

// Package dnsmasq implements DHCP and DNS services using dnsmasq.
package main

// DHCPConfig represents the DHCP server configuration (plugin-agnostic)
type DHCPConfig struct {
	Version      string              `json:"version"`
	Server       DHCPServer          `json:"server"`
	DHCPPools    map[string]DHCPPool `json:"dhcp_pools"`
	StaticLeases []StaticLease       `json:"static_leases"`
}

// DHCPServer represents global DHCP server settings (plugin-agnostic)
type DHCPServer struct {
	Enabled       bool   `json:"enabled"`
	Domain        string `json:"domain,omitempty"`
	LeaseFile     string `json:"leasefile,omitempty"`
	Authoritative bool   `json:"authoritative,omitempty"`
	LogDHCP       bool   `json:"log_dhcp,omitempty"`
	Comment       string `json:"comment,omitempty"`
}

// DHCPPool represents a DHCP address pool
type DHCPPool struct {
	Interface    string   `json:"interface"`
	Start        int      `json:"start"`
	Limit        int      `json:"limit"`
	LeaseTime    string   `json:"leasetime"`
	DHCPv4       string   `json:"dhcpv4"`
	DHCPv6       string   `json:"dhcpv6"`
	RA           string   `json:"ra"`
	RAManagement int      `json:"ra_management"`
	RADefault    int      `json:"ra_default"`
	DNS          []string `json:"dns"`
	Domain       string   `json:"domain"`
	Comment      string   `json:"comment"`
}

// StaticLease represents a static DHCP lease
type StaticLease struct {
	Name    string `json:"name"`
	MAC     string `json:"mac"`
	IP      string `json:"ip"`
	Comment string `json:"comment"`
}

// DNSConfig represents the DNS server configuration
type DNSConfig struct {
	Version string     `json:"version"`
	Server  DNSServer  `json:"server"`
	Records DNSRecords `json:"records"`
}

// DNSServer represents DNS server settings (plugin-agnostic)
type DNSServer struct {
	Enabled          bool     `json:"enabled"`
	Port             int      `json:"port"`
	Domain           string   `json:"domain,omitempty"`
	ExpandHosts      bool     `json:"expand_hosts,omitempty"`
	LocaliseQueries  bool     `json:"localise_queries,omitempty"`
	RebindProtection bool     `json:"rebind_protection,omitempty"`
	RebindLocalhost  bool     `json:"rebind_localhost,omitempty"`
	Authoritative    bool     `json:"authoritative,omitempty"`
	DomainNeeded     bool     `json:"domain_needed,omitempty"`
	BogusPriv        bool     `json:"bogus_priv,omitempty"`
	LogQueries       bool     `json:"log_queries,omitempty"`
	CacheSize        int      `json:"cache_size,omitempty"`
	Upstreams        []string `json:"upstreams,omitempty"` // Upstream DNS servers
	Comment          string   `json:"comment,omitempty"`
}

// DNSRecords represents custom DNS records
type DNSRecords struct {
	ARecords     []ARecord     `json:"a_records,omitempty"`
	AAAARecords  []AAAARecord  `json:"aaaa_records,omitempty"`
	CNAMERecords []CNAMERecord `json:"cname_records,omitempty"`
	MXRecords    []MXRecord    `json:"mx_records,omitempty"`
	SRVRecords   []SRVRecord   `json:"srv_records,omitempty"`
	TXTRecords   []TXTRecord   `json:"txt_records,omitempty"`
	PTRRecords   []PTRRecord   `json:"ptr_records,omitempty"`
}

// ARecord represents an IPv4 address record
type ARecord struct {
	Name    string `json:"name"`
	IP      string `json:"ip"`
	TTL     int    `json:"ttl,omitempty"`
	Comment string `json:"comment,omitempty"`
}

// AAAARecord represents an IPv6 address record
type AAAARecord struct {
	Name    string `json:"name"`
	IP      string `json:"ip"`
	TTL     int    `json:"ttl,omitempty"`
	Comment string `json:"comment,omitempty"`
}

// CNAMERecord represents a canonical name record
type CNAMERecord struct {
	CNAME   string `json:"cname"`
	Target  string `json:"target"`
	TTL     int    `json:"ttl,omitempty"`
	Comment string `json:"comment,omitempty"`
}

// MXRecord represents a mail exchange record
type MXRecord struct {
	Domain   string `json:"domain"`
	Server   string `json:"server"`
	Priority int    `json:"priority"`
	TTL      int    `json:"ttl,omitempty"`
	Comment  string `json:"comment,omitempty"`
}

// SRVRecord represents a service record
type SRVRecord struct {
	Service  string `json:"service"`
	Proto    string `json:"proto"`
	Domain   string `json:"domain"`
	Target   string `json:"target"`
	Port     int    `json:"port"`
	Priority int    `json:"priority"`
	Weight   int    `json:"weight"`
	TTL      int    `json:"ttl,omitempty"`
	Comment  string `json:"comment,omitempty"`
}

// TXTRecord represents a text record
type TXTRecord struct {
	Name    string `json:"name"`
	Text    string `json:"text"`
	TTL     int    `json:"ttl,omitempty"`
	Comment string `json:"comment,omitempty"`
}

// PTRRecord represents a pointer record (reverse DNS)
type PTRRecord struct {
	IP      string `json:"ip"`
	Name    string `json:"name"`
	TTL     int    `json:"ttl,omitempty"`
	Comment string `json:"comment,omitempty"`
}
