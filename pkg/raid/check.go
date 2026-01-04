package raid

import (
	"context"
	"fmt"
)

// Checker implements check.Checker for RAID health.
type Checker struct {
	MdstatPath string
	Arrays     []string
}

// NewChecker creates a RAID health checker.
func NewChecker(mdstatPath string, arrays []string) *Checker {
	if mdstatPath == "" {
		mdstatPath = DefaultMdstatPath
	}
	return &Checker{
		MdstatPath: mdstatPath,
		Arrays:     arrays,
	}
}

// Name returns the check name.
func (c *Checker) Name() string {
	return "raid"
}

// Check performs the RAID health check.
// Returns nil if all expected arrays are healthy, error otherwise.
func (c *Checker) Check(ctx context.Context) error {
	// Check for context cancellation before expensive I/O
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	healthy, reason, err := Check(c.MdstatPath, c.Arrays)
	if err != nil {
		return fmt.Errorf("raid check failed: %w", err)
	}
	if !healthy {
		return fmt.Errorf("%s", reason)
	}
	return nil
}
