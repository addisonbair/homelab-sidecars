// Package jellyfin provides a client for checking Jellyfin streaming sessions.
package jellyfin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Session represents a session from the Jellyfin API
type Session struct {
	ID             string          `json:"Id"`
	UserID         string          `json:"UserId"`
	UserName       string          `json:"UserName"`
	Client         string          `json:"Client"`
	DeviceName     string          `json:"DeviceName"`
	NowPlayingItem *NowPlayingItem `json:"NowPlayingItem,omitempty"`
	PlayState      *PlayState      `json:"PlayState,omitempty"`
}

// NowPlayingItem represents what's currently playing
type NowPlayingItem struct {
	Name       string `json:"Name"`
	Type       string `json:"Type"` // Movie, Episode, etc.
	SeriesName string `json:"SeriesName,omitempty"`
}

// PlayState represents the current play state
type PlayState struct {
	IsPaused bool `json:"IsPaused"`
}

// Describe returns a human-readable description of the session
func (s *Session) Describe() string {
	if s.NowPlayingItem == nil {
		return fmt.Sprintf("%s on %s (idle)", s.UserName, s.DeviceName)
	}

	item := s.NowPlayingItem.Name
	if s.NowPlayingItem.SeriesName != "" {
		item = fmt.Sprintf("%s - %s", s.NowPlayingItem.SeriesName, item)
	}

	return fmt.Sprintf("%s watching %s on %s", s.UserName, item, s.DeviceName)
}

// Client handles communication with Jellyfin API
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new Jellyfin API client
func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// GetActiveSessions returns all sessions that are currently playing content
func (c *Client) GetActiveSessions(ctx context.Context) ([]Session, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/Sessions", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("X-Emby-Token", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var sessions []Session
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Filter to only active sessions (those with NowPlayingItem)
	var active []Session
	for _, s := range sessions {
		if s.NowPlayingItem != nil {
			active = append(active, s)
		}
	}

	return active, nil
}

// HasActiveStreams returns true if there are any active streaming sessions
func (c *Client) HasActiveStreams(ctx context.Context) (bool, []Session, error) {
	sessions, err := c.GetActiveSessions(ctx)
	if err != nil {
		return false, nil, err
	}
	return len(sessions) > 0, sessions, nil
}
