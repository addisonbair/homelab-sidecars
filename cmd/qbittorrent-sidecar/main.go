// qbittorrent-sidecar prevents shutdown while qBittorrent is downloading.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"time"

	sidecar "github.com/addisonbair/go-systemd-sidecar"
)

func main() {
	jar, _ := cookiejar.New(nil)

	checker := &qbittorrentChecker{
		url:      requireEnv("QBITTORRENT_URL"),
		username: getEnv("QBITTORRENT_USERNAME", ""),
		password: getEnv("QBITTORRENT_PASSWORD", ""),
		client:   &http.Client{Timeout: 10 * time.Second, Jar: jar},
	}

	sidecar.MustRun(context.Background(), checker, sidecar.Options{
		InhibitWhat:  getEnv("INHIBIT_WHAT", "shutdown"),
		PollInterval: getDuration("POLL_INTERVAL", 30*time.Second),
		NotifyReady:  getEnv("NOTIFY_READY", "true") == "true",
		NotifyStatus: true,
	})
}

type qbittorrentChecker struct {
	url      string
	username string
	password string
	client   *http.Client
	loggedIn bool
}

func (c *qbittorrentChecker) Name() string {
	return "qbittorrent"
}

func (c *qbittorrentChecker) login(ctx context.Context) error {
	if c.username == "" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.url+"/api/v2/auth/login",
		strings.NewReader(fmt.Sprintf("username=%s&password=%s", c.username, c.password)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	c.loggedIn = resp.StatusCode == http.StatusOK
	return nil
}

func (c *qbittorrentChecker) Check(ctx context.Context) (bool, string, error) {
	if !c.loggedIn && c.username != "" {
		if err := c.login(ctx); err != nil {
			return false, "", nil // Can't reach qBittorrent
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET",
		c.url+"/api/v2/torrents/info?filter=downloading", nil)
	if err != nil {
		return false, "", err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return false, "", nil // Can't reach qBittorrent
	}
	defer resp.Body.Close()

	// Re-login if unauthorized
	if resp.StatusCode == http.StatusForbidden {
		c.loggedIn = false
		if err := c.login(ctx); err != nil {
			return false, "", nil
		}
		return c.Check(ctx)
	}

	var torrents []struct {
		Name     string  `json:"name"`
		Progress float64 `json:"progress"`
		State    string  `json:"state"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&torrents); err != nil {
		return false, "", nil
	}

	var downloading []string
	for _, t := range torrents {
		if t.Progress < 1.0 {
			downloading = append(downloading,
				fmt.Sprintf("%s (%.0f%%)", t.Name, t.Progress*100))
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
