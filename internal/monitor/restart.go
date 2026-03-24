package monitor

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type RestartResult struct {
	UptimeSeconds float64 `json:"uptime_seconds"`
	Alert         bool    `json:"alert"`
}

var lastUptime float64 = 0

func CheckRestart() (*RestartResult, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return nil, fmt.Errorf("uptime: %w", err)
	}

	fields := strings.Fields(strings.TrimSpace(string(data)))
	if len(fields) < 1 {
		return nil, fmt.Errorf("unexpected /proc/uptime format")
	}

	uptime, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return nil, err
	}

	alert := false
	if lastUptime > 0 && uptime < lastUptime {
		alert = true
	}
	lastUptime = uptime

	result := &RestartResult{
		UptimeSeconds: uptime,
		Alert:         alert,
	}

	return result, nil
}
