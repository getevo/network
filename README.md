# Network Configuration Package

A Go package for retrieving network configuration information across different operating systems (Windows and Linux).

## Features

- **Cross-platform support**: Works on both Windows and Linux
- **Thread-safe singleton pattern**: Ensures consistent configuration access
- **Comprehensive network information**: Retrieves IP addresses, DNS servers, gateways, and more
- **Network diagnostics**: Ping hosts with RTT and packet loss statistics
- **DNS operations**: NSLookup and comprehensive DNS record resolution
- **Error handling**: Robust error handling with detailed error messages
- **Security**: Protection against command injection and safe command execution

## Installation

```bash
go get github.com/getevo/network
```

## Usage

### Basic Usage

```go
package main

import (
    "fmt"
    "log"
    "github.com/getevo/network"
)

func main() {
    // Get network configuration
    config, err := network.GetConfig()
    if err != nil {
        log.Fatal(err)
    }

    // Print network information
    fmt.Println(config.String())
    
    // Access individual fields
    fmt.Printf("Local IP: %s\n", config.LocalIP)
    fmt.Printf("Interface: %s\n", config.InterfaceName)
    fmt.Printf("MAC Address: %s\n", config.HardwareAddress)
}
```

### Refresh Configuration

```go
// Refresh network configuration (useful after network changes)
newConfig, err := network.RefreshConfig()
if err != nil {
    log.Fatal(err)
}
```

### Ping a Host

```go
// Ping with default options
result, err := network.Ping("google.com", nil)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Packet Loss: %.1f%%\n", result.PacketLoss)
fmt.Printf("Average RTT: %v\n", result.AvgRTT)

// Ping with custom options
options := &network.PingOptions{
    Count:   10,
    Timeout: 5 * time.Second,
    Size:    64,
}
result, err = network.Ping("8.8.8.8", options)
fmt.Println(result.String())
```

### DNS Lookup

```go
// Simple NS lookup - get IP addresses for a domain
ips, err := network.NSLookup("google.com")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("IP addresses: %v\n", ips)

// Comprehensive DNS resolution - get all DNS records
records, err := network.Resolve("example.com")
if err != nil {
    log.Fatal(err)
}
fmt.Println(records.String())

// Access specific record types
for _, mx := range records.MX {
    fmt.Printf("Mail server: %s (priority %d)\n", mx.Host, mx.Priority)
}
```

## API Reference

### Types

#### Network Struct

```go
type Network struct {
    LocalIP                       net.IP
    DNS                           []string
    SubnetMask                    net.IP
    DefaultGateway                net.IP
    DefaultGatewayHardwareAddress net.HardwareAddr
    InterfaceName                 string
    HardwareAddress               net.HardwareAddr
    Suffix                        string
    Interface                     *net.Interface
}
```

#### PingResult Struct

```go
type PingResult struct {
    Host         string
    Sent         int
    Received     int
    Lost         int
    PacketLoss   float64       // Percentage
    MinRTT       time.Duration
    MaxRTT       time.Duration
    AvgRTT       time.Duration
    StdDevRTT    time.Duration
    Success      bool
    ErrorMessage string
}
```

#### DNSRecords Struct

```go
type DNSRecords struct {
    Domain string
    A      []string    // IPv4 addresses
    AAAA   []string    // IPv6 addresses
    CNAME  []string    // Canonical names
    MX     []MXRecord  // Mail exchange records
    NS     []string    // Name servers
    TXT    []string    // Text records (includes SPF)
    SOA    *SOARecord  // Start of Authority
    PTR    []string    // Pointer records
}
```

### Functions

#### GetConfig() (*Network, error)

Returns the singleton instance of the network configuration. Thread-safe and returns the same instance on subsequent calls.

```go
config, err := network.GetConfig()
```

#### RefreshConfig() (*Network, error)

Forces a refresh of the network configuration, creating a new instance.

```go
config, err := network.RefreshConfig()
```

#### String() string

Returns a formatted string representation of the network configuration.

```go
fmt.Println(config.String())
```

#### Ping(host string, options *PingOptions) (*PingResult, error)

Sends ICMP echo requests to a host and returns detailed statistics including RTT and packet loss.

```go
result, err := network.Ping("google.com", nil)
```

#### NSLookup(domain string) ([]string, error)

Converts a domain name to a list of IP addresses.

```go
ips, err := network.NSLookup("google.com")
```

#### Resolve(domain string) (*DNSRecords, error)

Returns comprehensive DNS records for a domain including A, AAAA, CNAME, MX, NS, TXT, SOA, and PTR records.

```go
records, err := network.Resolve("example.com")
```

## Platform-Specific Behavior

### Windows
- Uses `ipconfig` and `arp` commands to gather network information
- Retrieves DNS servers from Windows network configuration

### Linux
- Uses `ip`, `ifconfig`, and `arp` commands
- Reads DHCP lease files for DNS information
- Automatically searches for commands in common locations

## Error Handling

The package includes comprehensive error handling for:
- Missing network commands
- Invalid network configurations
- Command execution failures
- Parsing errors

## Thread Safety

The package uses mutex locks to ensure thread-safe access to the singleton instance. Multiple goroutines can safely call `GetConfig()` concurrently.

## Testing

Run the test suite:

```bash
go test -v
```

Run benchmarks:

```bash
go test -bench=.
```

## Security Considerations

- Command injection protection for user-supplied interface names
- Safe parsing of command outputs with bounds checking
- Nil pointer protection throughout the codebase

## Requirements

- Go 1.20 or higher
- Administrative privileges may be required on some systems
- Linux systems need standard networking tools (`ip`, `ifconfig`, `arp`)
- Windows systems need standard Windows networking commands

## License

Please check the repository for license information.

## Contributing

Contributions are welcome! Please ensure all tests pass and add new tests for any new functionality.

## Known Limitations

- Some network information may not be available in containerized environments
- VPN connections may affect the detected network configuration
- Some Linux distributions may have networking tools in non-standard locations