// Package ui provides user interface components for EREZMonitor.
package ui

import (
	"fmt"
	"sync"

	"github.com/getlantern/systray"

	"github.com/NaveLIL/erez-monitor/alerter"
	"github.com/NaveLIL/erez-monitor/collector"
	"github.com/NaveLIL/erez-monitor/config"
	"github.com/NaveLIL/erez-monitor/logger"
	"github.com/NaveLIL/erez-monitor/models"
)

// TrayStatus represents the tray icon status.
type TrayStatus int

const (
	TrayStatusOK TrayStatus = iota
	TrayStatusWarning
	TrayStatusCritical
)

// TrayUI manages the system tray icon and menu.
type TrayUI struct {
	config    *config.UIConfig
	alertCfg  *config.AlertsConfig
	collector *collector.Collector
	alerter   *alerter.Alerter
	log       *logger.Logger

	// Menu items
	mShowDetails   *systray.MenuItem
	mToggleOverlay *systray.MenuItem
	mSettings      *systray.MenuItem
	mExportLogs    *systray.MenuItem
	mAutostart     *systray.MenuItem
	mQuit          *systray.MenuItem

	// Callbacks
	onShowDetails   func()
	onToggleOverlay func()
	onSettings      func()
	onExportLogs    func()
	onAutostart     func() bool // returns new state
	onQuit          func()

	// State
	currentStatus TrayStatus
	mu            sync.Mutex
	running       bool
	quitting      bool

	// Icons (embedded at build time or loaded)
	iconGreen  []byte
	iconYellow []byte
	iconRed    []byte
}

// NewTrayUI creates a new TrayUI.
func NewTrayUI(cfg *config.UIConfig, alertCfg *config.AlertsConfig, coll *collector.Collector, alt *alerter.Alerter) *TrayUI {
	return &TrayUI{
		config:    cfg,
		alertCfg:  alertCfg,
		collector: coll,
		alerter:   alt,
		log:       logger.Get(),
	}
}

// SetCallbacks sets the callback functions for menu actions.
func (t *TrayUI) SetCallbacks(onShowDetails, onToggleOverlay, onSettings, onExportLogs, onQuit func(), onAutostart func() bool) {
	t.onShowDetails = onShowDetails
	t.onToggleOverlay = onToggleOverlay
	t.onSettings = onSettings
	t.onExportLogs = onExportLogs
	t.onAutostart = onAutostart
	t.onQuit = onQuit
}

// Run starts the system tray. This function blocks until the tray is closed.
func (t *TrayUI) Run() {
	systray.Run(t.onReady, t.onExit)
}

// onReady is called when the systray is ready.
func (t *TrayUI) onReady() {
	t.mu.Lock()
	t.running = true
	t.mu.Unlock()

	// Set initial icon
	t.setIcon(TrayStatusOK)
	systray.SetTitle("EREZMonitor")
	systray.SetTooltip("EREZMonitor - System Monitor\nLoading...")

	// Create menu items
	t.mShowDetails = systray.AddMenuItem("Show Details", "Open the detailed statistics window")
	t.mToggleOverlay = systray.AddMenuItem("Toggle Overlay", "Enable/disable the in-game overlay")
	systray.AddSeparator()
	t.mSettings = systray.AddMenuItem("Settings", "Open settings")
	t.mExportLogs = systray.AddMenuItem("Export Logs", "Export metrics to CSV")
	t.mAutostart = systray.AddMenuItemCheckbox("Start with Windows", "Start automatically when Windows starts", t.config.Autostart)
	systray.AddSeparator()
	t.mQuit = systray.AddMenuItem("Exit", "Exit EREZMonitor")

	// Start menu event handler
	go t.handleMenuEvents()

	// Start metrics update loop
	go t.updateLoop()

	t.log.Info("System tray initialized")
}

// onExit is called when the systray is being closed.
func (t *TrayUI) onExit() {
	t.mu.Lock()
	t.running = false
	t.mu.Unlock()
	t.log.Info("System tray closed")
}

