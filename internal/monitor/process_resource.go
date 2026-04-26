package monitor

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type ProcessResourceEntry struct {
	PID     int     `json:"pid"`
	Name    string  `json:"name"`
	CPU     float64 `json:"cpu"`
	Memory  float64 `json:"memory"`
	Command string  `json:"command"`
}

type ProcessResourceResult struct {
	HighCPUProcesses    []ProcessResourceEntry `json:"high_cpu_processes"`
	HighMemoryProcesses []ProcessResourceEntry `json:"high_memory_processes"`
	Alert               bool                   `json:"alert"`
	Trigger             string                 `json:"trigger"`
	TopCPU              []ProcessResourceEntry `json:"top_cpu"`
	TopMemory           []ProcessResourceEntry `json:"top_memory"`
}

var (
	processCPU    = make(map[int]cpuTimes)
	processMemory = make(map[int]uint64)
)

type cpuTimes struct {
	utime     uint64
	stime     uint64
	lastCheck time.Time
}

func CheckProcessResources(cpuThreshold, memThreshold int) (*ProcessResourceResult, error) {
	result := &ProcessResourceResult{
		HighCPUProcesses:    []ProcessResourceEntry{},
		HighMemoryProcesses: []ProcessResourceEntry{},
		TopCPU:              []ProcessResourceEntry{},
		TopMemory:           []ProcessResourceEntry{},
	}

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("read /proc: %w", err)
	}

	var allProcesses []ProcessResourceEntry
	totalMem := getTotalMemory()

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		if pid == 1 {
			continue
		}

		proc, err := getProcessResource(pid, totalMem)
		if err != nil {
			continue
		}

		if proc != nil {
			allProcesses = append(allProcesses, *proc)

			if proc.CPU >= float64(cpuThreshold) {
				result.HighCPUProcesses = append(result.HighCPUProcesses, *proc)
				result.Alert = true
				result.Trigger = "cpu"
			}

			if proc.Memory >= float64(memThreshold) {
				result.HighMemoryProcesses = append(result.HighMemoryProcesses, *proc)
				result.Alert = true
				if result.Trigger == "" {
					result.Trigger = "memory"
				}
			}
		}
	}

	sortByCPU := make([]ProcessResourceEntry, len(allProcesses))
	copy(sortByCPU, allProcesses)
	for i := 0; i < len(sortByCPU)-1; i++ {
		for j := i + 1; j < len(sortByCPU); j++ {
			if sortByCPU[j].CPU > sortByCPU[i].CPU {
				sortByCPU[i], sortByCPU[j] = sortByCPU[j], sortByCPU[i]
			}
		}
	}
	if len(sortByCPU) > 10 {
		result.TopCPU = sortByCPU[:10]
	} else {
		result.TopCPU = sortByCPU
	}

	sortByMem := make([]ProcessResourceEntry, len(allProcesses))
	copy(sortByMem, allProcesses)
	for i := 0; i < len(sortByMem)-1; i++ {
		for j := i + 1; j < len(sortByMem); j++ {
			if sortByMem[j].Memory > sortByMem[i].Memory {
				sortByMem[i], sortByMem[j] = sortByMem[j], sortByMem[i]
			}
		}
	}
	if len(sortByMem) > 10 {
		result.TopMemory = sortByMem[:10]
	} else {
		result.TopMemory = sortByMem
	}

	return result, nil
}

func getProcessResource(pid int, totalMem uint64) (*ProcessResourceEntry, error) {
	comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(string(comm))

	cmdline, _ := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	command := strings.ReplaceAll(string(cmdline), "\x00", " ")
	command = strings.TrimSpace(command)
	if command == "" {
		command = name
	}

	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return nil, err
	}
	parts := strings.Fields(string(stat))
	if len(parts) < 15 {
		return nil, fmt.Errorf("invalid stat")
	}

	utime, _ := strconv.ParseUint(parts[13], 10, 64)
	stime, _ := strconv.ParseUint(parts[14], 10, 64)

	prev, exists := processCPU[pid]
	var cpuPercent float64

	if exists {
		elapsed := time.Since(prev.lastCheck).Seconds()
		if elapsed > 0 {
			deltaU := (utime - prev.utime) / uint64(time.Second)
			deltaS := (stime - prev.stime) / uint64(time.Second)
			cpuPercent = float64(deltaU+deltaS) / elapsed * 100
		}
	}

	processCPU[pid] = cpuTimes{
		utime:     utime,
		stime:     stime,
		lastCheck: time.Now(),
	}

	memBytes, err := getProcessMemory(pid)
	if err != nil {
		return nil, err
	}

	var memPercent float64
	if totalMem > 0 {
		memPercent = float64(memBytes) / float64(totalMem) * 100
	}

	return &ProcessResourceEntry{
		PID:     pid,
		Name:    name,
		CPU:     cpuPercent,
		Memory:  memPercent,
		Command: command,
	}, nil
}

func getProcessMemory(pid int) (uint64, error) {
	file, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				mem, err := strconv.ParseUint(fields[1], 10, 64)
				if err != nil {
					return 0, err
				}
				return mem * 1024, nil
			}
		}
	}
	return 0, nil
}

func getTotalMemory() uint64 {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				mem, _ := strconv.ParseUint(fields[1], 10, 64)
				return mem * 1024
			}
		}
	}
	return 0
}

func KillProcess(pid int) error {
	if pid <= 1 {
		return fmt.Errorf("cannot kill PID 1")
	}

	selfPID := os.Getpid()
	if pid == selfPID {
		return fmt.Errorf("cannot kill self")
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process: %w", err)
	}

	err = proc.Kill()
	if err != nil {
		return fmt.Errorf("kill process: %w", err)
	}

	return nil
}

func KillProcessByName(name string) ([]int, error) {
	var pids []int

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

		if pid == 1 || pid == os.Getpid() {
			continue
		}

		comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
		if err != nil {
			continue
		}

		if strings.TrimSpace(string(comm)) == name {
			if err := KillProcess(pid); err == nil {
				pids = append(pids, pid)
			}
		}
	}

	return pids, nil
}
