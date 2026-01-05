// raid-sidecar prevents shutdown during RAID rebuilds or when arrays are degraded.
// This runs on the host, not in a container.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	sidecar "github.com/addisonbair/go-systemd-sidecar"
	"github.com/addisonbair/homelab-sidecars/pkg/raid"
)

func main() {
	arraysStr := requireEnv("RAID_ARRAYS")
	arrays := strings.Split(arraysStr, ",")
	for i := range arrays {
		arrays[i] = strings.TrimSpace(arrays[i])
	}

	mdstatPath := getEnv("MDSTAT_PATH", raid.DefaultMdstatPath)

	checker := &raidChecker{
		mdstatPath: mdstatPath,
		arrays:     arrays,
	}

	sidecar.MustRun(context.Background(), checker, sidecar.Options{
		InhibitWhat:  getEnv("INHIBIT_WHAT", "shutdown"),
		PollInterval: getDuration("POLL_INTERVAL", 30*time.Second),
		NotifyReady:  getEnv("NOTIFY_READY", "true") == "true",
		NotifyStatus: true,
	})
}

type raidChecker struct {
	mdstatPath string
	arrays     []string
}

func (c *raidChecker) Name() string {
	return "raid"
}

func (c *raidChecker) Check(ctx context.Context) (bool, string, error) {
	healthy, reason, err := raid.Check(c.mdstatPath, c.arrays)
	if err != nil {
		return false, "", err
	}

	if !healthy {
		// RAID is rebuilding or degraded - block shutdown
		return true, reason, nil
	}

	return false, "", nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "Error: %s is required\n", key)
		os.Exit(1)
	}
	return v
}

func getDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
