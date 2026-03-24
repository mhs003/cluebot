package monitor

import (
	"fmt"
	"os/exec"
	"strings"
)

type ServiceStatus struct {
	Name        string   `json:"name"`
	Active      bool     `json:"active"`
	State       string   `json:"state"`
	SubState    string   `json:"sub_state"`
	MainPID     string   `json:"main_pid"`
	ExecStart   string   `json:"exec_start"`
	Loaded      string   `json:"loaded"`
	Fragment    string   `json:"fragment"`
	ActiveSince string   `json:"active_since"`
	ExitCode    string   `json:"exit_code"`
	ExitStatus  string   `json:"exit_status"`
	StatusMsg   string   `json:"status_msg"`
	StdErr      string   `json:"std_err"`
	RecentLogs  []string `json:"recent_logs,omitempty"`
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
		status := getServiceStatus(svc)
		result.Services = append(result.Services, status)
		if !status.Active {
			result.Alert = true
		}
	}

	return result, nil
}

func getServiceStatus(name string) ServiceStatus {
	status := ServiceStatus{
		Name:   name,
		Active: false,
	}

	// Get state using is-active
	state := getServiceProperty(name, "ActiveState")
	status.State = state
	status.Active = state == "active"

	// Get sub-state
	status.SubState = getServiceProperty(name, "SubState")

	// Get main PID
	status.MainPID = getServiceProperty(name, "MainPID")

	// Get exec start command
	status.ExecStart = getServiceProperty(name, "ExecStart")

	// Get loaded state
	status.Loaded = getServiceProperty(name, "LoadState")

	// Get fragment path
	status.Fragment = getServiceProperty(name, "FragmentPath")

	// Get active since timestamp
	status.ActiveSince = getServiceProperty(name, "ActiveEnterTimestamp")

	// Get exit code and status for failed services
	if state == "failed" || state == "inactive" {
		status.ExitCode = getServiceProperty(name, "ExecMainCode")
		status.ExitStatus = getServiceProperty(name, "ExecMainStatus")
		status.StatusMsg = getServiceProperty(name, "StatusText")
		status.StdErr = getServiceProperty(name, "StandardError")
	}

	// Get recent journal logs for failed/inactive services
	if !status.Active {
		status.RecentLogs = getServiceLogs(name)
	}

	return status
}

func getServiceProperty(service, property string) string {
	cmd := exec.Command("systemctl", "show", service, "--property="+property, "--no-pager")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(output))
	if strings.Contains(line, "=") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}
	return line
}

func getServiceLogs(service string) []string {
	cmd := exec.Command("journalctl", "-u", service, "--no-pager", "-n", "15", "--output=short")
	output, err := cmd.Output()
	if err != nil {
		return []string{fmt.Sprintf("failed to get logs: %v", err)}
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return []string{"no logs available"}
	}

	// Filter out empty lines
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}

	if len(result) == 0 {
		return []string{"no logs available"}
	}
	return result
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
