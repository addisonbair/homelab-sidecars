package jellyfin

import (
	"context"
	"fmt"
	"strings"
)

// Checker implements check.Checker for Jellyfin streaming sessions.
// Returns unhealthy (error) when active streams exist, healthy (nil) when idle.
// This inverts the typical health check logic because we want to BLOCK
// reboots when Jellyfin IS streaming, not when it's down.
type Checker struct {
	Client *Client
}

// NewChecker creates a Jellyfin stream checker.
func NewChecker(client *Client) *Checker {
	return &Checker{Client: client}
}

// Name returns the check name.
func (c *Checker) Name() string {
	return "jellyfin"
}

// Check returns nil if no active streams (safe to reboot),
// error if streams are active (not safe to reboot).
func (c *Checker) Check(ctx context.Context) error {
	hasStreams, sessions, err := c.Client.HasActiveStreams(ctx)
	if err != nil {
		// If we can't reach Jellyfin, assume it's safe to reboot
		// (Jellyfin is down anyway)
		return nil
	}

	if hasStreams {
		var descriptions []string
		for _, s := range sessions {
			descriptions = append(descriptions, s.Describe())
		}
		return fmt.Errorf("%d active stream(s): %s", len(sessions), strings.Join(descriptions, "; "))
	}

	return nil
}
