package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type CLI struct {
	PIDFile string
}

func New(pidFile string) *CLI {
	return &CLI{PIDFile: pidFile}
}

func (c *CLI) WritePID() error {
	pid := os.Getpid()
	return os.WriteFile(c.PIDFile, []byte(strconv.Itoa(pid)), 0644)
}

func (c *CLI) RemovePID() error {
	if _, err := os.Stat(c.PIDFile); err == nil {
		return os.Remove(c.PIDFile)
	}
	return nil
}

func (c *CLI) ReadPID() (int, error) {
	data, err := os.ReadFile(c.PIDFile)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID file: %w", err)
	}
	return pid, nil
}

func (c *CLI) IsRunning() bool {
	pid, err := c.ReadPID()
	if err != nil {
		return false
	}
	return processExists(pid)
}

func (c *CLI) Stop() error {
	pid, err := c.ReadPID()
	if err != nil {
		return fmt.Errorf("cluebot is not running: %w", err)
	}

	if !processExists(pid) {
		c.RemovePID()
		return fmt.Errorf("stale PID file found, cleaned up")
	}

	cmd := exec.Command("kill", "-15", strconv.Itoa(pid))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop process %d: %w", pid, err)
	}

	c.RemovePID()
	return nil
}

func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(os.Signal(nil))
	return err == nil
}
