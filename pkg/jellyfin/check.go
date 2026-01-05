package jellyfin

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Checker implements check.Checker for Jellyfin streaming sessions.
// Returns unhealthy (error) when active streams exist, healthy (nil) when idle.
// This inverts the typical health check logic because we want to BLOCK
// reboots when Jellyfin IS streaming, not when it's down.
//
// Includes a grace period after streams end to prevent interrupting
// users who briefly pause.
type Checker struct {
	Client      *Client
	GracePeriod time.Duration

	mu             sync.Mutex
	lastActiveTime time.Time
}

// NewChecker creates a Jellyfin stream checker with the given grace period.
// Grace period of 0 disables the feature.
func NewChecker(client *Client, gracePeriod time.Duration) *Checker {
	return &Checker{
		Client:      client,
		GracePeriod: gracePeriod,
	}
}

// Name returns the check name.
func (c *Checker) Name() string {
	return "jellyfin"
}

// Check returns nil if no active streams and grace period elapsed (safe to reboot),
// error if streams are active or within grace period (not safe to reboot).
func (c *Checker) Check(ctx context.Context) error {
	hasStreams, sessions, err := c.Client.HasActiveStreams(ctx)
	if err != nil {
		// If we can't reach Jellyfin, assume it's safe to reboot
		// (Jellyfin is down anyway)
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if hasStreams {
		// Update last active time whenever we see streams
		c.lastActiveTime = time.Now()
		var descriptions []string
		for _, s := range sessions {
			descriptions = append(descriptions, s.Describe())
		}
		return fmt.Errorf("%d active stream(s): %s", len(sessions), strings.Join(descriptions, "; "))
	}

	// No active streams - check grace period
	if c.GracePeriod > 0 && !c.lastActiveTime.IsZero() {
		elapsed := time.Since(c.lastActiveTime)
		if elapsed < c.GracePeriod {
			remaining := c.GracePeriod - elapsed
			return fmt.Errorf("grace period: stream ended %s ago, waiting %s", elapsed.Round(time.Second), remaining.Round(time.Second))
		}
	}

	return nil
}
