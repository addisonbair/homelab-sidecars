// transmission-sidecar prevents shutdown while Transmission is downloading.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	sidecar "github.com/addisonbair/go-systemd-sidecar"
)

func main() {
	checker := &transmissionChecker{
		url:    requireEnv("TRANSMISSION_URL"),
		client: &http.Client{Timeout: 10 * time.Second},
	}

	sidecar.MustRun(context.Background(), checker, sidecar.Options{
		InhibitWhat:  getEnv("INHIBIT_WHAT", "shutdown"),
		PollInterval: getDuration("POLL_INTERVAL", 30*time.Second),
		NotifyReady:  getEnv("NOTIFY_READY", "true") == "true",
		NotifyStatus: true,
	})
}

type transmissionChecker struct {
	url       string
	client    *http.Client
	sessionID string
}

func (c *transmissionChecker) Name() string {
	return "transmission"
}

func (c *transmissionChecker) Check(ctx context.Context) (bool, string, error) {
	payload := `{"method":"torrent-get","arguments":{"fields":["name","percentDone","status"]}}`

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.url+"/transmission/rpc", strings.NewReader(payload))
	if err != nil {
		return false, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.sessionID != "" {
		req.Header.Set("X-Transmission-Session-Id", c.sessionID)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		// Can't reach transmission, don't block shutdown
		return false, "", nil
	}
	defer resp.Body.Close()

	// Handle session ID negotiation
	if resp.StatusCode == http.StatusConflict {
		c.sessionID = resp.Header.Get("X-Transmission-Session-Id")
		return c.Check(ctx)
	}

	var result struct {
		Arguments struct {
			Torrents []struct {
				Name        string  `json:"name"`
				PercentDone float64 `json:"percentDone"`
				Status      int     `json:"status"`
			} `json:"torrents"`
		} `json:"arguments"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, "", nil
	}

	var downloading []string
	for _, t := range result.Arguments.Torrents {
		// Status 4 = downloading
		if t.Status == 4 && t.PercentDone < 1.0 {
			downloading = append(downloading,
				fmt.Sprintf("%s (%.0f%%)", t.Name, t.PercentDone*100))
		}
	}

	if len(downloading) > 0 {
		return true, fmt.Sprintf("downloading: %s", strings.Join(downloading, ", ")), nil
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