// handleMenuEvents handles menu item clicks.
func (t *TrayUI) handleMenuEvents() {
	for {
		t.mu.Lock()
		if t.quitting {
			t.mu.Unlock()
			return
		}
		t.mu.Unlock()

		select {
		case <-t.mShowDetails.ClickedCh:
			if t.onShowDetails != nil {
				t.onShowDetails()
			}

		case <-t.mToggleOverlay.ClickedCh:
			if t.onToggleOverlay != nil {
				t.onToggleOverlay()
			}

		case <-t.mSettings.ClickedCh:
			if t.onSettings != nil {
				t.onSettings()
			}

		case <-t.mExportLogs.ClickedCh:
			if t.onExportLogs != nil {
				t.onExportLogs()
			}

		case <-t.mAutostart.ClickedCh:
			if t.onAutostart != nil {
				enabled := t.onAutostart()
				if enabled {
					t.mAutostart.Check()
				} else {
					t.mAutostart.Uncheck()
				}
			}

		case <-t.mQuit.ClickedCh:
			if t.onQuit != nil {
				t.onQuit()
			}
			return
		}
	}
}

// updateLoop periodically updates the tray tooltip and icon.
func (t *TrayUI) updateLoop() {
	// Subscribe to metrics updates
	metricsCh := make(chan *models.Metrics, 1)
	t.collector.Subscribe(metricsCh)
	defer t.collector.Unsubscribe(metricsCh)

	for metrics := range metricsCh {
		t.mu.Lock()
		if !t.running {
			t.mu.Unlock()
			return
		}
		t.mu.Unlock()

		t.updateTray(metrics)
	}
}

// updateTray updates the tray icon and tooltip based on current metrics.
func (t *TrayUI) updateTray(m *models.Metrics) {
	// Determine status based on metrics
	status := t.determineStatus(m)

	// Update icon if status changed
	if status != t.currentStatus {
		t.setIcon(status)
		t.currentStatus = status
	}

	// Update tooltip
	tooltip := t.formatTooltip(m)
	systray.SetTooltip(tooltip)
}

// determineStatus determines the tray status based on metrics.
func (t *TrayUI) determineStatus(m *models.Metrics) TrayStatus {
	// Critical: CPU > 80% or RAM > 85%
	if m.CPU.UsagePercent >= t.alertCfg.CPUThreshold ||
		m.Memory.UsedPercent >= t.alertCfg.RAMThreshold {
		return TrayStatusCritical
	}

	// Warning: CPU > 50% or RAM > 70%
	if m.CPU.UsagePercent >= 50 || m.Memory.UsedPercent >= 70 {
		return TrayStatusWarning
	}

	return TrayStatusOK
}

// formatTooltip creates the tooltip text from metrics.
func (t *TrayUI) formatTooltip(m *models.Metrics) string {
	tooltip := fmt.Sprintf(
		"EREZMonitor\n"+
			"───────────\n"+
			"CPU: %.0f%%\n"+
			"RAM: %d/%d MB (%.0f%%)",
		m.CPU.UsagePercent,
		m.Memory.UsedMB, m.Memory.TotalMB, m.Memory.UsedPercent,
	)

	if m.GPU.Available {
		tooltip += fmt.Sprintf("\nGPU: %.0f%% | %d°C", m.GPU.UsagePercent, m.GPU.TemperatureC)
	}

	// Add network info
	tooltip += fmt.Sprintf("\n───────────\n↓ %.1f KB/s | ↑ %.1f KB/s",
		m.Network.DownloadKBps, m.Network.UploadKBps)

	return tooltip
}

// setIcon sets the tray icon based on status.
func (t *TrayUI) setIcon(status TrayStatus) {
	var icon []byte
	switch status {
	case TrayStatusOK:
		icon = t.getGreenIcon()
	case TrayStatusWarning:
		icon = t.getYellowIcon()
	case TrayStatusCritical:
		icon = t.getRedIcon()
	}

	if len(icon) > 0 {
		systray.SetIcon(icon)
	}
}

// toggleAutostart toggles the autostart setting.
func (t *TrayUI) toggleAutostart() {
	t.config.Autostart = !t.config.Autostart
	if t.config.Autostart {
		t.mAutostart.Check()
	} else {
		t.mAutostart.Uncheck()
	}
	// The actual registry modification should be done by the autostart module
}

