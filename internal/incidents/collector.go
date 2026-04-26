package incidents

import (
	"cluebot/internal/alerts"
	"cluebot/internal/logger"
	"cluebot/internal/monitor"
	"fmt"
	"time"
)

type SystemSnapshot struct {
	Time            time.Time                      `json:"time"`
	Trigger         string                         `json:"trigger"`
	CPU             *monitor.CPUResult             `json:"cpu,omitempty"`
	Memory          *monitor.MemoryResult          `json:"memory,omitempty"`
	Disk            *monitor.DiskResult            `json:"disk,omitempty"`
	Restart         *monitor.RestartResult         `json:"restart,omitempty"`
	Services        *monitor.ServiceResult         `json:"services,omitempty"`
	Processes       *monitor.ProcessResult         `json:"processes,omitempty"`
	Kernel          *monitor.KernelResult          `json:"kernel,omitempty"`
	ProcessResource *monitor.ProcessResourceResult `json:"process_resource,omitempty"`
	Port            *monitor.PortScanResult        `json:"port,omitempty"`
}

func Collect(
	trigger string,
	cpu *monitor.CPUResult,
	mem *monitor.MemoryResult,
	disk *monitor.DiskResult,
	restart *monitor.RestartResult,
	services *monitor.ServiceResult,
	processes *monitor.ProcessResult,
	kernel *monitor.KernelResult,
	processResource *monitor.ProcessResourceResult,
	port *monitor.PortScanResult,
	log *logger.Logger,
	telegram *alerts.TelegramBot,
	alertTracker *monitor.AlertTracker,
) error {
	if alertTracker != nil && !alertTracker.ShouldAlert(trigger) {
		return nil
	}

	snapshot := &SystemSnapshot{
		Time:            time.Now(),
		Trigger:         trigger,
		CPU:             cpu,
		Memory:          mem,
		Disk:            disk,
		Restart:         restart,
		Services:        services,
		Processes:       processes,
		Kernel:          kernel,
		ProcessResource: processResource,
		Port:            port,
	}

	if err := log.LogIncident(trigger, snapshot); err != nil {
		return err
	}

	if err := log.SaveSnapshot(trigger, snapshot); err != nil {
		return err
	}

	if alertTracker != nil {
		alertTracker.RecordAlert(trigger)
	}

	if telegram != nil {
		go sendTelegramAlert(trigger, snapshot, telegram)
	}

	return nil
}

func sendTelegramAlert(trigger string, snapshot *SystemSnapshot, telegram *alerts.TelegramBot) {
	details := make(map[string]string)

	switch trigger {
	case "cpu":
		if snapshot.CPU != nil {
			details["CPU Usage"] = fmt.Sprintf("%.1f%%", snapshot.CPU.Usage)
			details["Load Average"] = fmt.Sprintf("%.2f, %.2f, %.2f",
				snapshot.CPU.LoadAvg1, snapshot.CPU.LoadAvg5, snapshot.CPU.LoadAvg15)
		}
	case "memory":
		if snapshot.Memory != nil {
			details["Memory Usage"] = fmt.Sprintf("%.1f%%", snapshot.Memory.Usage)
			if snapshot.Memory.SwapTotalMB > 0 {
				swapUsage := snapshot.Memory.SwapUsedMB / snapshot.Memory.SwapTotalMB * 100
				details["Swap Usage"] = fmt.Sprintf("%.1f%%", swapUsage)
			}
		}
	case "disk":
		if snapshot.Disk != nil && len(snapshot.Disk.Mounts) > 0 {
			m := snapshot.Disk.Mounts[0]
			details["Disk Usage"] = fmt.Sprintf("%.1f%%", m.Usage)
		}
	case "process":
		if snapshot.Processes != nil {
			details["Total Processes"] = fmt.Sprintf("%d", snapshot.Processes.TotalProcesses)
			details["Baseline"] = fmt.Sprintf("%d", snapshot.Processes.BaselineCount)
		}
		if snapshot.ProcessResource != nil {
			if len(snapshot.ProcessResource.HighCPUProcesses) > 0 {
				p := snapshot.ProcessResource.HighCPUProcesses[0]
				details["High CPU Process"] = fmt.Sprintf("%s (PID: %d, CPU: %.1f%%)", p.Name, p.PID, p.CPU)
			}
			if len(snapshot.ProcessResource.HighMemoryProcesses) > 0 {
				p := snapshot.ProcessResource.HighMemoryProcesses[0]
				details["High Memory Process"] = fmt.Sprintf("%s (PID: %d, Mem: %.1f%%)", p.Name, p.PID, p.Memory)
			}
		}
	case "service":
		if snapshot.Services != nil {
			for _, svc := range snapshot.Services.Services {
				details[fmt.Sprintf("Service: %s", svc.Name)] = fmt.Sprintf("%s (%s)", svc.State, svc.SubState)
			}
		}
	case "kernel":
		if snapshot.Kernel != nil {
			details["Kernel Events"] = fmt.Sprintf("%d events", len(snapshot.Kernel.Events))
			for _, ev := range snapshot.Kernel.Events {
				details[ev.Severity] = ev.Message
				if len(details) > 5 {
					break
				}
			}
		}
	case "restart":
		if snapshot.Restart != nil {
			details["Uptime"] = fmt.Sprintf("%.0f seconds", snapshot.Restart.UptimeSeconds)
		}
	case "port":
		if snapshot.Port != nil {
			details["Port Issues"] = fmt.Sprintf("%d ports", len(snapshot.Port.TriggeredPorts))
		}
	}

	message := alerts.FormatIncidentMessage(trigger, details)
	if err := telegram.SendAlert(trigger, message); err != nil {
		fmt.Printf("Failed to send telegram alert: %v\n", err)
	}
}
