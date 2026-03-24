package monitor

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type ProcessEntry struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type ProcessResult struct {
	TotalProcesses int            `json:"total_processes"`
	BaselineCount  int            `json:"baseline_count"`
	TopProcesses   []ProcessEntry `json:"top_processes"`
	Alert          bool           `json:"alert"`
}

var processBaseline int = 0
var baselineSamples int = 0

const baselineSampleSize = 5
const explosionMultiplier = 5

// QuickProcessCount returns just the total process count from /proc.
// This is very lightweight - only reads directory entries, no file I/O.
// Use this for fast fork bomb detection (1-2s interval).
func QuickProcessCount() (int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(entry.Name()); err == nil {
			count++
		}
	}
	return count, nil
}

// QuickProcessCheck runs a fast process count and checks against baseline and limit.
// Call this from the fast goroutine (1s interval).
func QuickProcessCheck(limit int) (*ProcessResult, error) {
	total, err := QuickProcessCount()
	if err != nil {
		return nil, fmt.Errorf("quick process check: %w", err)
	}

	// Build baseline from first few samples
	if baselineSamples < baselineSampleSize {
		processBaseline += total
		baselineSamples++
		if baselineSamples == baselineSampleSize {
			processBaseline = max(processBaseline/baselineSampleSize, 50)
		}
	}

	alert := false

	// Check against explosion multiplier
	if processBaseline > 0 && total > processBaseline*explosionMultiplier {
		alert = true
	}

	// Check against hard limit
	if limit > 0 && total > limit {
		alert = true
	}

	return &ProcessResult{
		TotalProcesses: total,
		BaselineCount:  processBaseline,
		Alert:          alert,
	}, nil
}

// CheckProcesses does full process enumeration with top processes list.
// Call this from the main monitor loop (5s interval) for dashboard display.
func CheckProcesses(limit int) (*ProcessResult, error) {
	counts, err := getProcessCounts()
	if err != nil {
		return nil, fmt.Errorf("process check: %w", err)
	}

	total := 0
	for _, count := range counts {
		total += count
	}

	// Build baseline from first few samples
	if baselineSamples < baselineSampleSize {
		processBaseline += total
		baselineSamples++
		if baselineSamples == baselineSampleSize {
			processBaseline = processBaseline / baselineSampleSize
			if processBaseline < 50 {
				processBaseline = 50
			}
		}
	}

	// Sort by count descending for top processes
	var topProcesses []ProcessEntry
	for name, count := range counts {
		topProcesses = append(topProcesses, ProcessEntry{Name: name, Count: count})
	}
	sortProcesses(topProcesses)

	// Keep top 10
	if len(topProcesses) > 10 {
		topProcesses = topProcesses[:10]
	}

	// Check for explosion
	alert := false
	if processBaseline > 0 && total > processBaseline*explosionMultiplier {
		alert = true
	}

	// Check against hard limit
	if limit > 0 && total > limit {
		alert = true
	}

	// Also alert on single process spawning too many instances
	for _, p := range topProcesses {
		if p.Count > 200 {
			alert = true
			break
		}
	}

	return &ProcessResult{
		TotalProcesses: total,
		BaselineCount:  processBaseline,
		TopProcesses:   topProcesses,
		Alert:          alert,
	}, nil
}

func getProcessCounts() (map[string]int, error) {
	counts := make(map[string]int)

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		cmdline, err := getProcessName(pid)
		if err != nil || cmdline == "" {
			continue
		}

		counts[cmdline]++
	}

	return counts, nil
}

func getProcessName(pid int) (string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func sortProcesses(entries []ProcessEntry) {
	for i := range entries {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].Count > entries[i].Count {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
}
