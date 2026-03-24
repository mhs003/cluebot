package monitor

import (
	"fmt"
	"os/exec"
	"strings"
)

type ServiceStatus struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type ServiceResult struct {
	Services []ServiceStatus `json:"services"`
	Alert    bool            `json:"alert"`
}

func CheckServices(services []string) (*ServiceResult, error) {
	result := &ServiceResult{
		Alert: false,
	}

	for _, svc := range services {
		active := isServiceActive(svc)
		result.Services = append(result.Services, ServiceStatus{
			Name:   svc,
			Active: active,
		})
		if !active {
			result.Alert = true
		}
	}

	return result, nil
}

func isServiceActive(name string) bool {
	cmd := exec.Command("systemctl", "is-active", name)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "active"
}

func CheckService(name string) error {
	cmd := exec.Command("systemctl", "status", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("service %s: %s", name, strings.TrimSpace(string(output)))
	}
	return nil
}
