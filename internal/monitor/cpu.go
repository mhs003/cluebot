package monitor

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type CPUResult struct {
	Usage     float64 `json:"usage"`
	LoadAvg1  float64 `json:"load_avg_1"`
	LoadAvg5  float64 `json:"load_avg_5"`
	LoadAvg15 float64 `json:"load_avg_15"`
	Alert     bool    `json:"alert"`
}

func CheckCPU(threshold int) (*CPUResult, error) {
	usage, err := getCPUUsage()
	if err != nil {
		return nil, fmt.Errorf("cpu usage: %w", err)
	}

	load1, load5, load15, err := getLoadAvg()
	if err != nil {
		return nil, fmt.Errorf("loadavg: %w", err)
	}

	result := &CPUResult{
		Usage:     usage,
		LoadAvg1:  load1,
		LoadAvg5:  load5,
		LoadAvg15: load15,
		Alert:     usage > float64(threshold),
	}

	return result, nil
}

func getCPUUsage() (float64, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return 0, fmt.Errorf("empty /proc/stat")
	}

	fields := strings.Fields(lines[0])
	if fields[0] != "cpu" {
		return 0, fmt.Errorf("unexpected /proc/stat format")
	}

	var values []int64
	for _, f := range fields[1:] {
		v, err := strconv.ParseInt(f, 10, 64)
		if err != nil {
			return 0, err
		}
		values = append(values, v)
	}

	if len(values) < 4 {
		return 0, fmt.Errorf("insufficient cpu values")
	}

	total := int64(0)
	for _, v := range values {
		total += v
	}
	idle := values[3]

	if total == 0 {
		return 0, nil
	}

	usage := float64(total-idle) / float64(total) * 100
	return usage, nil
}

func getLoadAvg() (float64, float64, float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, err
	}

	fields := strings.Fields(strings.TrimSpace(string(data)))
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("unexpected /proc/loadavg format")
	}

	l1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, 0, 0, err
	}
	l5, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, 0, 0, err
	}
	l15, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return 0, 0, 0, err
	}

	return l1, l5, l15, nil
}
