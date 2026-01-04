package check

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockChecker struct {
	name string
	err  error
}

func (m *mockChecker) Name() string                        { return m.name }
func (m *mockChecker) Check(ctx context.Context) error     { return m.err }

func TestRunAll(t *testing.T) {
	tests := []struct {
		name           string
		checks         []Checker
		wantAllHealthy bool
		wantResults    int
	}{
		{
			name:           "no checks",
			checks:         nil,
			wantAllHealthy: true,
			wantResults:    0,
		},
		{
			name: "all healthy",
			checks: []Checker{
				&mockChecker{name: "check1", err: nil},
				&mockChecker{name: "check2", err: nil},
			},
			wantAllHealthy: true,
			wantResults:    2,
		},
		{
			name: "one unhealthy",
			checks: []Checker{
				&mockChecker{name: "check1", err: nil},
				&mockChecker{name: "check2", err: errors.New("failed")},
			},
			wantAllHealthy: false,
			wantResults:    2,
		},
		{
			name: "all unhealthy",
			checks: []Checker{
				&mockChecker{name: "check1", err: errors.New("failed1")},
				&mockChecker{name: "check2", err: errors.New("failed2")},
			},
			wantAllHealthy: false,
			wantResults:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			results := RunAll(ctx, tt.checks)

			if len(results) != tt.wantResults {
				t.Errorf("got %d results, want %d", len(results), tt.wantResults)
			}

			if AllHealthy(results) != tt.wantAllHealthy {
				t.Errorf("AllHealthy() = %v, want %v", AllHealthy(results), tt.wantAllHealthy)
			}
		})
	}
}

func TestRunAllTimeout(t *testing.T) {
	slowChecker := &mockChecker{name: "slow", err: nil}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Let context expire
	time.Sleep(5 * time.Millisecond)

	results := RunAll(ctx, []Checker{slowChecker})

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Healthy {
		t.Error("expected unhealthy due to timeout")
	}
}

func TestSummarizeFailures(t *testing.T) {
	tests := []struct {
		name    string
		results []Result
		want    string
	}{
		{
			name:    "no failures",
			results: []Result{{Name: "check1", Healthy: true}},
			want:    "all checks passed",
		},
		{
			name: "one failure",
			results: []Result{
				{Name: "check1", Healthy: false, Reason: "disk full"},
			},
			want: "check1: disk full",
		},
		{
			name: "multiple failures",
			results: []Result{
				{Name: "raid", Healthy: false, Reason: "degraded"},
				{Name: "network", Healthy: false, Reason: "no route"},
			},
			want: "raid: degraded; network: no route",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SummarizeFailures(tt.results)
			if got != tt.want {
				t.Errorf("SummarizeFailures() = %q, want %q", got, tt.want)
			}
		})
	}
}
