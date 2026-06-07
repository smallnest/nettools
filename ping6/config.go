// Package ping6 implements multi-target ICMPv6 Echo Request probing for IPv6.
// It sends ICMPv6 Echo Request packets via raw sockets using the goscapy library,
// receives Echo Replies, and reports per-target latency and loss statistics.
package ping6

import (
	"fmt"
	"net"
	"os"
	"time"
)

// Config holds all runtime parameters for the ICMPv6 ping tool.
type Config struct {
	TargetAddrs []string

	Rate         int64
	Span         time.Duration
	Delay        time.Duration
	Count        int
	SendDuration time.Duration

	Size     int
	TC       int // traffic class
	HopLimit int

	Timeout time.Duration

	LocalAddr string
	Interface string

	Verbose bool
	Hwts    bool
}

// Validate checks and fills in default values for the configuration.
// It auto-detects the local IPv6 address and interface when not explicitly provided.
func (c *Config) Validate() error {
	if len(c.TargetAddrs) == 0 {
		return fmt.Errorf("at least one target address is required")
	}
	for _, addr := range c.TargetAddrs {
		if err := validateIPv6(addr); err != nil {
			return err
		}
	}

	if c.LocalAddr == "" {
		ip, err := resolveLocalIPv6()
		if err != nil {
			return fmt.Errorf("local address not provided and failed to resolve local IPv6: %w", err)
		}
		c.LocalAddr = ip
	}
	if err := validateIPv6(c.LocalAddr); err != nil {
		return fmt.Errorf("invalid local IPv6 address: %w", err)
	}

	if c.Interface == "" {
		iface := findInterfaceByIPv6(c.LocalAddr)
		if iface == "" {
			return fmt.Errorf("cannot determine outgoing interface for %s, use --interface/-I", c.LocalAddr)
		}
		c.Interface = iface
	}

	if c.Rate == 0 {
		c.Rate = 100
	}
	if c.Span == 0 {
		c.Span = time.Second
	}
	if c.Delay == 0 {
		c.Delay = 3 * time.Second
	}
	if c.Size < 8 {
		c.Size = 64
	}
	if c.HopLimit == 0 {
		c.HopLimit = 64
	}
	if c.Timeout == 0 {
		c.Timeout = time.Second
	}

	return nil
}

// validateIPv6 checks that addr is a valid IPv6 address (not IPv4).
func validateIPv6(addr string) error {
	ip := net.ParseIP(addr)
	if ip == nil {
		return fmt.Errorf("invalid IPv6 address %q", addr)
	}
	if ip.To4() != nil {
		return fmt.Errorf("address %q is not an IPv6 address", addr)
	}
	return nil
}

// resolveLocalIPv6 returns the first non-loopback IPv6 address associated
// with the current hostname. Falls back to scanning network interfaces.
func resolveLocalIPv6() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		// Fall through to interface scan.
		return resolveLocalIPv6FromInterfaces()
	}
	addrs, err := net.LookupHost(hostname)
	if err == nil {
		for _, addr := range addrs {
			ip := net.ParseIP(addr)
			if ip != nil && ip.To4() == nil && !ip.IsLoopback() {
				return addr, nil
			}
		}
	}
	return resolveLocalIPv6FromInterfaces()
}

// resolveLocalIPv6FromInterfaces scans network interfaces for a non-loopback
// global unicast IPv6 address with an IPv6 default route.
func resolveLocalIPv6FromInterfaces() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("failed to list interfaces: %w", err)
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				ip := ipnet.IP
				if ip.To4() == nil && ip.To16() != nil && ip.IsGlobalUnicast() {
					return ip.String(), nil
				}
			}
		}
	}
	return "", fmt.Errorf("no global unicast IPv6 address found on any non-loopback interface")
}

// findInterfaceByIPv6 returns the interface name that has the given IPv6 address,
// preferring non-loopback interfaces.
func findInterfaceByIPv6(ipStr string) string {
	targetIP := net.ParseIP(ipStr)

	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	// If the IP is a loopback address, handle it directly.
	if targetIP != nil && targetIP.IsLoopback() {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagLoopback != 0 {
				return iface.Name
			}
		}
		return ""
	}

	// First pass: find a non-loopback interface with the exact IP.
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.Equal(targetIP) {
				return iface.Name
			}
		}
	}
	// Second pass: any interface (including loopback).
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.Equal(targetIP) {
				return iface.Name
			}
		}
	}
	return ""
}
