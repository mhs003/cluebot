package incidents

import (
	"cluebot/internal/logger"
	"cluebot/internal/monitor"
	"time"
)

type SystemSnapshot struct {
	Time      time.Time              `json:"time"`
	Trigger   string                 `json:"trigger"`
	CPU       *monitor.CPUResult     `json:"cpu,omitempty"`
	Memory    *monitor.MemoryResult  `json:"memory,omitempty"`
	Disk      *monitor.DiskResult    `json:"disk,omitempty"`
	Restart   *monitor.RestartResult `json:"restart,omitempty"`
	Services  *monitor.ServiceResult `json:"services,omitempty"`
	Processes *monitor.ProcessResult `json:"processes,omitempty"`
	Kernel    *monitor.KernelResult  `json:"kernel,omitempty"`
}

func Collect(trigger string, cpu *monitor.CPUResult, mem *monitor.MemoryResult, disk *monitor.DiskResult, restart *monitor.RestartResult, services *monitor.ServiceResult, processes *monitor.ProcessResult, kernel *monitor.KernelResult, log *logger.Logger) error {
	snapshot := &SystemSnapshot{
		Time:      time.Now(),
		Trigger:   trigger,
		CPU:       cpu,
		Memory:    mem,
		Disk:      disk,
		Restart:   restart,
		Services:  services,
		Processes: processes,
		Kernel:    kernel,
	}

	if err := log.LogIncident(trigger, snapshot); err != nil {
		return err
	}

	if err := log.SaveSnapshot(trigger, snapshot); err != nil {
		return err
	}

	return nil
}
