// health-check performs one-shot health checks for Greenboot integration.
// Exits 0 if all checks pass, 1 if any check fails.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/addisonbair/homelab-sidecars/pkg/check"
	"github.com/addisonbair/homelab-sidecars/pkg/jellyfin"
	"github.com/addisonbair/homelab-sidecars/pkg/network"
	"github.com/addisonbair/homelab-sidecars/pkg/raid"
)

func main() {
	// Global flags
	timeout := flag.Duration("timeout", 30*time.Second, "Overall timeout for all checks")

	// RAID flags
	raidArrays := flag.String("raid-arrays", "", "Comma-separated RAID arrays to check (e.g., md0,md1)")
	raidMdstat := flag.String("raid-mdstat", raid.DefaultMdstatPath, "Path to mdstat file")

	// Network flags
	networkEnabled := flag.Bool("network", true, "Enable network stack check")
	networkGateway := flag.String("network-gateway", "", "Gateway IP (auto-detect if empty)")

	// Jellyfin flags (for checking active streams before reboot)
	jellyfinURL := flag.String("jellyfin-url", "", "Jellyfin URL (skip if empty)")
	jellyfinKeyFile := flag.String("jellyfin-key-file", "", "Path to Jellyfin API key file")

	flag.Parse()

	// Build check list
	var checks []check.Checker

	// RAID check (if arrays specified)
	if *raidArrays != "" {
		arrays := strings.Split(*raidArrays, ",")
		for i := range arrays {
			arrays[i] = strings.TrimSpace(arrays[i])
		}
		checks = append(checks, raid.NewChecker(*raidMdstat, arrays))
	}

	// Network check (enabled by default)
	if *networkEnabled {
		checks = append(checks, network.NewChecker(*networkGateway, 2*time.Second))
	}

	// Jellyfin check (if configured)
	if *jellyfinURL != "" && *jellyfinKeyFile != "" {
		keyData, err := os.ReadFile(*jellyfinKeyFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cannot read Jellyfin key file: %v (skipping check)\n", err)
		} else {
			apiKey := strings.TrimSpace(string(keyData))
			client := jellyfin.NewClient(*jellyfinURL, apiKey, 5*time.Second)
			checks = append(checks, jellyfin.NewChecker(client))
		}
	}

	if len(checks) == 0 {
		fmt.Println("No checks configured")
		os.Exit(0)
	}

	// Run checks with timeout
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	results := check.RunAll(ctx, checks)

	// Print results
	exitCode := 0
	for _, r := range results {
		if r.Healthy {
			fmt.Printf("✓ %s\n", r.Name)
		} else {
			fmt.Printf("✗ %s: %s\n", r.Name, r.Reason)
			exitCode = 1
		}
	}

	if exitCode == 0 {
		fmt.Println("All checks passed")
	} else {
		fmt.Println("Some checks failed")
	}

	os.Exit(exitCode)
}
