package network

import (
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// Network is the interface which store network configuration data
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

var (
	instance *Network
	mu       sync.Mutex
)

// RefreshConfig refetch network configuration
func RefreshConfig() (*Network, error) {
	mu.Lock()
	instance = nil
	mu.Unlock()
	return GetConfig()
}

// GetConfig return  instance of network configuration.
func GetConfig() (*Network, error) {
	mu.Lock()
	defer mu.Unlock()

	if instance != nil {
		return instance, nil
	}
	network := Network{}

	if runtime.GOOS == "windows" {
		conn, err := net.Dial("udp", "8.8.8.8:80")
		if err != nil {
			return nil, err
		}
		defer conn.Close()

		localAddr := conn.LocalAddr()
		if udpAddr, ok := localAddr.(*net.UDPAddr); ok {
			network.LocalIP = udpAddr.IP
		} else {
			return nil, fmt.Errorf("failed to get local UDP address")
		}

		interfaces, err := net.Interfaces()
		if err != nil {
			return nil, fmt.Errorf("failed to get network interfaces: %w", err)
		}
		for _, interf := range interfaces {

			if addrs, err := interf.Addrs(); err == nil {
				for _, addr := range addrs {
					if strings.Contains(addr.String(), network.LocalIP.String()) {
						network.InterfaceName = interf.Name
						network.HardwareAddress = interf.HardwareAddr
						network.Interface = &interf
					}
				}
			}
		}

		err = network.getWindows()
		if err != nil {
			return nil, err
		}
	} else {
		err := network.getLinux()
		if err != nil {
			return nil, err
		}
	}
	instance = &network
	return &network, nil
}

// getLinux read network data for linux
func (network *Network) getLinux() error {
	// Try common locations for ip command
	ipCmd := findCommand("ip", []string{"/bin/ip", "/sbin/ip", "/usr/bin/ip", "/usr/sbin/ip"})
	if ipCmd == "" {
		return fmt.Errorf("ip command not found")
	}

	out, err := exec.Command(ipCmd, "route", "get", "8.8.8.8").Output()

	if err != nil {
		return err
	}
	parts := strings.Fields(string(out))
	if len(parts) < 7 {
		return fmt.Errorf("unexpected ip route output format")
	}
	network.DefaultGateway = net.ParseIP(parts[2])
	network.InterfaceName = parts[4]
	network.LocalIP = net.ParseIP(parts[6])

	interf, err := net.InterfaceByName(network.InterfaceName)
	if err == nil {
		network.HardwareAddress = interf.HardwareAddr
		network.Interface = interf

	} else {
		return err
	}

	// Try common locations for ifconfig command
	ifconfigCmd := findCommand("ifconfig", []string{"/sbin/ifconfig", "/bin/ifconfig", "/usr/sbin/ifconfig", "/usr/bin/ifconfig"})
	if ifconfigCmd == "" {
		// ifconfig might not be available, skip subnet mask detection
		// Some modern systems don't have ifconfig by default
		ifconfigCmd = "ifconfig"
	}

	out, err = exec.Command(ifconfigCmd, network.InterfaceName).Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")

		if len(lines) > 1 {
			fields := strings.Fields(strings.TrimSpace(lines[1]))
			if len(fields) > 4 {
				network.SubnetMask = net.ParseIP(fields[4])
			}
		}
	} else {
		return err
	}

	// Sanitize interface name to prevent command injection
	if strings.ContainsAny(network.InterfaceName, ";&|`$()\n") {
		return fmt.Errorf("invalid interface name")
	}
	leasePath := filepath.Join("/var/lib/dhcp", "dhclient."+network.InterfaceName+".leases")
	out, err = exec.Command("grep", "domain-name", leasePath).Output()

	if err == nil {
		dnslist := ""
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, line := range lines {
			if strings.Contains(line, "domain-name-servers") {
				trimmedLine := strings.TrimSpace(line)
				if len(trimmedLine) > 26 {
					line = strings.TrimRight(trimmedLine[26:], ";")
					list := strings.Split(line, ",")
					for _, dnsitem := range list {
						if !strings.Contains(dnslist, dnsitem) {
							dnslist += dnsitem + ","
						}
					}
				}

			} else {
				trimmedLine := strings.TrimSpace(line)
				if len(trimmedLine) > 18 {
					network.Suffix = strings.TrimRight(trimmedLine[18:], ";")
				}
			}
			dnslist = strings.TrimRight(dnslist, ",")
		}

		network.DNS = strings.Split(dnslist, ",")
	} else {
		return err
	}
	// Validate IP before using in command
	if network.DefaultGateway == nil {
		// Skip ARP lookup if no default gateway
		return nil
	}
	out, err = exec.Command("arp", "-e", network.DefaultGateway.String()).Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")

		if len(lines) >= 2 {
			fields := strings.Fields(lines[1])
			if len(fields) > 2 {
				network.DefaultGatewayHardwareAddress, _ = net.ParseMAC(fields[2])
			}
		}
	} else {
		return err
	}
	return nil
}

