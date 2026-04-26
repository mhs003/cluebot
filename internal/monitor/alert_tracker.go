package monitor

import (
	"sync"
	"time"
)

type AlertTracker struct {
	mu        sync.RWMutex
	lastAlert map[string]time.Time
	cooldown  time.Duration
	enabled   bool
}

func NewAlertTracker(cooldownMinutes int, enabled bool) *AlertTracker {
	return &AlertTracker{
		lastAlert: make(map[string]time.Time),
		cooldown:  time.Duration(cooldownMinutes) * time.Minute,
		enabled:   enabled,
	}
}

func (a *AlertTracker) ShouldAlert(incidentType string) bool {
	if !a.enabled {
		return true
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	lastTime, exists := a.lastAlert[incidentType]
	if !exists {
		return true
	}

	return time.Since(lastTime) >= a.cooldown
}

func (a *AlertTracker) RecordAlert(incidentType string) {
	if !a.enabled {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.lastAlert[incidentType] = time.Now()
}

func (a *AlertTracker) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.lastAlert = make(map[string]time.Time)
}

func (a *AlertTracker) GetCooldownStatus(incidentType string) (time.Duration, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	lastTime, exists := a.lastAlert[incidentType]
	if !exists {
		return 0, false
	}

	remaining := a.cooldown - time.Since(lastTime)
	if remaining < 0 {
		return 0, false
	}

	return remaining, true
}
