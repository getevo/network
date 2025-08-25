package network

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

// DNSRecords holds all DNS record types for a domain
type DNSRecords struct {
	Domain string
	A      []string // IPv4 addresses
	AAAA   []string // IPv6 addresses
	CNAME  []string // Canonical names
	MX     []MXRecord
	NS     []string // Name servers
	TXT    []string // Text records (includes SPF)
	SOA    *SOARecord
	PTR    []string // Pointer records
}

// MXRecord represents a mail exchange record
type MXRecord struct {
	Host     string
	Priority uint16
}

// SOARecord represents a Start of Authority record
type SOARecord struct {
	NS      string // Primary name server
	Mbox    string // Responsible person's email
	Serial  uint32
	Refresh uint32
	Retry   uint32
	Expire  uint32
	MinTTL  uint32
}

// NSLookup converts a domain name to a list of IP addresses
func NSLookup(domain string) ([]string, error) {
	if domain == "" {
		return nil, fmt.Errorf("domain cannot be empty")
	}

	// Remove protocol if present
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimSuffix(domain, "/")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resolver := &net.Resolver{
		PreferGo: true,
	}

	ips, err := resolver.LookupHost(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup %s: %w", domain, err)
	}

	// Remove duplicates
	uniqueIPs := make(map[string]bool)
	var result []string
	for _, ip := range ips {
		if !uniqueIPs[ip] {
			uniqueIPs[ip] = true
			result = append(result, ip)
		}
	}

	return result, nil
}

// Resolve gets a domain and returns all DNS records
func Resolve(domain string) (*DNSRecords, error) {
	if domain == "" {
		return nil, fmt.Errorf("domain cannot be empty")
	}

	// Clean domain
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimSuffix(domain, "/")

	records := &DNSRecords{
		Domain: domain,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resolver := &net.Resolver{
		PreferGo: true,
	}

	// Get A records (IPv4)
	if addrs, err := resolver.LookupHost(ctx, domain); err == nil {
		for _, addr := range addrs {
			if ip := net.ParseIP(addr); ip != nil {
				if ip.To4() != nil {
					records.A = append(records.A, addr)
				} else {
					records.AAAA = append(records.AAAA, addr)
				}
			}
		}
	}

	// Get CNAME records
	if cname, err := resolver.LookupCNAME(ctx, domain); err == nil && cname != domain+"." {
		records.CNAME = append(records.CNAME, strings.TrimSuffix(cname, "."))
	}

	// Get MX records
	if mxRecords, err := resolver.LookupMX(ctx, domain); err == nil {
		for _, mx := range mxRecords {
			records.MX = append(records.MX, MXRecord{
				Host:     strings.TrimSuffix(mx.Host, "."),
				Priority: mx.Pref,
			})
		}
	}

	// Get NS records
	if nsRecords, err := resolver.LookupNS(ctx, domain); err == nil {
		for _, ns := range nsRecords {
			records.NS = append(records.NS, strings.TrimSuffix(ns.Host, "."))
		}
	}

	// Get TXT records (includes SPF)
	if txtRecords, err := resolver.LookupTXT(ctx, domain); err == nil {
		records.TXT = txtRecords
	}

	// Try to get SOA record
	records.SOA = lookupSOA(domain)

	// Get PTR records if the input is an IP
	if ip := net.ParseIP(domain); ip != nil {
		if names, err := resolver.LookupAddr(ctx, domain); err == nil {
			records.PTR = names
		}
	}

	return records, nil
}

// lookupSOA attempts to retrieve SOA record using DNS query
func lookupSOA(domain string) *SOARecord {
	// SOA records require more complex DNS queries
	// For now, we'll use the basic resolver capabilities
	resolver := &net.Resolver{
		PreferGo: true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to get NS records which often include SOA information
	if nsRecords, err := resolver.LookupNS(ctx, domain); err == nil && len(nsRecords) > 0 {
		// Return a basic SOA record with available information
		return &SOARecord{
			NS: strings.TrimSuffix(nsRecords[0].Host, "."),
		}
	}

	return nil
}

// String returns a formatted string representation of DNS records
func (r *DNSRecords) String() string {
	var result strings.Builder

	result.WriteString(fmt.Sprintf("DNS Records for %s:\n", r.Domain))
	result.WriteString(strings.Repeat("-", 50) + "\n")

	if len(r.A) > 0 {
		result.WriteString(fmt.Sprintf("A Records (IPv4):\n"))
		for _, a := range r.A {
			result.WriteString(fmt.Sprintf("  - %s\n", a))
		}
	}

	if len(r.AAAA) > 0 {
		result.WriteString(fmt.Sprintf("AAAA Records (IPv6):\n"))
		for _, aaaa := range r.AAAA {
			result.WriteString(fmt.Sprintf("  - %s\n", aaaa))
		}
	}

	if len(r.CNAME) > 0 {
		result.WriteString(fmt.Sprintf("CNAME Records:\n"))
		for _, cname := range r.CNAME {
			result.WriteString(fmt.Sprintf("  - %s\n", cname))
		}
	}

	if len(r.MX) > 0 {
		result.WriteString(fmt.Sprintf("MX Records (Mail):\n"))
		for _, mx := range r.MX {
			result.WriteString(fmt.Sprintf("  - Priority %d: %s\n", mx.Priority, mx.Host))
		}
	}

	if len(r.NS) > 0 {
		result.WriteString(fmt.Sprintf("NS Records (Name Servers):\n"))
		for _, ns := range r.NS {
			result.WriteString(fmt.Sprintf("  - %s\n", ns))
		}
	}

	if len(r.TXT) > 0 {
		result.WriteString(fmt.Sprintf("TXT Records (includes SPF):\n"))
		for _, txt := range r.TXT {
			result.WriteString(fmt.Sprintf("  - %s\n", txt))
		}
	}

	if r.SOA != nil {
		result.WriteString(fmt.Sprintf("SOA Record:\n"))
		result.WriteString(fmt.Sprintf("  - Primary NS: %s\n", r.SOA.NS))
		if r.SOA.Mbox != "" {
			result.WriteString(fmt.Sprintf("  - Contact: %s\n", r.SOA.Mbox))
		}
	}

	if len(r.PTR) > 0 {
		result.WriteString(fmt.Sprintf("PTR Records (Reverse DNS):\n"))
		for _, ptr := range r.PTR {
			result.WriteString(fmt.Sprintf("  - %s\n", ptr))
		}
	}

	return result.String()
}