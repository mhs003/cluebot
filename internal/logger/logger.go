package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Logger struct {
	logDir string
	mu     sync.Mutex
}

type IncidentEntry struct {
	Time         string      `json:"time"`
	Type         string      `json:"type"`
	Trigger      string      `json:"trigger,omitempty"`
	System       interface{} `json:"system,omitempty"`
	TopProcesses []Process   `json:"top_processes,omitempty"`
}

type Process struct {
	PID  int     `json:"pid"`
	Name string  `json:"name"`
	CPU  float64 `json:"cpu"`
	Mem  float64 `json:"mem"`
}

func New(logDir string) (*Logger, error) {
	if err := os.MkdirAll(filepath.Join(logDir, "logs"), 0755); err != nil {
		return nil, fmt.Errorf("create logs dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(logDir, "incidents"), 0755); err != nil {
		return nil, fmt.Errorf("create incidents dir: %w", err)
	}
	return &Logger{logDir: logDir}, nil
}

func (l *Logger) LogIncident(incidentType string, data interface{}) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	entry := IncidentEntry{
		Time:    now.Format(time.RFC3339),
		Type:    incidentType,
		Trigger: incidentType,
		System:  data,
	}

	jsonData, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal incident: %w", err)
	}

	typeDir := filepath.Join(l.logDir, "logs", incidentType)
	if err := os.MkdirAll(typeDir, 0755); err != nil {
		return fmt.Errorf("create type dir: %w", err)
	}

	logFile := filepath.Join(typeDir, now.Format("2006-01-02")+".log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(string(jsonData) + "\n"); err != nil {
		return fmt.Errorf("write log: %w", err)
	}

	return nil
}

func (l *Logger) SaveSnapshot(incidentType string, snapshot interface{}) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	filename := now.Format("2006-01-02T15-04-05") + ".json"
	snapshotFile := filepath.Join(l.logDir, "incidents", filename)

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}

	if err := os.WriteFile(snapshotFile, data, 0644); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}

	return nil
}

func (l *Logger) GetRecentIncidents(limit int) ([]IncidentEntry, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	incidentsDir := filepath.Join(l.logDir, "incidents")
	entries, err := os.ReadDir(incidentsDir)
	if err != nil {
		return nil, err
	}

	var results []IncidentEntry
	start := 0
	if len(entries) > limit {
		start = len(entries) - limit
	}

	for i := start; i < len(entries); i++ {
		if entries[i].IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(incidentsDir, entries[i].Name()))
		if err != nil {
			continue
		}

		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}

		entry := IncidentEntry{
			Time:    asString(raw["time"]),
			Type:    asString(raw["trigger"]),
			Trigger: asString(raw["trigger"]),
			System:  raw,
		}

		results = append(results, entry)
	}

	return results, nil
}

func asString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
