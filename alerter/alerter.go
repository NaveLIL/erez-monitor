// Package alerter provides system alert functionality.
package alerter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/NaveLIL/erez-monitor/config"
	"github.com/NaveLIL/erez-monitor/logger"
	"github.com/NaveLIL/erez-monitor/models"
)

// AlertHandler is a function that handles an alert.
type AlertHandler func(alert *models.Alert)

// Alerter monitors metrics and triggers alerts when thresholds are exceeded.
type Alerter struct {
	config     *config.AlertsConfig
	log        *logger.Logger
	handlers   []AlertHandler
	handlersMu sync.RWMutex

	// Cooldown tracking
	lastAlerts map[models.AlertType]time.Time
	alertsMu   sync.Mutex

	// Alert history
	history   []*models.Alert
	historyMu sync.RWMutex

	// State
	running bool
	mu      sync.RWMutex
}

// New creates a new Alerter with the given configuration.
func New(cfg *config.AlertsConfig) *Alerter {
	return &Alerter{
		config:     cfg,
		log:        logger.Get(),
		lastAlerts: make(map[models.AlertType]time.Time),
		history:    make([]*models.Alert, 0, 100),
	}
}

// Start starts the alerter.
func (a *Alerter) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return nil
	}
	a.running = true
	a.mu.Unlock()

	a.log.Info("Alerter started")
	return nil
}

// Stop stops the alerter.
func (a *Alerter) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return
	}

	a.running = false
	a.log.Info("Alerter stopped")
}

// AddHandler adds an alert handler.
func (a *Alerter) AddHandler(handler AlertHandler) {
	a.handlersMu.Lock()
	defer a.handlersMu.Unlock()
	a.handlers = append(a.handlers, handler)
}

// Check checks the metrics against thresholds and triggers alerts.
func (a *Alerter) Check(metrics *models.Metrics) {
	a.mu.RLock()
	if !a.running || !a.config.Enabled {
		a.mu.RUnlock()
		return
	}
	a.mu.RUnlock()

	// Check CPU threshold
	if metrics.CPU.UsagePercent >= a.config.CPUThreshold {
		a.triggerAlert(models.AlertTypeCPU,
			fmt.Sprintf("CPU usage is %.1f%% (threshold: %.1f%%)",
				metrics.CPU.UsagePercent, a.config.CPUThreshold),
			metrics.CPU.UsagePercent,
			a.config.CPUThreshold)
	}

	// Check RAM threshold
	if metrics.Memory.UsedPercent >= a.config.RAMThreshold {
		a.triggerAlert(models.AlertTypeRAM,
			fmt.Sprintf("RAM usage is %.1f%% (threshold: %.1f%%)",
				metrics.Memory.UsedPercent, a.config.RAMThreshold),
			metrics.Memory.UsedPercent,
			a.config.RAMThreshold)
	}

	// Check GPU threshold (if available)
	if metrics.GPU.Available {
		if metrics.GPU.UsagePercent >= a.config.GPUThreshold {
			a.triggerAlert(models.AlertTypeGPU,
				fmt.Sprintf("GPU usage is %.1f%% (threshold: %.1f%%)",
					metrics.GPU.UsagePercent, a.config.GPUThreshold),
				metrics.GPU.UsagePercent,
				a.config.GPUThreshold)
		}

		// Check GPU temperature
		if float64(metrics.GPU.TemperatureC) >= a.config.GPUTempThreshold {
			a.triggerAlert(models.AlertTypeGPU,
				fmt.Sprintf("GPU temperature is %d°C (threshold: %.0f°C)",
					metrics.GPU.TemperatureC, a.config.GPUTempThreshold),
				float64(metrics.GPU.TemperatureC),
				a.config.GPUTempThreshold)
		}
	}

	// Check disk thresholds
	for _, disk := range metrics.Disk.Disks {
		if disk.UsedPercent >= a.config.DiskThreshold {
			a.triggerAlert(models.AlertTypeDisk,
				fmt.Sprintf("Disk %s usage is %.1f%% (threshold: %.1f%%)",
					disk.Path, disk.UsedPercent, a.config.DiskThreshold),
				disk.UsedPercent,
				a.config.DiskThreshold)
		}
	}
}

