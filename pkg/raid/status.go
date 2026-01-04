// Package raid provides utilities for checking Linux software RAID (mdadm) status.
package raid

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Status represents the status of a RAID array
type Status struct {
	Name       string
	State      string // active, inactive, etc.
	Level      string // raid1, raid5, etc.
	Devices    int    // total devices
	Active     int    // active devices
	DeviceList string // e.g., "[UU]" or "[U_]"
	Healthy    bool
	Rebuilding bool
	Progress   string // rebuild progress if applicable
}

// DefaultMdstatPath is the default path to mdstat
const DefaultMdstatPath = "/proc/mdstat"

// Check checks if all RAID arrays are healthy
func Check(mdstatPath string, expectedArrays []string) (healthy bool, reason string, err error) {
	statuses, err := ParseMdstat(mdstatPath)
	if err != nil {
		return false, "", fmt.Errorf("failed to read mdstat: %w", err)
	}

	if len(statuses) == 0 {
		return false, "no RAID arrays found", nil
	}

	// Check each expected array
	for _, expected := range expectedArrays {
		found := false
		for _, status := range statuses {
			if status.Name == expected {
				found = true
				if !status.Healthy {
					if status.Rebuilding {
						return false, fmt.Sprintf("%s rebuilding: %s", status.Name, status.Progress), nil
					}
					return false, fmt.Sprintf("%s degraded: %s", status.Name, status.DeviceList), nil
				}
			}
		}
		if !found {
			return false, fmt.Sprintf("expected array %s not found", expected), nil
		}
	}

	// All arrays healthy
	var names []string
	for _, s := range statuses {
		names = append(names, s.Name)
	}
	return true, fmt.Sprintf("all healthy: %s", strings.Join(names, ", ")), nil
}

// ParseMdstat parses /proc/mdstat and returns status for each array
func ParseMdstat(path string) ([]Status, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return parseMdstatReader(file)
}

func parseMdstatReader(file *os.File) ([]Status, error) {
	var statuses []Status
	scanner := bufio.NewScanner(file)

	// Regex patterns
	arrayLine := regexp.MustCompile(`^(md\d+)\s*:\s*(\w+)\s+(\w+)\s+(.*)`)
	statusLine := regexp.MustCompile(`\[(\d+)/(\d+)\]\s*\[([U_]+)\]`)
	recoveryLine := regexp.MustCompile(`recovery\s*=\s*([\d.]+%)`)

	var current *Status

	for scanner.Scan() {
		line := scanner.Text()

		// Check for array definition line
		if matches := arrayLine.FindStringSubmatch(line); matches != nil {
			if current != nil {
				statuses = append(statuses, *current)
			}
			current = &Status{
				Name:  matches[1],
				State: matches[2],
				Level: matches[3],
			}
			continue
		}

		if current == nil {
			continue
		}

		// Check for status line with [UU] pattern
		if matches := statusLine.FindStringSubmatch(line); matches != nil {
			current.Devices = mustAtoi(matches[1])
			current.Active = mustAtoi(matches[2])
			current.DeviceList = "[" + matches[3] + "]"
			current.Healthy = !strings.Contains(matches[3], "_")
		}

		// Check for recovery progress
		if matches := recoveryLine.FindStringSubmatch(line); matches != nil {
			current.Rebuilding = true
			current.Progress = matches[1]
			current.Healthy = false
		}
	}

	if current != nil {
		statuses = append(statuses, *current)
	}

	return statuses, scanner.Err()
}

func mustAtoi(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}
