package monitor

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type MemoryResult struct {
	TotalMB     float64 `json:"total_mb"`
	UsedMB      float64 `json:"used_mb"`
	AvailableMB float64 `json:"available_mb"`
	Usage       float64 `json:"usage"`
	SwapTotalMB float64 `json:"swap_total_mb"`
	SwapUsedMB  float64 `json:"swap_used_mb"`
	Alert       bool    `json:"alert"`
}

func CheckMemory(threshold int) (*MemoryResult, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return nil, fmt.Errorf("meminfo: %w", err)
	}

	info := make(map[string]uint64)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimSuffix(fields[0], ":")
		val, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		info[name] = val * 1024
	}

	total, ok := info["MemTotal"]
	if !ok {
		return nil, fmt.Errorf("MemTotal not found")
	}

	available, ok := info["MemAvailable"]
	if !ok {
		available = total - info["MemFree"]
	}

	used := total - available
	usage := float64(used) / float64(total) * 100

	swapTotal := info["SwapTotal"]
	swapFree := info["SwapFree"]
	swapUsed := swapTotal - swapFree

	result := &MemoryResult{
		TotalMB:     float64(total) / 1024 / 1024,
		UsedMB:      float64(used) / 1024 / 1024,
		AvailableMB: float64(available) / 1024 / 1024,
		Usage:       usage,
		SwapTotalMB: float64(swapTotal) / 1024 / 1024,
		SwapUsedMB:  float64(swapUsed) / 1024 / 1024,
		Alert:       usage > float64(threshold),
	}

	return result, nil
}
