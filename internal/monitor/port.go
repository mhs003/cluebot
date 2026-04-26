package monitor

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type PortInfo struct {
	Port        int    `json:"port"`
	Protocol    string `json:"protocol"`
	PID         int    `json:"pid"`
	ProcessName string `json:"process_name"`
	Command     string `json:"command"`
	State       string `json:"state"`
	LocalAddr   string `json:"local_addr"`
	RemoteAddr  string `json:"remote_addr"`
}

type PortScanResult struct {
	Ports          []PortInfo `json:"ports"`
	Alert          bool       `json:"alert"`
	TriggeredPorts []int      `json:"triggered_ports"`
	Unexpected     []PortInfo `json:"unexpected_ports"`
}

func ScanPorts(monitoredPorts []int, alertOnUnexpected bool) (*PortScanResult, error) {
	result := &PortScanResult{
		Ports:          []PortInfo{},
		Alert:          false,
		TriggeredPorts: []int{},
		Unexpected:     []PortInfo{},
	}

	tcpPorts, err := scanProtocol("tcp")
	if err != nil {
		return nil, fmt.Errorf("scan tcp: %w", err)
	}
	result.Ports = append(result.Ports, tcpPorts...)

	udpPorts, err := scanProtocol("udp")
	if err != nil {
		return nil, fmt.Errorf("scan udp: %w", err)
	}
	result.Ports = append(result.Ports, udpPorts...)

	if len(monitoredPorts) > 0 || alertOnUnexpected {
		portMap := make(map[int]bool)
		for _, p := range monitoredPorts {
			portMap[p] = true
		}

		for _, port := range result.Ports {
			isMonitored := portMap[port.Port]
			isExpected := isMonitored || isSystemPort(port.Port)

			if isMonitored {
				result.TriggeredPorts = append(result.TriggeredPorts, port.Port)
				result.Alert = true
			}

			if alertOnUnexpected && !isExpected && port.State == "LISTEN" {
				result.Unexpected = append(result.Unexpected, port)
				result.Alert = true
			}
		}
	}

	return result, nil
}

func scanProtocol(protocol string) ([]PortInfo, error) {
	var ports []PortInfo

	netFile := fmt.Sprintf("/proc/net/%s", protocol)
	file, err := os.Open(netFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return ports, nil
	}

	lineNum := 0
	stateMap := getStateMap(protocol)

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}

		localAddr := fields[1]
		remoteAddr := fields[2]
		stateStr := fields[3]
		inode := fields[9]

		if inode == "0" {
			continue
		}

		localPort := hexToInt(localAddr[strings.LastIndex(localAddr, ":")+1:])
		state := stateMap[stateStr]

		if protocol == "udp" && state == "" {
			state = "UNCONN"
		}

		pid, procName, cmd := getProcessByInode(inode)

		port := PortInfo{
			Port:        localPort,
			Protocol:    protocol,
			PID:         pid,
			ProcessName: procName,
			Command:     cmd,
			State:       state,
			LocalAddr:   formatAddr(localAddr),
			RemoteAddr:  formatAddr(remoteAddr),
		}

		ports = append(ports, port)
	}

	return ports, nil
}

func getStateMap(protocol string) map[string]string {
	if protocol == "tcp" {
		return map[string]string{
			"01": "ESTABLISHED",
			"02": "SYN_SENT",
			"03": "SYN_RECV",
			"04": "FIN_WAIT1",
			"05": "FIN_WAIT2",
			"06": "TIME_WAIT",
			"07": "CLOSE",
			"08": "CLOSE_WAIT",
			"09": "LAST_ACK",
			"0A": "LISTEN",
			"0B": "CLOSING",
		}
	}
	return map[string]string{}
}

func hexToInt(hexStr string) int {
	val, err := strconv.ParseInt(hexStr, 16, 64)
	if err != nil {
		return 0
	}
	return int(val)
}

func formatAddr(hexAddr string) string {
	if len(hexAddr) < 8 {
		return ""
	}
	ip := hexAddr[:len(hexAddr)-8]
	port := hexAddr[len(ip):]

	ipHex, _ := strconv.ParseUint(ip, 16, 64)
	portDec, _ := strconv.ParseInt(port, 16, 64)

	ip1 := (ipHex >> 24) & 0xFF
	ip2 := (ipHex >> 16) & 0xFF
	ip3 := (ipHex >> 8) & 0xFF
	ip4 := ipHex & 0xFF

	return fmt.Sprintf("%d.%d.%d.%d:%d", ip1, ip2, ip3, ip4, portDec)
}

func getProcessByInode(inode string) (int, string, string) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, "", ""
	}

	_, err = strconv.ParseInt(inode, 10, 64)
	if err != nil {
		return 0, "", ""
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		fdDir := fmt.Sprintf("/proc/%d/fd", pid)
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}

		for _, fd := range fds {
			link, err := os.Readlink(fmt.Sprintf("%s/%s", fdDir, fd.Name()))
			if err != nil {
				continue
			}

			if strings.Contains(link, fmt.Sprintf("socket:[%s]", inode)) {
				comm, _ := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
				name := strings.TrimSpace(string(comm))

				cmdline, _ := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
				command := strings.ReplaceAll(string(cmdline), "\x00", " ")
				command = strings.TrimSpace(command)
				if command == "" {
					command = name
				}

				return pid, name, command
			}
		}
	}

	return 0, "", ""
}

func isSystemPort(port int) bool {
	systemPorts := []int{22, 23, 25, 53, 80, 110, 143, 443, 465, 587, 993, 995, 3306, 5432, 6379, 27017}
	for _, p := range systemPorts {
		if port == p {
			return true
		}
	}
	return false
}

func KillProcessOnPort(port int) (int, error) {
	ports, err := ScanPorts([]int{port}, false)
	if err != nil {
		return 0, err
	}

	for _, p := range ports.Ports {
		if p.Port == port && p.PID > 0 {
			if err := KillProcess(p.PID); err != nil {
				return 0, err
			}
			return p.PID, nil
		}
	}

	return 0, fmt.Errorf("no process found on port %d", port)
}
