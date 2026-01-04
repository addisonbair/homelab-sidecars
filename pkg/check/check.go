// Package check provides a common interface for health checks.
package check

import (
	"context"
	"fmt"
	"strings"
)

// Checker performs a health check.
type Checker interface {
	// Name returns a short identifier for this check.
	Name() string
	// Check performs the health check. Returns nil if healthy, error otherwise.
	Check(ctx context.Context) error
}

// Result of a single check execution.
type Result struct {
	Name    string
	Healthy bool
	Reason  string
	Err     error
}

// RunAll executes all checks and returns results.
// Checks are run sequentially to avoid resource contention.
func RunAll(ctx context.Context, checks []Checker) []Result {
	results := make([]Result, 0, len(checks))
	for _, c := range checks {
		select {
		case <-ctx.Done():
			results = append(results, Result{
				Name:    c.Name(),
				Healthy: false,
				Reason:  "timeout",
				Err:     ctx.Err(),
			})
			return results
		default:
		}

		err := c.Check(ctx)
		r := Result{
			Name:    c.Name(),
			Healthy: err == nil,
		}
		if err != nil {
			r.Err = err
			r.Reason = err.Error()
		}
		results = append(results, r)
	}
	return results
}

// AllHealthy returns true if all results indicate healthy status.
func AllHealthy(results []Result) bool {
	for _, r := range results {
		if !r.Healthy {
			return false
		}
	}
	return true
}

// SummarizeFailures returns a human-readable summary of failed checks.
func SummarizeFailures(results []Result) string {
	var failures []string
	for _, r := range results {
		if !r.Healthy {
			failures = append(failures, fmt.Sprintf("%s: %s", r.Name, r.Reason))
		}
	}
	if len(failures) == 0 {
		return "all checks passed"
	}
	return strings.Join(failures, "; ")
}