// triggerAlert creates and dispatches an alert if cooldown has passed.
func (a *Alerter) triggerAlert(alertType models.AlertType, message string, value, threshold float64) {
	a.alertsMu.Lock()

	// Check cooldown
	if lastTime, ok := a.lastAlerts[alertType]; ok {
		if time.Since(lastTime) < a.config.Cooldown {
			a.alertsMu.Unlock()
			return
		}
	}

	// Update last alert time
	a.lastAlerts[alertType] = time.Now()
	a.alertsMu.Unlock()

	// Create alert
	alert := &models.Alert{
		Type:      alertType,
		Timestamp: time.Now(),
		Message:   message,
		Value:     value,
		Threshold: threshold,
	}

	// Add to history
	a.historyMu.Lock()
	a.history = append(a.history, alert)
	// Keep only last 100 alerts
	if len(a.history) > 100 {
		a.history = a.history[len(a.history)-100:]
	}
	a.historyMu.Unlock()

	// Log the alert
	a.log.Alert(string(alertType), message)

	// Notify handlers
	a.handlersMu.RLock()
	handlers := make([]AlertHandler, len(a.handlers))
	copy(handlers, a.handlers)
	a.handlersMu.RUnlock()

	for _, handler := range handlers {
		go handler(alert)
	}

	// Play sound if enabled
	if a.config.SoundEnabled {
		a.playAlertSound()
	}
}

// playAlertSound plays the system alert sound.
func (a *Alerter) playAlertSound() {
	// Windows API call to play system sound
	// Using MessageBeep or PlaySound via syscall
	// For simplicity, using the console beep
	// In production, use golang.org/x/sys/windows to call MessageBeep

	/*
		import "golang.org/x/sys/windows"

		// MB_ICONEXCLAMATION = 0x00000030
		windows.MessageBeep(0x30)
	*/
}

// GetHistory returns the alert history.
func (a *Alerter) GetHistory() []*models.Alert {
	a.historyMu.RLock()
	defer a.historyMu.RUnlock()

	result := make([]*models.Alert, len(a.history))
	copy(result, a.history)
	return result
}

// GetRecentAlerts returns alerts from the last n minutes.
func (a *Alerter) GetRecentAlerts(minutes int) []*models.Alert {
	a.historyMu.RLock()
	defer a.historyMu.RUnlock()

	cutoff := time.Now().Add(-time.Duration(minutes) * time.Minute)
	var result []*models.Alert

	for _, alert := range a.history {
		if alert.Timestamp.After(cutoff) {
			result = append(result, alert)
		}
	}

	return result
}

// GetAlertsByType returns alerts of a specific type.
func (a *Alerter) GetAlertsByType(alertType models.AlertType) []*models.Alert {
	a.historyMu.RLock()
	defer a.historyMu.RUnlock()

	var result []*models.Alert
	for _, alert := range a.history {
		if alert.Type == alertType {
			result = append(result, alert)
		}
	}

	return result
}

// ClearHistory clears the alert history.
func (a *Alerter) ClearHistory() {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	a.history = a.history[:0]
}

// ResetCooldowns resets all cooldown timers.
func (a *Alerter) ResetCooldowns() {
	a.alertsMu.Lock()
	defer a.alertsMu.Unlock()
	a.lastAlerts = make(map[models.AlertType]time.Time)
}

// UpdateConfig updates the alerter configuration.
func (a *Alerter) UpdateConfig(cfg *config.AlertsConfig) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config = cfg
}

// IsEnabled returns whether alerts are enabled.
func (a *Alerter) IsEnabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.config.Enabled
}

// SetEnabled enables or disables alerts.
func (a *Alerter) SetEnabled(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config.Enabled = enabled
}

// GetAlertCount returns the number of alerts in history.
func (a *Alerter) GetAlertCount() int {
	a.historyMu.RLock()
	defer a.historyMu.RUnlock()
	return len(a.history)
}

// GetLastAlertTime returns the time of the last alert for a given type.
func (a *Alerter) GetLastAlertTime(alertType models.AlertType) (time.Time, bool) {
	a.alertsMu.Lock()
	defer a.alertsMu.Unlock()

	t, ok := a.lastAlerts[alertType]
	return t, ok
}
