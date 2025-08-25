package network

import (
	"strings"
	"testing"
	"time"
)

func TestNSLookup(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		wantErr bool
	}{
		{
			name:    "Valid domain - Google",
			domain:  "google.com",
			wantErr: false,
		},
		{
			name:    "Valid domain with protocol",
			domain:  "https://google.com",
			wantErr: false,
		},
		{
			name:    "Localhost",
			domain:  "127.0.0.1",
			wantErr: false,
		},
		{
			name:    "Invalid domain",
			domain:  "this-domain-definitely-does-not-exist-12345.com",
			wantErr: true,
		},
		{
			name:    "Empty domain",
			domain:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ips, err := NSLookup(tt.domain)
			if (err != nil) != tt.wantErr {
				t.Errorf("NSLookup() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(ips) == 0 {
				t.Errorf("NSLookup() returned no IPs for %s", tt.domain)
			}

			// Validate IP format
			if !tt.wantErr {
				for _, ip := range ips {
					if ip == "" {
						t.Errorf("NSLookup() returned empty IP")
					}
				}
			}
		})
	}
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		wantErr bool
		checks  func(*DNSRecords) bool
	}{
		{
			name:    "Google.com DNS records",
			domain:  "google.com",
			wantErr: false,
			checks: func(r *DNSRecords) bool {
				// Google should have A records
				return len(r.A) > 0 || len(r.AAAA) > 0
			},
		},
		{
			name:    "Domain with MX records",
			domain:  "gmail.com",
			wantErr: false,
			checks: func(r *DNSRecords) bool {
				// Gmail should have MX records
				return len(r.MX) > 0
			},
		},
		{
			name:    "Empty domain",
			domain:  "",
			wantErr: true,
			checks:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			records, err := Resolve(tt.domain)
			if (err != nil) != tt.wantErr {
				t.Errorf("Resolve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if records == nil {
					t.Error("Resolve() returned nil records")
					return
				}

				if records.Domain != strings.TrimPrefix(strings.TrimPrefix(tt.domain, "http://"), "https://") {
					t.Errorf("Resolve() domain = %v, want %v", records.Domain, tt.domain)
				}

				if tt.checks != nil && !tt.checks(records) {
					t.Errorf("Resolve() failed validation checks for %s", tt.domain)
					t.Logf("Records: %+v", records)
				}
			}
		})
	}
}

func TestDNSRecordsString(t *testing.T) {
	records := &DNSRecords{
		Domain: "example.com",
		A:      []string{"192.168.1.1", "192.168.1.2"},
		AAAA:   []string{"2001:db8::1"},
		CNAME:  []string{"alias.example.com"},
		MX: []MXRecord{
			{Host: "mail1.example.com", Priority: 10},
			{Host: "mail2.example.com", Priority: 20},
		},
		NS:  []string{"ns1.example.com", "ns2.example.com"},
		TXT: []string{"v=spf1 include:_spf.example.com ~all"},
		SOA: &SOARecord{
			NS: "ns1.example.com",
		},
		PTR: []string{"example.com"},
	}

	str := records.String()
	if str == "" {
		t.Error("String() returned empty string")
	}

	// Check that all record types are present in output
	expectedStrings := []string{
		"example.com",
		"A Records",
		"AAAA Records",
		"CNAME Records",
		"MX Records",
		"NS Records",
		"TXT Records",
		"SOA Record",
		"PTR Records",
		"192.168.1.1",
		"2001:db8::1",
		"mail1.example.com",
		"ns1.example.com",
		"v=spf1",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(str, expected) {
			t.Errorf("String() output missing expected content: %s", expected)
		}
	}
}

func TestMXRecord(t *testing.T) {
	mx := MXRecord{
		Host:     "mail.example.com",
		Priority: 10,
	}

	if mx.Host != "mail.example.com" {
		t.Errorf("MXRecord.Host = %v, want mail.example.com", mx.Host)
	}

	if mx.Priority != 10 {
		t.Errorf("MXRecord.Priority = %v, want 10", mx.Priority)
	}
}

func BenchmarkNSLookup(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NSLookup("google.com")
	}
}

func BenchmarkResolve(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Resolve("google.com")
	}
}

func TestNSLookupConcurrent(t *testing.T) {
	domains := []string{"google.com", "github.com", "stackoverflow.com"}
	results := make(chan error, len(domains))

	for _, domain := range domains {
		go func(d string) {
			_, err := NSLookup(d)
			results <- err
		}(domain)
	}

	timeout := time.After(15 * time.Second)
	for i := 0; i < len(domains); i++ {
		select {
		case err := <-results:
			if err != nil {
				t.Errorf("Concurrent NSLookup failed: %v", err)
			}
		case <-timeout:
			t.Error("Concurrent NSLookup timed out")
		}
	}
}