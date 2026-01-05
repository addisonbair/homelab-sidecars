// jellyfin-sidecar prevents shutdown while users are streaming from Jellyfin.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	sidecar "github.com/addisonbair/go-systemd-sidecar"
	"github.com/addisonbair/homelab-sidecars/pkg/jellyfin"
)

func main() {
	url := requireEnv("JELLYFIN_URL")
	apiKey := getEnv("JELLYFIN_API_KEY", "")
	apiKeyFile := getEnv("JELLYFIN_API_KEY_FILE", "")

	// Read API key from file if specified
	if apiKeyFile != "" && apiKey == "" {
		data, err := os.ReadFile(apiKeyFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading API key file: %v\n", err)
			os.Exit(1)
		}
		apiKey = strings.TrimSpace(string(data))
	}

	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: JELLYFIN_API_KEY or JELLYFIN_API_KEY_FILE required")
		os.Exit(1)
	}

	client := jellyfin.NewClient(url, apiKey, 10*time.Second)
	gracePeriod := getDuration("JELLYFIN_GRACE_PERIOD", 5*time.Minute)

	checker := &jellyfinChecker{
		client:      client,
		gracePeriod: gracePeriod,
	}

	sidecar.MustRun(context.Background(), checker, sidecar.Options{
		InhibitWhat:  getEnv("INHIBIT_WHAT", "shutdown:sleep"),
		PollInterval: getDuration("POLL_INTERVAL", 30*time.Second),
		NotifyReady:  getEnv("NOTIFY_READY", "true") == "true",
		NotifyStatus: true,
	})
}

type jellyfinChecker struct {
	client      *jellyfin.Client
	gracePeriod time.Duration

	mu             sync.Mutex
	lastActiveTime time.Time
}

func (c *jellyfinChecker) Name() string {
	return "jellyfin"
}

func (c *jellyfinChecker) Check(ctx context.Context) (bool, string, error) {
	hasStreams, sessions, err := c.client.HasActiveStreams(ctx)
	if err != nil {
		// If Jellyfin is unreachable, don't block shutdown
		return false, "", nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if hasStreams {
		c.lastActiveTime = time.Now()
		var descriptions []string
		for _, s := range sessions {
			descriptions = append(descriptions, s.Describe())
		}
		return true, strings.Join(descriptions, "; "), nil
	}

	// Check grace period
	if c.gracePeriod > 0 && !c.lastActiveTime.IsZero() {
		elapsed := time.Since(c.lastActiveTime)
		if elapsed < c.gracePeriod {
			remaining := c.gracePeriod - elapsed
			return true, fmt.Sprintf("grace period: %s remaining", remaining.Round(time.Second)), nil
		}
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
