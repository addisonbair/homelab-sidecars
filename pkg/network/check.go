// Package network provides health checks for local network stack.
package network

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// Checker implements check.Checker for local network health.
// Validates that the network stack is functional WITHOUT external dependencies.
type Checker struct {
	// Gateway to check. If empty, auto-detected from default route.
	Gateway string
	// Timeout for gateway ping.
	Timeout time.Duration
}

// NewChecker creates a network health checker.
func NewChecker(gateway string, timeout time.Duration) *Checker {
	if timeout == 0 {
		timeout = 2 * time.Second
	}
	return &Checker{
		Gateway: gateway,
		Timeout: timeout,
	}
}

// Name returns the check name.
func (c *Checker) Name() string {
	return "network"
}

// Check validates local network stack health.
func (c *Checker) Check(ctx context.Context) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Step 1: Verify default route exists
	gateway := c.Gateway
	if gateway == "" {
		var err error
		gateway, err = getDefaultGateway()
		if err != nil {
			return fmt.Errorf("no default route: %w", err)
		}
	}

	// Step 2: Verify DNS resolver is configured
	if err := checkResolverConfigured(); err != nil {
		return err
	}

	// Step 3: Verify gateway is reachable (TCP connect, not ICMP - works without root)
	if err := c.checkGatewayReachable(ctx, gateway); err != nil {
		return fmt.Errorf("gateway %s unreachable: %w", gateway, err)
	}

	return nil
}

// getDefaultGateway extracts the default gateway from /proc/net/route
func getDefaultGateway() (string, error) {
	file, err := os.Open("/proc/net/route")
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Skip header
	scanner.Scan()

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		// Destination 00000000 = default route
		if fields[1] == "00000000" {
			// Gateway is in hex, little-endian
			return parseHexIP(fields[2])
		}
	}

	return "", fmt.Errorf("no default route found")
}

// parseHexIP converts a hex IP from /proc/net/route to dotted decimal
func parseHexIP(hex string) (string, error) {
	if len(hex) != 8 {
		return "", fmt.Errorf("invalid hex IP: %s", hex)
	}
	var octets [4]uint8
	for i := 0; i < 4; i++ {
		var val uint8
		_, err := fmt.Sscanf(hex[i*2:i*2+2], "%02X", &val)
		if err != nil {
			return "", err
		}
		// /proc/net/route uses little-endian
		octets[3-i] = val
	}
	return fmt.Sprintf("%d.%d.%d.%d", octets[0], octets[1], octets[2], octets[3]), nil
}

// checkResolverConfigured verifies /etc/resolv.conf has content
func checkResolverConfigured() error {
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return fmt.Errorf("cannot read resolv.conf: %w", err)
	}
	if len(data) == 0 {
		return fmt.Errorf("resolv.conf is empty")
	}
	// Check for at least one nameserver
	if !strings.Contains(string(data), "nameserver") {
		return fmt.Errorf("no nameserver configured")
	}
	return nil
}

// checkGatewayReachable attempts a TCP connection to a common port on the gateway.
// This is more reliable than ICMP ping which requires root privileges.
func (c *Checker) checkGatewayReachable(ctx context.Context, gateway string) error {
	// Try common ports that might be open on a router
	// We don't care if they're actually open, just that we can route to them
	ports := []string{"80", "443", "53", "22"}

	var lastErr error
	for _, port := range ports {
		addr := net.JoinHostPort(gateway, port)
		d := net.Dialer{Timeout: c.Timeout}
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err == nil {
			conn.Close()
			return nil
		}
		// Connection refused means we reached the gateway, port just isn't open
		if strings.Contains(err.Error(), "connection refused") {
			return nil
		}
		lastErr = err
	}

	// If all ports failed, check if it's a routing issue or host down
	// by looking at the error type
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("cannot reach gateway")
}
