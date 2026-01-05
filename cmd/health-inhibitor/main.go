// health-inhibitor continuously monitors system health and holds a systemd
// inhibitor lock when the system is not safe to reboot.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/addisonbair/homelab-sidecars/pkg/check"
	"github.com/addisonbair/homelab-sidecars/pkg/inhibitor"
	"github.com/addisonbair/homelab-sidecars/pkg/jellyfin"
	"github.com/addisonbair/homelab-sidecars/pkg/raid"
)

func main() {
	// Global flags
	interval := flag.Duration("interval", 30*time.Second, "Check interval")
	checkTimeout := flag.Duration("check-timeout", 10*time.Second, "Timeout for each check cycle")

	// Inhibitor flags
	inhibitorWho := flag.String("inhibitor-who", "health-inhibitor", "Inhibitor 'who' field")
	inhibitorWhat := flag.String("inhibitor-what", "shutdown:reboot", "What to inhibit")

	// RAID flags
	raidArrays := flag.String("raid-arrays", "", "Comma-separated RAID arrays to check (e.g., md0,md1)")
	raidMdstat := flag.String("raid-mdstat", raid.DefaultMdstatPath, "Path to mdstat file")

	// Jellyfin flags
	jellyfinURL := flag.String("jellyfin-url", "", "Jellyfin URL (skip if empty)")
	jellyfinKeyFile := flag.String("jellyfin-key-file", "", "Path to Jellyfin API key file")
	jellyfinGrace := flag.Duration("jellyfin-grace", 5*time.Minute, "Grace period after last stream before allowing reboot")

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
		log.Printf("Enabled RAID check for arrays: %v", arrays)
	}

	// Jellyfin check (if configured)
	if *jellyfinURL != "" && *jellyfinKeyFile != "" {
		keyData, err := os.ReadFile(*jellyfinKeyFile)
		if err != nil {
			log.Printf("Warning: cannot read Jellyfin key file: %v (Jellyfin check disabled)", err)
		} else {
			apiKey := strings.TrimSpace(string(keyData))
			client := jellyfin.NewClient(*jellyfinURL, apiKey, 5*time.Second)
			checks = append(checks, jellyfin.NewChecker(client, *jellyfinGrace))
			log.Printf("Enabled Jellyfin check at %s (grace=%s)", *jellyfinURL, *jellyfinGrace)
		}
	}

	if len(checks) == 0 {
		log.Fatal("No checks configured. Specify at least -raid-arrays or -jellyfin-url")
	}

	// Create inhibitor lock
	lock := inhibitor.New(*inhibitorWho, "System not safe for reboot")
	lock.What = *inhibitorWhat

	// Create runner
	runner := &check.Runner{
		Checks:   checks,
		Interval: *interval,
		Timeout:  *checkTimeout,
		Lock:     lock,
	}

	// Handle signals
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("Received signal %v, shutting down...", sig)
		cancel()
	}()

	log.Printf("Starting health-inhibitor (interval=%s, timeout=%s)", *interval, *checkTimeout)

	// Run until cancelled
	if err := runner.Run(ctx); err != nil && err != context.Canceled {
		log.Printf("Runner exited with error: %v", err)
	}

	log.Println("Shutdown complete")
}

func init() {
	// Configure log format
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.SetPrefix("[health-inhibitor] ")

	// Print usage on -help
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "health-inhibitor monitors system health and holds a systemd inhibitor\n")
		fmt.Fprintf(os.Stderr, "lock when the system is not safe to reboot.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Monitor RAID array md0\n")
		fmt.Fprintf(os.Stderr, "  %s -raid-arrays=md0\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Monitor RAID and Jellyfin streams\n")
		fmt.Fprintf(os.Stderr, "  %s -raid-arrays=md0 -jellyfin-url=http://localhost:8096 -jellyfin-key-file=/etc/homelab/jellyfin-api-key\n", os.Args[0])
	}
}
