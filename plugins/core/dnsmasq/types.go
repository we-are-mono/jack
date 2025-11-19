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

import (
	"fmt"

	"github.com/we-are-mono/jack/validation"
)

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

// Validate checks if the DHCPConfig is valid.
func (dc *DHCPConfig) Validate() error {
	v := validation.NewCollector()

	// Validate each pool
	for poolName, pool := range dc.DHCPPools {
		if err := pool.Validate(); err != nil {
			v.CheckMsg(err, fmt.Sprintf("DHCP pool %s", poolName))
		}
	}

	// Check for overlapping IP ranges on the same interface
	for poolName1, pool1 := range dc.DHCPPools {
		for poolName2, pool2 := range dc.DHCPPools {
			// Skip comparing pool with itself
			if poolName1 == poolName2 {
				continue
			}

			// Only check pools on the same interface
			if pool1.Interface != pool2.Interface {
				continue
			}

			// Check if ranges overlap
			// Pool range is [Start, Start + Limit - 1]
			end1 := pool1.Start + pool1.Limit - 1
			end2 := pool2.Start + pool2.Limit - 1

			// Ranges overlap if: start1 <= end2 && start2 <= end1
			if pool1.Start <= end2 && pool2.Start <= end1 {
				v.Check(fmt.Errorf("DHCP pools %s and %s have overlapping IP ranges on interface %s",
					poolName1, poolName2, pool1.Interface))
			}
		}
	}

	// Validate static leases
	for idx, lease := range dc.StaticLeases {
		if err := lease.Validate(); err != nil {
			v.CheckMsg(err, fmt.Sprintf("static lease %d", idx))
		}
	}

	return v.Error()
}

// Validate checks if the DHCPPool configuration is valid.
func (dp *DHCPPool) Validate() error {
	v := validation.NewCollector()

	if dp.Start <= 0 {
		v.Check(fmt.Errorf("start address must be positive"))
	}

	if dp.Limit <= 0 {
		v.Check(fmt.Errorf("limit must be positive"))
	}

	if dp.Limit > 65536 {
		v.Check(fmt.Errorf("limit %d exceeds reasonable maximum (65536)", dp.Limit))
	}

	if dp.Interface == "" {
		v.Check(fmt.Errorf("interface not specified"))
	}

	for _, dns := range dp.DNS {
		v.CheckMsg(validation.ValidateIP(dns), fmt.Sprintf("invalid DNS server %s", dns))
	}

	if dp.Domain != "" {
		v.CheckMsg(validation.ValidateDomain(dp.Domain), "invalid domain")
	}

	return v.Error()
}

// Validate checks if the StaticLease is valid.
func (sl *StaticLease) Validate() error {
	v := validation.NewCollector()

	v.CheckMsg(validation.ValidateMAC(sl.MAC), "invalid MAC address")
	v.CheckMsg(validation.ValidateIP(sl.IP), "invalid IP address")

	return v.Error()
}

// Validate checks if the DNSConfig is valid.
func (dn *DNSConfig) Validate() error {
	v := validation.NewCollector().WithContext("DNS server")

	if dn.Server.Port > 0 {
		v.CheckMsg(validation.ValidatePort(dn.Server.Port), "invalid port")
	}

	if dn.Server.Domain != "" {
		v.CheckMsg(validation.ValidateDomain(dn.Server.Domain), "invalid domain")
	}

	for _, upstream := range dn.Server.Upstreams {
		v.CheckMsg(validation.ValidateIP(upstream), fmt.Sprintf("invalid upstream %s", upstream))
	}

	// Validate DNS records
	for idx, record := range dn.Records.ARecords {
		if err := record.Validate(); err != nil {
			v.CheckMsg(err, fmt.Sprintf("A record %d", idx))
		}
	}

	for idx, record := range dn.Records.AAAARecords {
		if err := record.Validate(); err != nil {
			v.CheckMsg(err, fmt.Sprintf("AAAA record %d", idx))
		}
	}

	for idx, record := range dn.Records.CNAMERecords {
		if err := record.Validate(); err != nil {
			v.CheckMsg(err, fmt.Sprintf("CNAME record %d", idx))
		}
	}

	for idx, record := range dn.Records.PTRRecords {
		if err := record.Validate(); err != nil {
			v.CheckMsg(err, fmt.Sprintf("PTR record %d", idx))
		}
	}

	return v.Error()
}

// Validate checks if the ARecord is valid.
func (ar *ARecord) Validate() error {
	v := validation.NewCollector()

	v.CheckMsg(validation.ValidateDomain(ar.Name), "invalid name")
	v.CheckMsg(validation.ValidateIP(ar.IP), "invalid IP")

	return v.Error()
}

// Validate checks if the AAAARecord is valid.
func (ar *AAAARecord) Validate() error {
	v := validation.NewCollector()

	v.CheckMsg(validation.ValidateDomain(ar.Name), "invalid name")
	v.CheckMsg(validation.ValidateIP(ar.IP), "invalid IP")

	return v.Error()
}

// Validate checks if the CNAMERecord is valid.
func (cr *CNAMERecord) Validate() error {
	v := validation.NewCollector()

	v.CheckMsg(validation.ValidateDomain(cr.CNAME), "invalid CNAME")
	v.CheckMsg(validation.ValidateDomain(cr.Target), "invalid target")

	return v.Error()
}

// Validate checks if the PTRRecord is valid.
func (ptr *PTRRecord) Validate() error {
	v := validation.NewCollector()

	v.CheckMsg(validation.ValidateIP(ptr.IP), "invalid IP")
	v.CheckMsg(validation.ValidateDomain(ptr.Name), "invalid name")

	return v.Error()
}
