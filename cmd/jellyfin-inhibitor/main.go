// jellyfin-inhibitor monitors Jellyfin for active streaming sessions and holds
// a systemd inhibitor lock while users are watching, preventing system updates.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/addisonbair/homelab-sidecars/pkg/inhibitor"
	"github.com/addisonbair/homelab-sidecars/pkg/jellyfin"
)

func main() {
	url := flag.String("url", "http://localhost:8096", "Jellyfin server URL")
	apiKey := flag.String("api-key", "", "Jellyfin API key")
	apiKeyFile := flag.String("api-key-file", "", "file containing Jellyfin API key")
	interval := flag.Duration("interval", 30*time.Second, "check interval")
	timeout := flag.Duration("timeout", 10*time.Second, "API request timeout")
	verbose := flag.Bool("verbose", false, "verbose logging")
	flag.Parse()

	// Get API key from flag or file
	key := *apiKey
	if key == "" && *apiKeyFile != "" {
		data, err := os.ReadFile(*apiKeyFile)
		if err != nil {
			log.Fatalf("failed to read API key file: %v", err)
		}
		key = string(data)
	}
	if key == "" {
		log.Fatal("API key required: use -api-key or -api-key-file")
	}

	client := jellyfin.NewClient(*url, key, *timeout)
	lock := inhibitor.New("Jellyfin", "Active streaming session")

	// Handle shutdown gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	log.Printf("jellyfin-inhibitor starting: monitoring %s every %v", *url, *interval)

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	// Initial check
	checkAndUpdate(client, lock, *timeout, *verbose)

	for {
		select {
		case <-ticker.C:
			checkAndUpdate(client, lock, *timeout, *verbose)

		case sig := <-sigCh:
			log.Printf("received %v, releasing inhibitor and exiting", sig)
			lock.Release()
			os.Exit(0)
		}
	}
}

func checkAndUpdate(client *jellyfin.Client, lock *inhibitor.Lock, timeout time.Duration, verbose bool) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	hasStreams, sessions, err := client.HasActiveStreams(ctx)
	if err != nil {
		log.Printf("error checking Jellyfin: %v", err)
		// On error, don't change lock state (be conservative)
		return
	}

	if hasStreams {
		// Build description of active streams
		var desc string
		for i, s := range sessions {
			if i > 0 {
				desc += "; "
			}
			desc += s.Describe()
		}

		if !lock.IsHolding() {
			log.Printf("active streams detected, acquiring inhibitor: %s", desc)
			lock.Acquire(desc)
		} else if verbose {
			log.Printf("still streaming: %s", desc)
		}
	} else {
		if lock.IsHolding() {
			log.Printf("no active streams, releasing inhibitor")
			lock.Release()
		} else if verbose {
			log.Printf("no active streams")
		}
	}
}
