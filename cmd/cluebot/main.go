package main

import (
	"cluebot/internal/alerts"
	"cluebot/internal/cli"
	"cluebot/internal/config"
	"cluebot/internal/incidents"
	"cluebot/internal/logger"
	"cluebot/internal/monitor"
	"cluebot/internal/server"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	alertTracker *monitor.AlertTracker
	telegramBot  *alerts.TelegramBot
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cfg, err := config.Load(config.LibDir + "/configs/default.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	c := cli.New(cfg.PIDFile)

	switch os.Args[1] {
	case "start":
		start(cfg, c)
	case "stop":
		stop(cfg, c)
	case "status":
		status(cfg, c)
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: cluebot <command>")
	fmt.Println("Commands:")
	fmt.Println("  start    Start the monitoring daemon")
	fmt.Println("  stop     Stop the monitoring daemon")
	fmt.Println("  status   Show daemon status")
}

func start(cfg *config.Config, c *cli.CLI) {
	if c.IsRunning() {
		fmt.Println("cluebot is already running")
		os.Exit(1)
	}

	if err := c.WritePID(); err != nil {
		log.Fatalf("Failed to write PID: %v", err)
	}

	logInst, err := logger.New(cfg.LogDir)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}

	alertTracker = monitor.NewAlertTracker(cfg.Alerts.CooldownMinutes, cfg.Alerts.Deduplication)

	telegramBot = alerts.NewTelegramBot(
		cfg.Alerts.Telegram.BotToken,
		cfg.Alerts.Telegram.ChatID,
		cfg.Alerts.Telegram.Enabled,
		cfg.Alerts.Telegram.NotifyOn,
	)

	srv := server.New(cfg.HTTPPort, logInst)
	go func() {
		log.Printf("Starting HTTP server on port %d", cfg.HTTPPort)
		if err := srv.Start(); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(time.Duration(cfg.MonitorInterval) * time.Second)
	defer ticker.Stop()

	processTicker := time.NewTicker(time.Duration(cfg.ProcessInterval) * time.Second)
	defer processTicker.Stop()

	fmt.Println("ClueBot monitoring started")
	fmt.Printf("Dashboard: http://localhost:%d\n", cfg.HTTPPort)

	if cfg.Alerts.Telegram.Enabled {
		fmt.Println("Telegram alerts enabled")
	}

	for {
		select {
		case <-ticker.C:
			runMonitorLoop(cfg, logInst, srv)
		case <-processTicker.C:
			runFastProcessCheck(cfg, logInst, srv)
		case <-sig:
			fmt.Println("\nShutting down ClueBot...")
			c.RemovePID()
			os.Exit(0)
		}
	}
}

func runMonitorLoop(cfg *config.Config, logInst *logger.Logger, srv *server.Server) {
	cpu, err := monitor.CheckCPU(cfg.Thresholds.CPUAlert)
	if err != nil {
		log.Printf("CPU check error: %v", err)
	}

	mem, err := monitor.CheckMemory(cfg.Thresholds.MemoryAlert)
	if err != nil {
		log.Printf("Memory check error: %v", err)
	}

	disk, err := monitor.CheckDisk(cfg.Thresholds.DiskAlert, []string{"/"})
	if err != nil {
		log.Printf("Disk check error: %v", err)
	}

	restart, err := monitor.CheckRestart()
	if err != nil {
		log.Printf("Restart check error: %v", err)
	}

	services, err := monitor.CheckServices(cfg.Services)
	if err != nil {
		log.Printf("Services check error: %v", err)
	}

	processes, err := monitor.CheckProcesses(cfg.Thresholds.ProcessLimit)
	if err != nil {
		log.Printf("Process check error: %v", err)
	}

	kernel, err := monitor.CheckKernelLogs()
	if err != nil {
		log.Printf("Kernel log check error: %v", err)
	}

	processResource, err := monitor.CheckProcessResources(cfg.Thresholds.SingleProcessCPU, cfg.Thresholds.SingleProcessMemory)
	if err != nil {
		log.Printf("Process resource check error: %v", err)
	}

	var portResult *monitor.PortScanResult
	if cfg.PortMonitoring.Enabled {
		portResult, err = monitor.ScanPorts(cfg.PortMonitoring.Ports, cfg.PortMonitoring.AlertOnUnexpected)
		if err != nil {
			log.Printf("Port scan error: %v", err)
		}
	}

	srv.UpdateStats(cpu, mem, disk, restart, services, processes, processResource, portResult)

	handleAlerts(cfg, cpu, mem, disk, restart, services, processes, kernel, processResource, portResult, logInst)

	handleAutoKill(cfg, processResource, processes)
}

func handleAlerts(
	cfg *config.Config,
	cpu *monitor.CPUResult,
	mem *monitor.MemoryResult,
	disk *monitor.DiskResult,
	restart *monitor.RestartResult,
	services *monitor.ServiceResult,
	processes *monitor.ProcessResult,
	kernel *monitor.KernelResult,
	processResource *monitor.ProcessResourceResult,
	portResult *monitor.PortScanResult,
	logInst *logger.Logger,
) {
	if cpu != nil && cpu.Alert {
		incidents.Collect("cpu", cpu, mem, disk, restart, services, processes, kernel, processResource, portResult, logInst, telegramBot, alertTracker)
	}
	if mem != nil && mem.Alert {
		incidents.Collect("memory", cpu, mem, disk, restart, services, processes, kernel, processResource, portResult, logInst, telegramBot, alertTracker)
	}
	if disk != nil && disk.Alert {
		incidents.Collect("disk", cpu, mem, disk, restart, services, processes, kernel, processResource, portResult, logInst, telegramBot, alertTracker)
	}
	if restart != nil && restart.Alert {
		incidents.Collect("restart", cpu, mem, disk, restart, services, processes, kernel, processResource, portResult, logInst, telegramBot, alertTracker)
	}
	if services != nil && services.Alert {
		incidents.Collect("service", cpu, mem, disk, restart, services, processes, kernel, processResource, portResult, logInst, telegramBot, alertTracker)
	}
	if processes != nil && processes.Alert {
		incidents.Collect("process", cpu, mem, disk, restart, services, processes, kernel, processResource, portResult, logInst, telegramBot, alertTracker)
	}
	if processResource != nil && processResource.Alert {
		incidents.Collect("process_resource", cpu, mem, disk, restart, services, processes, kernel, processResource, portResult, logInst, telegramBot, alertTracker)
	}
	if kernel != nil && kernel.Alert {
		incidents.Collect("kernel", cpu, mem, disk, restart, services, processes, kernel, processResource, portResult, logInst, telegramBot, alertTracker)
		for _, ev := range kernel.Events {
			log.Printf("KERNEL [%s]: %s", ev.Severity, ev.Message)
		}
	}
	if portResult != nil && portResult.Alert {
		incidents.Collect("port", cpu, mem, disk, restart, services, processes, kernel, processResource, portResult, logInst, telegramBot, alertTracker)
	}
}

func handleAutoKill(cfg *config.Config, processResource *monitor.ProcessResourceResult, processes *monitor.ProcessResult) {
	if !cfg.AutoKill.Enabled {
		return
	}

	if processes != nil && processes.Alert && cfg.AutoKill.ProcessExplosionKill {
		log.Printf("WARNING: Process explosion detected, auto-kill enabled")
		for _, p := range processes.TopProcesses {
			if p.Count > 200 {
				pids, err := monitor.KillProcessByName(p.Name)
				if err == nil && len(pids) > 0 {
					log.Printf("Auto-killed %d processes with name: %s", len(pids), p.Name)
				}
				break
			}
		}
	}

	if processResource != nil && processResource.Alert {
		for _, p := range processResource.HighCPUProcesses {
			if p.CPU >= float64(cfg.AutoKill.CPUThreshold) {
				log.Printf("WARNING: High CPU process %s (PID: %d, CPU: %.1f%%), auto-kill enabled",
					p.Name, p.PID, p.CPU)
				if err := monitor.KillProcess(p.PID); err == nil {
					log.Printf("Auto-killed process %s (PID: %d)", p.Name, p.PID)
				}
			}
		}
		for _, p := range processResource.HighMemoryProcesses {
			if p.Memory >= float64(cfg.AutoKill.MemoryThreshold) {
				log.Printf("WARNING: High memory process %s (PID: %d, Memory: %.1f%%), auto-kill enabled",
					p.Name, p.PID, p.Memory)
				if err := monitor.KillProcess(p.PID); err == nil {
					log.Printf("Auto-killed process %s (PID: %d)", p.Name, p.PID)
				}
			}
		}
	}
}

func runFastProcessCheck(cfg *config.Config, logInst *logger.Logger, _ *server.Server) {
	result, err := monitor.QuickProcessCheck(cfg.Thresholds.ProcessLimit)
	if err != nil {
		log.Printf("Fast process check error: %v", err)
		return
	}

	if result.Alert {
		full, _ := monitor.CheckProcesses(cfg.Thresholds.ProcessLimit)
		if full != nil {
			result = full
		}
		incidents.Collect("process", nil, nil, nil, nil, nil, result, nil, nil, nil, logInst, telegramBot, alertTracker)
		log.Printf("WARNING: Process explosion detected! Total: %d, Baseline: %d", result.TotalProcesses, result.BaselineCount)
	}
}

func stop(_ *config.Config, c *cli.CLI) {
	if err := c.Stop(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("ClueBot stopped")
}

func status(cfg *config.Config, c *cli.CLI) {
	if !c.IsRunning() {
		fmt.Println("ClueBot is not running")
		return
	}

	pid, _ := c.ReadPID()
	fmt.Printf("ClueBot is running (PID: %d)\n", pid)
	fmt.Printf("Dashboard: http://localhost:%d\n", cfg.HTTPPort)
}
