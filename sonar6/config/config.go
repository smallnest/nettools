// Package config defines the configuration types and validation logic
// for the bitflip6 IPv6 UDP probing tool, including role selection,
// IPv6-only address resolution, and port range management.
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

// Config holds all runtime parameters for the bitflip6 IPv6 probing tool.
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

	// ServerZone is the IPv6 zone ID (interface name) for link-local
	// server addresses. Empty for non-link-local addresses.
	ServerZone string

	// ClientZone is the IPv6 zone ID (interface name) for link-local
	// client addresses. Empty for non-link-local addresses.
	ClientZone string

	Verbose bool
}

// validateIPv6 checks that addr is a valid IPv6 address (not IPv4).
func validateIPv6(addr string) error {
	ip := net.ParseIP(addr)
	if ip == nil {
		return fmt.Errorf("invalid IPv6 address %q", addr)
	}
	if ip.To4() != nil {
		return fmt.Errorf("address %q is not an IPv6 address: IPv4 addresses are not accepted", addr)
	}
	if ip.To16() == nil {
		return fmt.Errorf("address %q is not an IPv6 address", addr)
	}
	return nil
}

// resolveLinkLocalZone detects whether addr is an IPv6 link-local
// unicast address and, if so, finds the network interface that owns
// it. It returns the zone identifier (interface name) or an empty
// string if the address is not link-local.
func resolveLinkLocalZone(addr string) (string, error) {
	ip := net.ParseIP(addr)
	if ip == nil {
		return "", fmt.Errorf("invalid IP address: %s", addr)
	}
	if !ip.IsLinkLocalUnicast() {
		return "", nil
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("failed to list network interfaces: %w", err)
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.Equal(ip) {
				return iface.Name, nil
			}
		}
	}
	return "", fmt.Errorf("link-local address %s not found on any interface", addr)
}

// resolveLocalIPv6 returns the first non-loopback IPv6 address associated
// with the current hostname.
func resolveLocalIPv6() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("failed to get hostname: %w", err)
	}
	addrs, err := net.LookupHost(hostname)
	if err != nil {
		return "", fmt.Errorf("failed to lookup %q: %w", hostname, err)
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip != nil && ip.To4() == nil && ip.To16() != nil && !ip.IsLoopback() {
			return addr, nil
		}
	}
	return "", fmt.Errorf("no IPv6 address found for hostname %q", hostname)
}

// Validate checks and fills in default values for the configuration.
// It auto-detects local IPv6 addresses when not explicitly provided and
// rejects IPv4 addresses with clear error messages.
func (c *Config) Validate() error {
	if c.Role != RoleServer && c.Role != RoleClient {
		return fmt.Errorf("invalid role %q: must be %q or %q", c.Role, RoleServer, RoleClient)
	}

	if c.Role == RoleServer {
		if c.ServerAddr() == "" {
			ip, err := resolveLocalIPv6()
			if err != nil {
				return fmt.Errorf("server address not provided and failed to resolve local IPv6: %w", err)
			}
			c.ServerAddrs = []string{ip}
		}
	}

	for _, addr := range c.ServerAddrs {
		if err := validateIPv6(addr); err != nil {
			return err
		}
	}
	if c.ServerAddr() != "" {
		zone, err := resolveLinkLocalZone(c.ServerAddr())
		if err != nil {
			return fmt.Errorf("server address %q: %w", c.ServerAddr(), err)
		}
		c.ServerZone = zone
	}

	if c.Role == RoleClient {
		if c.ClientAddr == "" {
			ip, err := resolveLocalIPv6()
			if err != nil {
				return fmt.Errorf("client address not provided and failed to resolve local IPv6: %w", err)
			}
			c.ClientAddr = ip
		}
	}

	if c.ClientAddr != "" {
		if err := validateIPv6(c.ClientAddr); err != nil {
			return err
		}
		zone, err := resolveLinkLocalZone(c.ClientAddr)
		if err != nil {
			return fmt.Errorf("client address %q: %w", c.ClientAddr, err)
		}
		c.ClientZone = zone
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
