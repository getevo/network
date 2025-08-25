package network

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPing(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		options *PingOptions
		wantErr bool
	}{
		{
			name:    "Ping localhost",
			host:    "127.0.0.1",
			options: DefaultPingOptions(),
			wantErr: false,
		},
		{
			name: "Ping with custom options",
			host: "127.0.0.1",
			options: &PingOptions{
				Count:   2,
				Timeout: 2 * time.Second,
				Size:    64,
			},
			wantErr: false,
		},
		{
			name:    "Ping Google DNS",
			host:    "8.8.8.8",
			options: DefaultPingOptions(),
			wantErr: false,
		},
		{
			name:    "Ping invalid host",
			host:    "999.999.999.999",
			options: DefaultPingOptions(),
			wantErr: false, // Returns result with error message
		},
		{
			name:    "Empty host",
			host:    "",
			options: DefaultPingOptions(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip external network tests in CI/restricted environments
			if (tt.host == "8.8.8.8" || strings.Contains(tt.host, "999")) && testing.Short() {
				t.Skip("Skipping external network test in short mode")
			}

			result, err := Ping(tt.host, tt.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("Ping() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if result == nil {
					t.Error("Ping() returned nil result")
					return
				}

				// Validate result structure
				if result.Host != tt.host {
					t.Errorf("Ping() host = %v, want %v", result.Host, tt.host)
				}

				// For localhost, we should have successful pings
				if tt.host == "127.0.0.1" && !result.Success {
					t.Error("Ping() to localhost should succeed")
				}

				// Check statistics are populated for successful pings
				if result.Success {
					if result.Sent == 0 {
						t.Error("Ping() sent packets should be > 0")
					}
					if result.Received == 0 {
						t.Error("Ping() received packets should be > 0 for successful ping")
					}
					if result.AvgRTT == 0 && result.Received > 0 {
						t.Error("Ping() average RTT should be > 0 for successful ping")
					}
				}

				// Validate packet loss calculation
				expectedLoss := float64(result.Lost) / float64(result.Sent) * 100
				if result.Sent > 0 && (result.PacketLoss < expectedLoss-0.1 || result.PacketLoss > expectedLoss+0.1) {
					t.Errorf("Ping() packet loss calculation error: got %.2f%%, expected %.2f%%",
						result.PacketLoss, expectedLoss)
				}
			}
		})
	}
}

func TestDefaultPingOptions(t *testing.T) {
	opts := DefaultPingOptions()

	if opts.Count != 4 {
		t.Errorf("DefaultPingOptions() Count = %v, want 4", opts.Count)
	}

	if opts.Timeout != 4*time.Second {
		t.Errorf("DefaultPingOptions() Timeout = %v, want 4s", opts.Timeout)
	}

	expectedSize := 56
	if runtime.GOOS == "windows" {
		expectedSize = 32
	}
	if opts.Size != expectedSize {
		t.Errorf("DefaultPingOptions() Size = %v, want %v", opts.Size, expectedSize)
	}
}

func TestPingResultString(t *testing.T) {
	result := &PingResult{
		Host:       "test.com",
		Sent:       4,
		Received:   3,
		Lost:       1,
		PacketLoss: 25.0,
		MinRTT:     5 * time.Millisecond,
		MaxRTT:     20 * time.Millisecond,
		AvgRTT:     12 * time.Millisecond,
		StdDevRTT:  3 * time.Millisecond,
		Success:    true,
	}

	str := result.String()
	if str == "" {
		t.Error("String() returned empty string")
	}

	// Check that all important information is present
	expectedStrings := []string{
		"test.com",
		"Sent = 4",
		"Received = 3",
		"Lost = 1",
		"25.0% loss",
		"Minimum",
		"Maximum",
		"Average",
		"SUCCESS",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(str, expected) {
			t.Errorf("String() output missing expected content: %s", expected)
		}
	}
}

func TestPingWindowsOutputParsing(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	output := `
Pinging 8.8.8.8 with 32 bytes of data:
Reply from 8.8.8.8: bytes=32 time=10ms TTL=117
Reply from 8.8.8.8: bytes=32 time=15ms TTL=117
Reply from 8.8.8.8: bytes=32 time=12ms TTL=117
Reply from 8.8.8.8: bytes=32 time=20ms TTL=117

Ping statistics for 8.8.8.8:
    Packets: Sent = 4, Received = 4, Lost = 0 (0% loss),
Approximate round trip times in milli-seconds:
    Minimum = 10ms, Maximum = 20ms, Average = 14ms
`

	result := &PingResult{Host: "8.8.8.8"}
	parseWindowsPingOutput(output, result)

	if result.Sent != 4 {
		t.Errorf("parseWindowsPingOutput() Sent = %v, want 4", result.Sent)
	}
	if result.Received != 4 {
		t.Errorf("parseWindowsPingOutput() Received = %v, want 4", result.Received)
	}
	if result.Lost != 0 {
		t.Errorf("parseWindowsPingOutput() Lost = %v, want 0", result.Lost)
	}
	if result.MinRTT != 10*time.Millisecond {
		t.Errorf("parseWindowsPingOutput() MinRTT = %v, want 10ms", result.MinRTT)
	}
	if result.MaxRTT != 20*time.Millisecond {
		t.Errorf("parseWindowsPingOutput() MaxRTT = %v, want 20ms", result.MaxRTT)
	}
	if result.AvgRTT != 14*time.Millisecond {
		t.Errorf("parseWindowsPingOutput() AvgRTT = %v, want 14ms", result.AvgRTT)
	}
}

func TestPingLinuxOutputParsing(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping Linux-specific test on non-Linux platform")
	}

	output := `PING 8.8.8.8 (8.8.8.8) 56(84) bytes of data.
64 bytes from 8.8.8.8: icmp_seq=1 ttl=117 time=10.5 ms
64 bytes from 8.8.8.8: icmp_seq=2 ttl=117 time=15.2 ms
64 bytes from 8.8.8.8: icmp_seq=3 ttl=117 time=12.8 ms
64 bytes from 8.8.8.8: icmp_seq=4 ttl=117 time=20.1 ms

--- 8.8.8.8 ping statistics ---
4 packets transmitted, 4 received, 0% packet loss, time 3003ms
rtt min/avg/max/mdev = 10.5/14.6/20.1/3.5 ms`

	result := &PingResult{Host: "8.8.8.8"}
	parseLinuxPingOutput(output, result)

	if result.Sent != 4 {
		t.Errorf("parseLinuxPingOutput() Sent = %v, want 4", result.Sent)
	}
	if result.Received != 4 {
		t.Errorf("parseLinuxPingOutput() Received = %v, want 4", result.Received)
	}
	if result.PacketLoss != 0 {
		t.Errorf("parseLinuxPingOutput() PacketLoss = %v, want 0", result.PacketLoss)
	}
}

func BenchmarkPing(b *testing.B) {
	opts := &PingOptions{
		Count:   1,
		Timeout: 1 * time.Second,
		Size:    32,
	}

	for i := 0; i < b.N; i++ {
		Ping("127.0.0.1", opts)
	}
}