package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Thresholds struct {
	CPUAlert            int `yaml:"cpu_alert"`
	MemoryAlert         int `yaml:"memory_alert"`
	DiskAlert           int `yaml:"disk_alert"`
	ProcessLimit        int `yaml:"process_limit"`
	SingleProcessCPU    int `yaml:"single_process_cpu"`
	SingleProcessMemory int `yaml:"single_process_memory"`
}

type PortMonitoring struct {
	Enabled           bool  `yaml:"enabled"`
	Ports             []int `yaml:"ports"`
	AlertOnUnexpected bool  `yaml:"alert_on_unexpected"`
}

type AutoKill struct {
	Enabled              bool `yaml:"enabled"`
	DelaySeconds         int  `yaml:"delay_seconds"`
	CPUThreshold         int  `yaml:"cpu_threshold"`
	MemoryThreshold      int  `yaml:"memory_threshold"`
	ProcessExplosionKill bool `yaml:"process_explosion_kill"`
}

type TelegramConfig struct {
	Enabled  bool     `yaml:"enabled"`
	BotToken string   `yaml:"bot_token"`
	ChatID   string   `yaml:"chat_id"`
	NotifyOn []string `yaml:"notify_on"`
}

type AlertsConfig struct {
	CooldownMinutes int            `yaml:"cooldown_minutes"`
	Deduplication   bool           `yaml:"deduplication"`
	Telegram        TelegramConfig `yaml:"telegram"`
}

type Config struct {
	MonitorInterval int            `yaml:"monitor_interval"`
	ProcessInterval int            `yaml:"process_interval"`
	HTTPPort        int            `yaml:"http_port"`
	LogDir          string         `yaml:"log_dir"`
	PIDFile         string         `yaml:"pid_file"`
	Services        []string       `yaml:"services"`
	Thresholds      Thresholds     `yaml:"thresholds"`
	PortMonitoring  PortMonitoring `yaml:"port_monitoring"`
	AutoKill        AutoKill       `yaml:"auto_kill"`
	Alerts          AlertsConfig   `yaml:"alerts"`
}

const LibDir = "/var/lib/cluebot"

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		MonitorInterval: 5,
		ProcessInterval: 1,
		HTTPPort:        8090,
		LogDir:          LibDir,
		PIDFile:         "/run/cluebot.pid",
		Services:        []string{"docker", "nginx"},
		Thresholds: Thresholds{
			CPUAlert:            90,
			MemoryAlert:         90,
			DiskAlert:           90,
			ProcessLimit:        5000,
			SingleProcessCPU:    80,
			SingleProcessMemory: 80,
		},
		PortMonitoring: PortMonitoring{
			Enabled:           false,
			Ports:             []int{22, 80, 443, 3000, 8080},
			AlertOnUnexpected: false,
		},
		AutoKill: AutoKill{
			Enabled:              false,
			DelaySeconds:         5,
			CPUThreshold:         95,
			MemoryThreshold:      95,
			ProcessExplosionKill: false,
		},
		Alerts: AlertsConfig{
			CooldownMinutes: 5,
			Deduplication:   true,
			Telegram: TelegramConfig{
				Enabled:  false,
				BotToken: "",
				ChatID:   "",
				NotifyOn: []string{"cpu", "memory", "disk", "service", "process", "kernel", "restart"},
			},
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
