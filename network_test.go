package network

import (
	"net"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestGetConfig(t *testing.T) {
	config, err := GetConfig()
	if err != nil {
		// In some environments (containers, VMs), network config might not be available
		t.Logf("GetConfig() error = %v (may be expected in restricted environments)", err)
		t.Skip("Skipping test due to restricted network environment")
	}

	if config == nil {
		t.Fatal("GetConfig() returned nil")
	}

	// Test singleton behavior
	config2, err := GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() second call error = %v", err)
	}

	if config != config2 {
		t.Error("GetConfig() should return the same instance")
	}
}

func TestRefreshConfig(t *testing.T) {
	// Get initial config
	config1, err := GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	// Refresh config
	config2, err := RefreshConfig()
	if err != nil {
		t.Fatalf("RefreshConfig() error = %v", err)
	}

	if config2 == nil {
		t.Fatal("RefreshConfig() returned nil")
	}

	// Verify new instance after refresh
	config3, err := GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() after refresh error = %v", err)
	}

	if config1 == config3 {
		t.Error("RefreshConfig() should create a new instance")
	}
}

func TestNetworkFields(t *testing.T) {
	config, err := GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	// Test LocalIP
	if config.LocalIP == nil {
		t.Error("LocalIP should not be nil")
	} else if config.LocalIP.IsLoopback() {
		t.Error("LocalIP should not be loopback address")
	}

	// Test InterfaceName
	if config.InterfaceName == "" {
		t.Error("InterfaceName should not be empty")
	}

	// Test Interface
	if config.Interface == nil {
		t.Error("Interface should not be nil")
	}

	// Test HardwareAddress
	if config.HardwareAddress == nil || len(config.HardwareAddress) == 0 {
		t.Error("HardwareAddress should not be nil or empty")
	}

	// Platform-specific tests
	if runtime.GOOS != "windows" {
		// On Linux, these fields should typically be populated
		if config.DefaultGateway == nil {
			t.Log("Warning: DefaultGateway is nil (might be expected in some environments)")
		}
	}
}

func TestString(t *testing.T) {
	config, err := GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	str := config.String()
	if str == "" {
		t.Error("String() should not return empty string")
	}

	// Check that all fields are represented
	expectedFields := []string{
		"InterfaceName:",
		"HardwareAddress:",
		"LocalIP:",
		"DNS:",
		"SubnetMask:",
		"DefaultGateway:",
		"DefaultGatewayHardwareAddress:",
		"Suffix:",
	}

	for _, field := range expectedFields {
		if !strings.Contains(str, field) {
			t.Errorf("String() missing field %s", field)
		}
	}
}

func TestConcurrency(t *testing.T) {
	// Reset instance for clean test
	instance = nil

	var wg sync.WaitGroup
	configs := make([]*Network, 10)
	errors := make([]error, 10)

	// Test concurrent GetConfig calls
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			configs[index], errors[index] = GetConfig()
		}(i)
	}

	wg.Wait()

	// Check all calls succeeded
	for i, err := range errors {
		if err != nil {
			t.Errorf("Concurrent GetConfig() call %d error = %v", i, err)
		}
	}

	// Check all returned the same instance
	for i := 1; i < len(configs); i++ {
		if configs[i] != configs[0] {
			t.Error("Concurrent GetConfig() calls should return the same instance")
		}
	}
}

func TestExtractDotted(t *testing.T) {
	lines := []string{
		"   Description . . . . . . . . . . . : Ethernet Adapter",
		"   Physical Address. . . . . . . . . : 00-11-22-33-44-55",
		"   DNS Servers . . . . . . . . . . . : 8.8.8.8",
		"                                        8.8.4.4",
		"                                        1.1.1.1",
		"   Default Gateway . . . . . . . . . : 192.168.1.1",
	}

	dns := extractDotted(lines, "DNS Servers")
	if len(dns) != 3 {
		t.Errorf("extractDotted() DNS expected 3 servers, got %d", len(dns))
	}

	if dns[0] != "8.8.8.8" || dns[1] != "8.8.4.4" || dns[2] != "1.1.1.1" {
		t.Errorf("extractDotted() DNS got unexpected values: %v", dns)
	}

	gateway := extractDotted(lines, "Default Gateway")
	if len(gateway) != 1 || gateway[0] != "192.168.1.1" {
		t.Errorf("extractDotted() Gateway expected [192.168.1.1], got %v", gateway)
	}

	// Test non-existent key
	notFound := extractDotted(lines, "NonExistent")
	if len(notFound) != 1 || notFound[0] != "" {
		t.Errorf("extractDotted() NonExistent expected empty result, got %v", notFound)
	}
}

func TestFindCommand(t *testing.T) {
	// Test with a command that should exist on most systems
	if runtime.GOOS != "windows" {
		paths := []string{"/bin/ls", "/usr/bin/ls"}
		cmd := findCommand("ls", paths)
		if cmd == "" {
			t.Log("Warning: ls command not found (might be expected in minimal environments)")
		}
	}

	// Test with non-existent command
	paths := []string{"/nonexistent/path", "/another/nonexistent"}
	cmd := findCommand("nonexistentcommand", paths)
	if cmd != "" {
		t.Errorf("findCommand() should return empty for non-existent command, got %s", cmd)
	}
}

func TestValidateIP(t *testing.T) {
	tests := []struct {
		ip    string
		valid bool
	}{
		{"192.168.1.1", true},
		{"8.8.8.8", true},
		{"::1", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		isValid := ip != nil
		if isValid != tt.valid {
			t.Errorf("ParseIP(%s) validity = %v, want %v", tt.ip, isValid, tt.valid)
		}
	}
}

func BenchmarkGetConfig(b *testing.B) {
	// First call to initialize
	GetConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetConfig()
	}
}

func BenchmarkString(b *testing.B) {
	config, err := GetConfig()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.String()
	}
}