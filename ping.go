package network

import (
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// PingResult contains the results of a ping operation
type PingResult struct {
	Host         string
	Sent         int
	Received     int
	Lost         int
	PacketLoss   float64 // Percentage
	MinRTT       time.Duration
	MaxRTT       time.Duration
	AvgRTT       time.Duration
	StdDevRTT    time.Duration
	Success      bool
	ErrorMessage string
}

// PingOptions configures ping behavior
type PingOptions struct {
	Count   int           // Number of packets to send (default: 4)
	Timeout time.Duration // Timeout for each packet (default: 4 seconds)
	Size    int           // Packet size in bytes (default: 32 on Windows, 56 on Linux)
}

// DefaultPingOptions returns default ping options
func DefaultPingOptions() *PingOptions {
	size := 56
	if runtime.GOOS == "windows" {
		size = 32
	}
	return &PingOptions{
		Count:   4,
		Timeout: 4 * time.Second,
		Size:    size,
	}
}

// Ping sends ICMP echo requests to a host and returns statistics
func Ping(host string, options *PingOptions) (*PingResult, error) {
	if host == "" {
		return nil, fmt.Errorf("host cannot be empty")
	}

	if options == nil {
		options = DefaultPingOptions()
	}

	// Validate options
	if options.Count <= 0 {
		options.Count = 4
	}
	if options.Timeout <= 0 {
		options.Timeout = 4 * time.Second
	}
	if options.Size <= 0 {
		options.Size = 32
	}

	result := &PingResult{
		Host: host,
	}

	var output []byte
	var err error

	if runtime.GOOS == "windows" {
		output, err = pingWindows(host, options)
	} else {
		output, err = pingLinux(host, options)
	}

	if err != nil {
		// Even if ping fails, try to parse partial output
		if output != nil && len(output) > 0 {
			if runtime.GOOS == "windows" {
				parseWindowsPingOutput(string(output), result)
			} else {
				parseLinuxPingOutput(string(output), result)
			}
		}
		
		// If we couldn't reach the host at all
		if result.Sent == 0 || result.Received == 0 {
			result.Success = false
			result.ErrorMessage = fmt.Sprintf("failed to ping %s: %v", host, err)
			return result, nil
		}
	}

	// Parse the output
	if runtime.GOOS == "windows" {
		parseWindowsPingOutput(string(output), result)
	} else {
		parseLinuxPingOutput(string(output), result)
	}

	// Calculate packet loss
	if result.Sent > 0 {
		result.Lost = result.Sent - result.Received
		result.PacketLoss = float64(result.Lost) / float64(result.Sent) * 100
		result.Success = result.Received > 0
	}

	return result, nil
}

// pingWindows executes ping command on Windows
func pingWindows(host string, options *PingOptions) ([]byte, error) {
	args := []string{
		"-n", strconv.Itoa(options.Count),
		"-w", strconv.Itoa(int(options.Timeout.Milliseconds())),
		"-l", strconv.Itoa(options.Size),
		host,
	}

	cmd := exec.Command("ping", args...)
	return cmd.CombinedOutput()
}

// pingLinux executes ping command on Linux
func pingLinux(host string, options *PingOptions) ([]byte, error) {
	// Find ping command
	pingCmd := findCommand("ping", []string{"/bin/ping", "/sbin/ping", "/usr/bin/ping", "/usr/sbin/ping"})
	if pingCmd == "" {
		pingCmd = "ping"
	}

	args := []string{
		"-c", strconv.Itoa(options.Count),
		"-W", strconv.Itoa(int(options.Timeout.Seconds())),
		"-s", strconv.Itoa(options.Size),
		host,
	}

	cmd := exec.Command(pingCmd, args...)
	return cmd.CombinedOutput()
}

// parseWindowsPingOutput parses Windows ping output
func parseWindowsPingOutput(output string, result *PingResult) {
	lines := strings.Split(output, "\n")

	// Parse packet statistics
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for packet statistics line
		// "Packets: Sent = 4, Received = 4, Lost = 0 (0% loss),"
		if strings.Contains(line, "Packets:") {
			re := regexp.MustCompile(`Sent = (\d+), Received = (\d+), Lost = (\d+)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) >= 4 {
				result.Sent, _ = strconv.Atoi(matches[1])
				result.Received, _ = strconv.Atoi(matches[2])
				result.Lost, _ = strconv.Atoi(matches[3])
			}
		}

		// Look for RTT statistics
		// "Minimum = 10ms, Maximum = 20ms, Average = 15ms"
		if strings.Contains(line, "Minimum") && strings.Contains(line, "Maximum") {
			re := regexp.MustCompile(`Minimum = (\d+)ms, Maximum = (\d+)ms, Average = (\d+)ms`)
			matches := re.FindStringSubmatch(line)
			if len(matches) >= 4 {
				if min, err := strconv.Atoi(matches[1]); err == nil {
					result.MinRTT = time.Duration(min) * time.Millisecond
				}
				if max, err := strconv.Atoi(matches[2]); err == nil {
					result.MaxRTT = time.Duration(max) * time.Millisecond
				}
				if avg, err := strconv.Atoi(matches[3]); err == nil {
					result.AvgRTT = time.Duration(avg) * time.Millisecond
				}
			}
		}
	}
}

// parseLinuxPingOutput parses Linux ping output
func parseLinuxPingOutput(output string, result *PingResult) {
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Parse packet statistics
		// "4 packets transmitted, 4 received, 0% packet loss, time 3003ms"
		if strings.Contains(line, "packets transmitted") {
			re := regexp.MustCompile(`(\d+) packets transmitted, (\d+) received`)
			matches := re.FindStringSubmatch(line)
			if len(matches) >= 3 {
				result.Sent, _ = strconv.Atoi(matches[1])
				result.Received, _ = strconv.Atoi(matches[2])
			}

			// Extract packet loss percentage
			re = regexp.MustCompile(`(\d+(?:\.\d+)?)% packet loss`)
			matches = re.FindStringSubmatch(line)
			if len(matches) >= 2 {
				result.PacketLoss, _ = strconv.ParseFloat(matches[1], 64)
			}
		}

		// Parse RTT statistics
		// "rtt min/avg/max/mdev = 0.035/0.048/0.062/0.011 ms"
		if strings.Contains(line, "rtt min/avg/max") {
			re := regexp.MustCompile(`(\d+(?:\.\d+)?)/(\d+(?:\.\d+)?)/(\d+(?:\.\d+)?)/(\d+(?:\.\d+)?)`)
			matches := re.FindStringSubmatch(line)
			if len(matches) >= 5 {
				if min, err := strconv.ParseFloat(matches[1], 64); err == nil {
					result.MinRTT = time.Duration(min * float64(time.Millisecond))
				}
				if avg, err := strconv.ParseFloat(matches[2], 64); err == nil {
					result.AvgRTT = time.Duration(avg * float64(time.Millisecond))
				}
				if max, err := strconv.ParseFloat(matches[3], 64); err == nil {
					result.MaxRTT = time.Duration(max * float64(time.Millisecond))
				}
				if stddev, err := strconv.ParseFloat(matches[4], 64); err == nil {
					result.StdDevRTT = time.Duration(stddev * float64(time.Millisecond))
				}
			}
		}
	}

	// Calculate lost packets if not already set
	if result.Sent > 0 && result.Lost == 0 {
		result.Lost = result.Sent - result.Received
	}
}

// String returns a formatted string representation of ping results
func (r *PingResult) String() string {
	var result strings.Builder

	result.WriteString(fmt.Sprintf("Ping statistics for %s:\n", r.Host))
	result.WriteString(strings.Repeat("-", 40) + "\n")

	if r.ErrorMessage != "" {
		result.WriteString(fmt.Sprintf("Error: %s\n", r.ErrorMessage))
	}

	result.WriteString(fmt.Sprintf("Packets: Sent = %d, Received = %d, Lost = %d (%.1f%% loss)\n",
		r.Sent, r.Received, r.Lost, r.PacketLoss))

	if r.Received > 0 {
		result.WriteString("Round Trip Times:\n")
		result.WriteString(fmt.Sprintf("  Minimum = %.2fms\n", float64(r.MinRTT.Microseconds())/1000))
		result.WriteString(fmt.Sprintf("  Maximum = %.2fms\n", float64(r.MaxRTT.Microseconds())/1000))
		result.WriteString(fmt.Sprintf("  Average = %.2fms\n", float64(r.AvgRTT.Microseconds())/1000))
		if r.StdDevRTT > 0 {
			result.WriteString(fmt.Sprintf("  StdDev  = %.2fms\n", float64(r.StdDevRTT.Microseconds())/1000))
		}
	}

	if r.Success {
		result.WriteString("Status: SUCCESS\n")
	} else {
		result.WriteString("Status: FAILED\n")
	}

	return result.String()
}