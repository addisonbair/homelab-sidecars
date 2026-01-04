package check

import (
	"context"
	"log"
	"time"

	"github.com/addisonbair/homelab-sidecars/pkg/inhibitor"
)

// Runner continuously executes health checks and manages an inhibitor lock.
type Runner struct {
	Checks   []Checker
	Interval time.Duration
	Timeout  time.Duration // Per-check timeout
	Lock     *inhibitor.Lock
}

// Run starts the check loop. Blocks until context is cancelled.
func (r *Runner) Run(ctx context.Context) error {
	// Run immediately on start
	r.runOnce(ctx)

	ticker := time.NewTicker(r.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Release lock on shutdown
			if r.Lock.IsHolding() {
				if err := r.Lock.Release(); err != nil {
					log.Printf("Failed to release inhibitor on shutdown: %v", err)
				}
			}
			return ctx.Err()
		case <-ticker.C:
			r.runOnce(ctx)
		}
	}
}

func (r *Runner) runOnce(ctx context.Context) {
	checkCtx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()

	results := RunAll(checkCtx, r.Checks)
	healthy := AllHealthy(results)

	// Log results
	for _, res := range results {
		if res.Healthy {
			log.Printf("[%s] healthy", res.Name)
		} else {
			log.Printf("[%s] unhealthy: %s", res.Name, res.Reason)
		}
	}

	// Manage inhibitor lock based on health
	if !healthy && !r.Lock.IsHolding() {
		reason := SummarizeFailures(results)
		log.Printf("Acquiring inhibitor: %s", reason)
		if err := r.Lock.Acquire(reason); err != nil {
			log.Printf("Failed to acquire inhibitor: %v", err)
		}
	} else if healthy && r.Lock.IsHolding() {
		log.Printf("Releasing inhibitor: all checks passed")
		if err := r.Lock.Release(); err != nil {
			log.Printf("Failed to release inhibitor: %v", err)
		}
	}
}
