// Package inhibitor provides a wrapper around systemd-inhibit for managing
// shutdown/reboot inhibitor locks.
package inhibitor

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
)

// Lock represents a systemd inhibitor lock
type Lock struct {
	Who  string
	Why  string
	What string // shutdown, sleep, idle, etc.
	Mode string // block or delay

	mu      sync.Mutex
	cmd     *exec.Cmd
	holding bool
}

// New creates a new inhibitor lock configuration
func New(who, why string) *Lock {
	return &Lock{
		Who:  who,
		Why:  why,
		What: "shutdown",
		Mode: "block",
	}
}

// Acquire acquires the inhibitor lock if not already held
func (l *Lock) Acquire(reason string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.holding {
		return nil // Already holding
	}

	why := l.Why
	if reason != "" {
		why = reason
	}

	// systemd-inhibit --what=shutdown --who=X --why=Y --mode=block sleep infinity
	l.cmd = exec.Command("systemd-inhibit",
		"--what="+l.What,
		"--who="+l.Who,
		"--why="+why,
		"--mode="+l.Mode,
		"sleep", "infinity",
	)

	// Detach from our process group so it survives
	l.cmd.Stdout = os.Stdout
	l.cmd.Stderr = os.Stderr

	if err := l.cmd.Start(); err != nil {
		return fmt.Errorf("failed to acquire inhibitor: %w", err)
	}

	l.holding = true
	return nil
}

// Release releases the inhibitor lock if held
func (l *Lock) Release() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.holding || l.cmd == nil || l.cmd.Process == nil {
		return nil // Not holding
	}

	if err := l.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to release inhibitor: %w", err)
	}

	// Wait for process to exit to avoid zombies
	l.cmd.Wait()

	l.holding = false
	l.cmd = nil
	return nil
}

// IsHolding returns whether the lock is currently held
func (l *Lock) IsHolding() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.holding
}

// Update updates the lock reason (releases and re-acquires with new reason)
func (l *Lock) Update(reason string) error {
	l.mu.Lock()
	wasHolding := l.holding
	l.mu.Unlock()

	if wasHolding {
		if err := l.Release(); err != nil {
			return err
		}
	}

	return l.Acquire(reason)
}
