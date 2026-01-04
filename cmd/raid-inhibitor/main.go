// raid-inhibitor monitors RAID array health and holds a systemd inhibitor
// lock when arrays are degraded or rebuilding, preventing system updates.
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/addisonbair/homelab-sidecars/pkg/inhibitor"
	"github.com/addisonbair/homelab-sidecars/pkg/raid"
)

func main() {
	mdstatPath := flag.String("mdstat", raid.DefaultMdstatPath, "path to mdstat file")
	arrays := flag.String("arrays", "md0", "comma-separated list of expected arrays")
	interval := flag.Duration("interval", 60*time.Second, "check interval")
	verbose := flag.Bool("verbose", false, "verbose logging")
	flag.Parse()

	expectedArrays := strings.Split(*arrays, ",")
	for i := range expectedArrays {
		expectedArrays[i] = strings.TrimSpace(expectedArrays[i])
	}

	lock := inhibitor.New("RAID Monitor", "RAID array unhealthy")

	// Handle shutdown gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	log.Printf("raid-inhibitor starting: monitoring %v every %v", expectedArrays, *interval)

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	// Initial check
	checkAndUpdate(lock, *mdstatPath, expectedArrays, *verbose)

	for {
		select {
		case <-ticker.C:
			checkAndUpdate(lock, *mdstatPath, expectedArrays, *verbose)

		case sig := <-sigCh:
			log.Printf("received %v, releasing inhibitor and exiting", sig)
			lock.Release()
			os.Exit(0)
		}
	}
}

func checkAndUpdate(lock *inhibitor.Lock, mdstatPath string, expectedArrays []string, verbose bool) {
	healthy, reason, err := raid.Check(mdstatPath, expectedArrays)
	if err != nil {
		log.Printf("error checking RAID: %v", err)
		// On error, acquire lock to be safe
		if !lock.IsHolding() {
			log.Printf("acquiring inhibitor due to check error")
			lock.Acquire("RAID check error: " + err.Error())
		}
		return
	}

	if healthy {
		if lock.IsHolding() {
			log.Printf("RAID healthy (%s), releasing inhibitor", reason)
			lock.Release()
		} else if verbose {
			log.Printf("RAID healthy: %s", reason)
		}
	} else {
		if !lock.IsHolding() {
			log.Printf("RAID unhealthy (%s), acquiring inhibitor", reason)
			lock.Acquire("RAID: " + reason)
		} else if verbose {
			log.Printf("RAID still unhealthy: %s", reason)
		}
	}
}