// String return network information as string
func (network *Network) String() string {
	res := "InterfaceName:" + network.InterfaceName + "\r\n"

	if network.HardwareAddress != nil {
		res += "HardwareAddress:" + network.HardwareAddress.String() + "\r\n"
	} else {
		res += "HardwareAddress:<nil>\r\n"
	}

	if network.LocalIP != nil {
		res += "LocalIP:" + network.LocalIP.String() + "\r\n"
	} else {
		res += "LocalIP:<nil>\r\n"
	}

	res += "DNS:" + strings.Join(network.DNS, ",") + "\r\n"

	if network.SubnetMask != nil {
		res += "SubnetMask:" + network.SubnetMask.String() + "\r\n"
	} else {
		res += "SubnetMask:<nil>\r\n"
	}

	if network.DefaultGateway != nil {
		res += "DefaultGateway:" + network.DefaultGateway.String() + "\r\n"
	} else {
		res += "DefaultGateway:<nil>\r\n"
	}

	if network.DefaultGatewayHardwareAddress != nil {
		res += "DefaultGatewayHardwareAddress:" + network.DefaultGatewayHardwareAddress.String() + "\r\n"
	} else {
		res += "DefaultGatewayHardwareAddress:<nil>\r\n"
	}

	res += "Suffix:" + network.Suffix + "\r\n"

	return res
}

// getWindows read network data in windows
func (network *Network) getWindows() error {
	out, err := exec.Command("ipconfig", "/all").Output()
	if err != nil {
		return err
	}
	items := strings.Split(string(out), "Ethernet adapter ")
	for _, item := range items {
		lines := strings.Split(item, "\r\n")
		if strings.HasPrefix(item, network.InterfaceName) {

			network.DNS = extractDotted(lines, "DNS Servers")
			if network.Suffix == "" {
				suffixes := extractDotted(lines, "Connection-specific DNS Suffix")
				if len(suffixes) > 0 {
					network.Suffix = suffixes[0]
				}
			}
			subnetMasks := extractDotted(lines, "Subnet Mask")
			if len(subnetMasks) > 0 {
				network.SubnetMask = net.ParseIP(subnetMasks[0])
			}

		}
		for _, line := range lines {
			if strings.Contains(line, "Default Gateway") {
				parts := strings.Split(line, ":")
				if len(parts) == 2 {
					ip := net.ParseIP(strings.TrimSpace(parts[1]))
					if ip != nil {
						network.DefaultGateway = ip
					}
				}
			}
		}
	}

	if network.DefaultGateway == nil {
		// Skip ARP lookup if no default gateway
		return nil
	}
	out, err = exec.Command("arp", "-a", network.DefaultGateway.String()).Output()
	if err != nil {
		return err
	}
	chunks := strings.Split(string(out), network.DefaultGateway.String())

	if len(chunks) >= 3 {
		fields := strings.Fields(chunks[2])
		if len(fields) > 0 {
			network.DefaultGatewayHardwareAddress, _ = net.ParseMAC(fields[0])
		}
	}
	return nil
}

// extractDotted extract data of ipconfig
func extractDotted(lines []string, key string) []string {
	result := ""
	found := false

	for _, line := range lines {
		if !found {
			if strings.HasPrefix(line, "   "+key) && len(line) > 39 {
				result = line[39:] + ""
				found = true
			}
		} else {
			if len(line) > 39 && strings.TrimSpace(line[0:39]) == "" {
				result += "," + strings.TrimSpace(line[39:])
			} else {
				break
			}
		}

	}

	return strings.Split(strings.Trim(result, ","), ",")
}

// findCommand searches for a command in common locations
func findCommand(name string, paths []string) string {
	for _, path := range paths {
		if _, err := exec.LookPath(path); err == nil {
			return path
		}
	}
	// Fallback to system PATH
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	return ""
}