// ShowNotification shows a balloon notification.
func (t *TrayUI) ShowNotification(title, message string) {
	// Note: systray doesn't support balloon notifications directly
	// This would require Windows API calls or a different approach
	t.log.Infof("Notification: %s - %s", title, message)
}

// Quit closes the system tray.
func (t *TrayUI) Quit() {
	t.mu.Lock()
	if t.quitting {
		t.mu.Unlock()
		return
	}
	t.quitting = true
	t.running = false
	t.mu.Unlock()

	systray.Quit()
}

// IsRunning returns whether the tray is running.
func (t *TrayUI) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.running
}

// getGreenIcon returns the green icon data.
// In production, this would load from embedded assets.
func (t *TrayUI) getGreenIcon() []byte {
	if t.iconGreen == nil {
		// Generate a simple green icon programmatically
		// In production, embed actual icon files
		t.iconGreen = generateSimpleIcon(0, 255, 0)
	}
	return t.iconGreen
}

// getYellowIcon returns the yellow icon data.
func (t *TrayUI) getYellowIcon() []byte {
	if t.iconYellow == nil {
		t.iconYellow = generateSimpleIcon(255, 255, 0)
	}
	return t.iconYellow
}

// getRedIcon returns the red icon data.
func (t *TrayUI) getRedIcon() []byte {
	if t.iconRed == nil {
		t.iconRed = generateSimpleIcon(255, 0, 0)
	}
	return t.iconRed
}

// generateSimpleIcon generates a simple 16x16 ICO file with a solid color.
// This is a minimal implementation - in production, use proper icon files.
func generateSimpleIcon(r, g, b byte) []byte {
	// ICO file format:
	// ICONDIR header (6 bytes)
	// ICONDIRENTRY (16 bytes per image)
	// Image data (BMP DIB or PNG)

	// For simplicity, we'll create a minimal 16x16 32-bit BMP-style icon
	width := 16
	height := 16

	// Calculate sizes
	imageSize := width * height * 4 // 32-bit RGBA
	xorSize := imageSize
	andSize := ((width + 31) / 32) * 4 * height // 1-bit mask, padded to 32-bit boundary

	// Total data size
	dataSize := 40 + xorSize + andSize // BITMAPINFOHEADER + XOR mask + AND mask

	// Create buffer
	buf := make([]byte, 6+16+dataSize)

	// ICONDIR
	buf[0] = 0 // Reserved
	buf[1] = 0
	buf[2] = 1 // Type: 1 = ICO
	buf[3] = 0
	buf[4] = 1 // Number of images
	buf[5] = 0

	// ICONDIRENTRY
	buf[6] = byte(width)  // Width
	buf[7] = byte(height) // Height
	buf[8] = 0            // Color palette
	buf[9] = 0            // Reserved
	buf[10] = 1           // Color planes
	buf[11] = 0
	buf[12] = 32 // Bits per pixel
	buf[13] = 0
	// Size of image data (little-endian)
	buf[14] = byte(dataSize)
	buf[15] = byte(dataSize >> 8)
	buf[16] = byte(dataSize >> 16)
	buf[17] = byte(dataSize >> 24)
	// Offset to image data
	buf[18] = 22
	buf[19] = 0
	buf[20] = 0
	buf[21] = 0

	// BITMAPINFOHEADER
	offset := 22
	buf[offset] = 40 // Header size
	buf[offset+4] = byte(width)
	buf[offset+8] = byte(height * 2) // Height * 2 for XOR + AND masks
	buf[offset+12] = 1               // Planes
	buf[offset+14] = 32              // Bits per pixel
	// biSizeImage
	buf[offset+20] = byte(xorSize + andSize)
	buf[offset+21] = byte((xorSize + andSize) >> 8)

	// XOR mask (pixel data) - BGRA format, bottom-up
	offset = 22 + 40
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := offset + (y*width+x)*4
			buf[idx] = b     // Blue
			buf[idx+1] = g   // Green
			buf[idx+2] = r   // Red
			buf[idx+3] = 255 // Alpha
		}
	}

	// AND mask (all zeros = all opaque)
	// Already zeroed

	return buf
}
