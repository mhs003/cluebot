package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Thresholds struct {
	CPUAlert    int `yaml:"cpu_alert"`
	MemoryAlert int `yaml:"memory_alert"`
	DiskAlert   int `yaml:"disk_alert"`
}

type Config struct {
	MonitorInterval int        `yaml:"monitor_interval"`
	HTTPPort        int        `yaml:"http_port"`
	LogDir          string     `yaml:"log_dir"`
	PIDFile         string     `yaml:"pid_file"`
	Services        []string   `yaml:"services"`
	Thresholds      Thresholds `yaml:"thresholds"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		MonitorInterval: 5,
		HTTPPort:        8090,
		LogDir:          "/var/lib/cluebot",
		PIDFile:         "/run/cluebot.pid",
		Services:        []string{"nginx", "docker", "postgres", "redis"},
		Thresholds: Thresholds{
			CPUAlert:    90,
			MemoryAlert: 90,
			DiskAlert:   90,
		},
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
