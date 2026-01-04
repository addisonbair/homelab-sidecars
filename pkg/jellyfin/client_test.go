package jellyfin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_GetActiveSessions(t *testing.T) {
	tests := []struct {
		name           string
		responseCode   int
		responseBody   string
		wantCount      int
		wantErr        bool
		wantErrContain string
	}{
		{
			name:         "no sessions",
			responseCode: 200,
			responseBody: `[]`,
			wantCount:    0,
			wantErr:      false,
		},
		{
			name:         "sessions but none playing",
			responseCode: 200,
			responseBody: `[
				{"Id": "abc", "UserName": "alice", "DeviceName": "iPhone"},
				{"Id": "def", "UserName": "bob", "DeviceName": "Android"}
			]`,
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:         "one active stream",
			responseCode: 200,
			responseBody: `[
				{"Id": "abc", "UserName": "alice", "DeviceName": "iPhone"},
				{"Id": "def", "UserName": "bob", "DeviceName": "TV", "NowPlayingItem": {"Name": "The Matrix", "Type": "Movie"}}
			]`,
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:         "multiple active streams",
			responseCode: 200,
			responseBody: `[
				{"Id": "abc", "UserName": "alice", "DeviceName": "TV", "NowPlayingItem": {"Name": "Inception", "Type": "Movie"}},
				{"Id": "def", "UserName": "bob", "DeviceName": "Tablet", "NowPlayingItem": {"Name": "Pilot", "Type": "Episode", "SeriesName": "Breaking Bad"}}
			]`,
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:         "TV show episode",
			responseCode: 200,
			responseBody: `[
				{"Id": "abc", "UserName": "kid", "DeviceName": "Living Room TV", "NowPlayingItem": {"Name": "The Flintstone Flyer", "Type": "Episode", "SeriesName": "The Flintstones"}}
			]`,
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:           "server error",
			responseCode:   500,
			responseBody:   `{"error": "internal server error"}`,
			wantCount:      0,
			wantErr:        true,
			wantErrContain: "unexpected status",
		},
		{
			name:           "unauthorized",
			responseCode:   401,
			responseBody:   `{"error": "unauthorized"}`,
			wantCount:      0,
			wantErr:        true,
			wantErrContain: "unexpected status",
		},
		{
			name:           "invalid json",
			responseCode:   200,
			responseBody:   `{not valid json`,
			wantCount:      0,
			wantErr:        true,
			wantErrContain: "decode response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/Sessions" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				if r.Header.Get("X-Emby-Token") != "test-api-key" {
					t.Errorf("missing or incorrect API key header")
				}

				w.WriteHeader(tt.responseCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-api-key", 5*time.Second)
			sessions, err := client.GetActiveSessions(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.wantErrContain != "" && !contains(err.Error(), tt.wantErrContain) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErrContain)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(sessions) != tt.wantCount {
				t.Errorf("got %d sessions, want %d", len(sessions), tt.wantCount)
			}
		})
	}
}

func TestSession_Describe(t *testing.T) {
	tests := []struct {
		name    string
		session Session
		want    string
	}{
		{
			name: "movie",
			session: Session{
				UserName:   "bob",
				DeviceName: "TV",
				NowPlayingItem: &NowPlayingItem{
					Name: "Avatar",
					Type: "Movie",
				},
			},
			want: "bob watching Avatar on TV",
		},
		{
			name: "TV episode",
			session: Session{
				UserName:   "kid",
				DeviceName: "Living Room",
				NowPlayingItem: &NowPlayingItem{
					Name:       "Episode 1",
					Type:       "Episode",
					SeriesName: "Flintstones",
				},
			},
			want: "kid watching Flintstones - Episode 1 on Living Room",
		},
		{
			name: "idle session",
			session: Session{
				UserName:   "alice",
				DeviceName: "Phone",
			},
			want: "alice on Phone (idle)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.session.Describe()
			if got != tt.want {
				t.Errorf("Describe() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClient_HasActiveStreams(t *testing.T) {
	tests := []struct {
		name         string
		responseBody string
		wantActive   bool
	}{
		{
			name:         "no streams",
			responseBody: `[]`,
			wantActive:   false,
		},
		{
			name:         "active stream",
			responseBody: `[{"Id": "1", "UserName": "bob", "DeviceName": "TV", "NowPlayingItem": {"Name": "Movie", "Type": "Movie"}}]`,
			wantActive:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-key", 5*time.Second)
			active, _, err := client.HasActiveStreams(context.Background())

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if active != tt.wantActive {
				t.Errorf("active = %v, want %v", active, tt.wantActive)
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
