// Package config defines the configuration types and validation logic
// for the bitflip UDP probing tool, including role selection, address
// resolution, and port range management.
package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/baidu/nettools/stat"
)

// Role determines whether the program runs as a client or server.
type Role string

const (
	RoleServer Role = "server"
	RoleClient Role = "client"
)

// PortRange is an alias for stat.PortRange.
type PortRange = stat.PortRange

// ParsePortRange parses a "min,max" string into a PortRange.
func ParsePortRange(s string) (PortRange, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return PortRange{}, fmt.Errorf("invalid port range %q: expected min,max", s)
	}
	portMin, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return PortRange{}, fmt.Errorf("invalid min port in %q: %w", s, err)
	}
	portMax, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return PortRange{}, fmt.Errorf("invalid max port in %q: %w", s, err)
	}
	if portMin > portMax {
		return PortRange{}, fmt.Errorf("invalid port range %q: min > max", s)
	}
	if portMin < 1 || portMax > 65535 {
		return PortRange{}, fmt.Errorf("invalid port range %q: ports must be between 1 and 65535", s)
	}
	return PortRange{Min: portMin, Max: portMax}, nil
}

// Config holds all runtime parameters for the bitflip probing tool.
type Config struct {
	Role Role

	ClientAddr  string
	ClientAddrs []string
	ServerAddrs []string

	TOS             int
	ClientPortRange PortRange
	ServerPortRange PortRange
	RateInSpan      int64
	Span            time.Duration
	Delay           time.Duration
	MsgLen          int

	Count        int
	SendDuration time.Duration
	Verbose      bool
}

// resolveLocalIP returns the first non-loopback IP address associated
// with the current hostname. Falls back to the first address if none
// are non-loopback.
func resolveLocalIP() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("failed to get hostname: %w", err)
	}
	addrs, err := net.LookupHost(hostname)
	if err != nil {
		return "", fmt.Errorf("failed to lookup %q: %w", hostname, err)
	}
	for _, addr := range addrs {
		if ip := net.ParseIP(addr); ip != nil && !ip.IsLoopback() {
			return addr, nil
		}
	}
	if len(addrs) > 0 {
		return addrs[0], nil
	}
	return "", fmt.Errorf("no address found for hostname %q", hostname)
}

// Validate checks and fills in default values for the configuration.
// It auto-detects local IP addresses when not explicitly provided and
// sets sensible defaults for port ranges, rate, span, and delay.
func (c *Config) Validate() error {
	if c.Role != RoleServer && c.Role != RoleClient {
		return fmt.Errorf("invalid role %q: must be %q or %q", c.Role, RoleServer, RoleClient)
	}

	if c.Role == RoleServer {
		if c.ServerAddr() == "" {
			ip, err := resolveLocalIP()
			if err != nil {
				return fmt.Errorf("server address not provided and failed to resolve local IP: %w", err)
			}
			c.ServerAddrs = []string{ip}
		}
		if ip := net.ParseIP(c.ServerAddr()); ip == nil {
			return fmt.Errorf("invalid server address %q", c.ServerAddr())
		}
	}

	if c.ClientAddr == "" {
		ip, err := resolveLocalIP()
		if err != nil {
			return fmt.Errorf("client address not provided and failed to resolve local IP: %w", err)
		}
		c.ClientAddr = ip
	}

	if c.ClientPortRange.Min == 0 && c.ClientPortRange.Max == 0 {
		c.ClientPortRange = PortRange{Min: 43500, Max: 43599}
	}
	if c.ServerPortRange.Min == 0 && c.ServerPortRange.Max == 0 {
		c.ServerPortRange = PortRange{Min: 43500, Max: 43509}
	}
	if c.RateInSpan == 0 {
		c.RateInSpan = 5000
	}
	if c.Span == 0 {
		c.Span = time.Second
	}
	if c.Delay == 0 {
		c.Delay = 3 * time.Second
	}

	if c.Role == RoleClient && c.MsgLen <= 0 {
		return fmt.Errorf("msglen must be positive, got %d", c.MsgLen)
	}
	if c.Role == RoleServer && c.MsgLen <= 0 {
		c.MsgLen = 1024
	}

	return nil
}

// ServerAddr returns the first server address from ServerAddrs, or an
// empty string if none are configured.
func (c *Config) ServerAddr() string {
	if len(c.ServerAddrs) > 0 {
		return c.ServerAddrs[0]
	}
	return ""
}

// GetNextPorts advances the client and server port pair to the next values
// within their respective ranges, wrapping around when the maximum is reached.
func GetNextPorts(clientPort, serverPort uint16, clientPortRange, serverPortRange PortRange) (uint16, uint16) {
	serverPort++
	if serverPort > uint16(serverPortRange.Max) {
		serverPort = uint16(serverPortRange.Min)
		clientPort++
	}
	if clientPort > uint16(clientPortRange.Max) {
		clientPort = uint16(clientPortRange.Min)
	}
	return clientPort, serverPort
}
