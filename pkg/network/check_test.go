package network

import (
	"testing"
)

func TestParseHexIP(t *testing.T) {
	tests := []struct {
		hex  string
		want string
		err  bool
	}{
		// 192.168.1.1 in little-endian hex = 0101A8C0
		{"0101A8C0", "192.168.1.1", false},
		// 10.0.0.1 = 0100000A
		{"0100000A", "10.0.0.1", false},
		// 172.16.0.1 = 010010AC
		{"010010AC", "172.16.0.1", false},
		// Invalid
		{"ZZZZZZZZ", "", true},
		{"short", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.hex, func(t *testing.T) {
			got, err := parseHexIP(tt.hex)
			if tt.err {
				if err == nil {
					t.Errorf("expected error for %s", tt.hex)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("parseHexIP(%s) = %s, want %s", tt.hex, got, tt.want)
			}
		})
	}
}

func TestCheckerName(t *testing.T) {
	c := NewChecker("", 0)
	if c.Name() != "network" {
		t.Errorf("Name() = %s, want network", c.Name())
	}
}
