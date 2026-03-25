package monitor

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

type KernelEvent struct {
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

type KernelResult struct {
	Events []KernelEvent `json:"events"`
	Alert  bool          `json:"alert"`
}

var (
	seenMessages = make(map[string]bool)
	kernelMu     sync.Mutex
)

var criticalPatterns = []string{
	"kernel panic",
	"oom-kill",
	"out of memory",
	"segfault",
	"segmentation fault",
	"general protection fault",
	"i/o error",
	"hardware error",
	"machine check",
	"mce:",
	"nmi",
	"bug:",
	"rcu_sched",
	"hung_task",
	"blocked for more than",
	"error",
	"critical",
	"fault",
	"exception",
}

func CheckKernelLogs() (*KernelResult, error) {
	messages, err := getKernelMessages()
	if err != nil {
		return nil, fmt.Errorf("kernel logs: %w", err)
	}

	kernelMu.Lock()
	defer kernelMu.Unlock()

	var events []KernelEvent
	alert := false

	for _, msg := range messages {
		msgLower := strings.ToLower(msg)

		// Skip if already seen
		hash := simpleHash(msg)
		if seenMessages[hash] {
			continue
		}

		// Check against critical patterns
		for _, pattern := range criticalPatterns {
			if strings.Contains(msgLower, pattern) {
				seenMessages[hash] = true
				events = append(events, KernelEvent{
					Message:  strings.TrimSpace(msg),
					Severity: classifySeverity(msgLower),
				})
				alert = true
				break
			}
		}
	}

	return &KernelResult{
		Events: events,
		Alert:  alert,
	}, nil
}

func getKernelMessages() ([]string, error) {
	// Try dmesg first
	cmd := exec.Command("dmesg", "--time-format", "iso")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		return splitLines(string(output)), nil
	}

	// Fallback to journalctl -k
	cmd = exec.Command("journalctl", "-k", "--no-pager", "-n", "100", "--output=short")
	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("dmesg and journalctl both failed: %w", err)
	}

	return splitLines(string(output)), nil
}

func splitLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func classifySeverity(msg string) string {
	panicPatterns := []string{"kernel panic", "panic", "fatal", "machine check", "nmi"}
	errorPatterns := []string{"error", "i/o error", "hardware error", "general protection fault", "segfault", "segmentation fault"}
	criticalPatterns := []string{"oom-kill", "out of memory", "critical", "hung_task", "blocked for more than"}

	for _, p := range panicPatterns {
		if strings.Contains(msg, p) {
			return "panic"
		}
	}
	for _, p := range criticalPatterns {
		if strings.Contains(msg, p) {
			return "critical"
		}
	}
	for _, p := range errorPatterns {
		if strings.Contains(msg, p) {
			return "error"
		}
	}
	return "warning"
}

func simpleHash(s string) string {
	// Use first 100 chars as a simple dedup key
	if len(s) > 100 {
		return s[:100]
	}
	return s
}
