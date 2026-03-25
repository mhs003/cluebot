package main

import (
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

	srv := server.New(cfg.HTTPPort, logInst)
	go func() {
		log.Printf("Starting HTTP server on port %d", cfg.HTTPPort)
		if err := srv.Start(); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	// Main monitor loop (CPU, RAM, Disk, Restart, Services, full Process check)
	ticker := time.NewTicker(time.Duration(cfg.MonitorInterval) * time.Second)
	defer ticker.Stop()

	// Fast process check loop (just count, lightweight, catches fork bombs quickly)
	processTicker := time.NewTicker(time.Duration(cfg.ProcessInterval) * time.Second)
	defer processTicker.Stop()

	fmt.Println("ClueBot monitoring started")
	fmt.Printf("Dashboard: http://localhost:%d\n", cfg.HTTPPort)

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

	srv.UpdateStats(cpu, mem, disk, restart, services, processes)

	if cpu != nil && cpu.Alert {
		incidents.Collect("cpu", cpu, mem, disk, restart, services, processes, kernel, logInst)
	}
	if mem != nil && mem.Alert {
		incidents.Collect("memory", cpu, mem, disk, restart, services, processes, kernel, logInst)
	}
	if disk != nil && disk.Alert {
		incidents.Collect("disk", cpu, mem, disk, restart, services, processes, kernel, logInst)
	}
	if restart != nil && restart.Alert {
		incidents.Collect("restart", cpu, mem, disk, restart, services, processes, kernel, logInst)
	}
	if services != nil && services.Alert {
		incidents.Collect("service", cpu, mem, disk, restart, services, processes, kernel, logInst)
	}
	if processes != nil && processes.Alert {
		incidents.Collect("process", cpu, mem, disk, restart, services, processes, kernel, logInst)
	}
	if kernel != nil && kernel.Alert {
		incidents.Collect("kernel", cpu, mem, disk, restart, services, processes, kernel, logInst)
		for _, ev := range kernel.Events {
			log.Printf("KERNEL [%s]: %s", ev.Severity, ev.Message)
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
		// Trigger full check to get top processes for the incident report
		full, _ := monitor.CheckProcesses(cfg.Thresholds.ProcessLimit)
		if full != nil {
			result = full
		}
		incidents.Collect("process", nil, nil, nil, nil, nil, result, nil, logInst)
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
