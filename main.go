// EREZMonitor - Lightweight System Resource Monitor for Windows
//
// A standalone Go application for monitoring CPU, RAM, GPU, disk and network
// with system tray integration, logging, and optional game overlay.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/NaveLIL/erez-monitor/alerter"
	"github.com/NaveLIL/erez-monitor/autostart"
	"github.com/NaveLIL/erez-monitor/collector"
	"github.com/NaveLIL/erez-monitor/config"
	"github.com/NaveLIL/erez-monitor/hotkeys"
	"github.com/NaveLIL/erez-monitor/logger"
	"github.com/NaveLIL/erez-monitor/models"
	"github.com/NaveLIL/erez-monitor/ui"
)

const (
	appName    = "EREZMonitor"
	appVersion = "1.0.0"
)

// Application holds all application components.
type Application struct {
	config    *config.Config
	configMgr *config.Manager
	log       *logger.Logger
	collector *collector.Collector
	alerter   *alerter.Alerter
	tray      *ui.TrayUI
	overlay   *ui.Overlay
	hotkeys   *hotkeys.Manager
	autostart *autostart.Manager

	ctx          context.Context
	cancel       context.CancelFunc
	shutdownOnce sync.Once
}

func main() {
	// Parse command line flags
	configPath := flag.String("config", "", "Path to configuration file")
	debug := flag.Bool("debug", false, "Enable debug logging")
	trayOnly := flag.Bool("tray-only", false, "Start minimized to system tray")
	version := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *version {
		fmt.Printf("%s v%s\n", appName, appVersion)
		os.Exit(0)
	}

	// Create application
	app := &Application{}

	// Initialize and run
	if err := app.init(*configPath, *debug); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	// Run the application
	app.run(*trayOnly)
}

// init initializes all application components.
func (app *Application) init(configPath string, debug bool) error {
	var err error

	// Create context for graceful shutdown
	app.ctx, app.cancel = context.WithCancel(context.Background())

	// Initialize logger first
	app.log = logger.Get()

	// Load configuration
	app.configMgr = config.GetManager()

	if configPath == "" {
		configPath, err = config.GetDefaultConfigPath()
		if err != nil {
			return fmt.Errorf("failed to get config path: %w", err)
		}
	}

	if err := app.configMgr.Load(configPath); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	app.config = app.configMgr.Get()

	// Override log level if debug flag is set
	if debug {
		app.config.Logging.Level = "debug"
	}

	// Get config directory for log files
	configDir := filepath.Dir(configPath)

	// Initialize logger with config
	if err := app.log.Init(&app.config.Logging, configDir); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	app.log.Infof("Starting %s v%s", appName, appVersion)
	app.log.Infof("Config loaded from: %s", configPath)

	// Validate configuration
	if errs := app.config.Validate(); len(errs) > 0 {
		for _, err := range errs {
			app.log.Warnf("Config validation warning: %v", err)
		}
	}

	// Initialize collector
	app.collector = collector.New(&app.config.Monitoring)

	// Initialize alerter
	app.alerter = alerter.New(&app.config.Alerts)

	// Initialize autostart manager
	app.autostart = autostart.New()

	// Initialize hotkey manager
	app.hotkeys = hotkeys.New()

	// Initialize tray UI
	app.tray = ui.NewTrayUI(
		&app.config.UI,
		&app.config.Alerts,
		app.collector,
		app.alerter,
	)

	// Initialize overlay
	app.overlay = ui.NewOverlay(&app.config.Overlay, app.collector)

	return nil
}

// run starts all components and runs the main loop.
func (app *Application) run(trayOnly bool) {
	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start collector
	if err := app.collector.Start(app.ctx); err != nil {
		app.log.Errorf("Failed to start collector: %v", err)
		return
	}

	// Log system information
	app.logSystemInfo()

	// Start alerter
	if err := app.alerter.Start(app.ctx); err != nil {
		app.log.Errorf("Failed to start alerter: %v", err)
		return
	}

	// Connect alerter to collector
	metricsCh := make(chan *models.Metrics, 10)
	app.collector.Subscribe(metricsCh)

	go func() {
		for metrics := range metricsCh {
			// Check alerts
			app.alerter.Check(metrics)

			// Log metrics to CSV
			app.log.LogMetrics(metrics)
		}
	}()

	// Set up alert handler for tray notifications
	app.alerter.AddHandler(func(alert *models.Alert) {
		app.tray.ShowNotification("EREZMonitor Alert", alert.Message)
	})

	// Set up tray callbacks
	app.tray.SetCallbacks(
		app.onShowDetails,
		app.onToggleOverlay,
		app.onMoveOverlay,
		app.onSettings,
		app.onExportLogs,
		app.onQuit,
		app.onAutostart,
	)

	// Start hotkey manager
	if err := app.hotkeys.Start(app.ctx); err != nil {
		app.log.Warnf("Failed to start hotkey manager: %v", err)
	} else {
		// Log hotkey config values
		app.log.Infof("Hotkey config - UI: %s, Overlay: %s, Move: %s",
			app.config.UI.Hotkey, app.config.Overlay.Hotkey, app.config.Overlay.MoveHotkey)

		// Register hotkeys
		app.hotkeys.RegisterDefaults(
			app.config.UI.Hotkey,
			app.config.Overlay.Hotkey,
			app.config.Overlay.MoveHotkey,
			app.onShowDetails,
			app.onToggleOverlay,
			app.onMoveOverlay,
		)
	}

	// Start overlay
	if app.config.Overlay.Enabled {
		if err := app.overlay.Start(); err != nil {
			app.log.Warnf("Failed to start overlay: %v", err)
		}
	}

	// Set callback for overlay position changes
	app.overlay.SetOnPositionChanged(func(x, y int) {
		app.config.Overlay.Position = "custom"
		app.config.Overlay.CustomX = x
		app.config.Overlay.CustomY = y
		if err := app.configMgr.Save(); err != nil {
			app.log.Errorf("Failed to save overlay position: %v", err)
		} else {
			app.log.Infof("Overlay position saved: (%d, %d)", x, y)
		}
	})

	// Start signal handler
	go func() {
		<-sigCh
		app.log.Info("Received shutdown signal")
		app.shutdown()
	}()

	app.log.Info("Application started")

	// Run the system tray (this blocks until tray is closed)
	app.tray.Run()
}

// shutdown gracefully shuts down all components.
func (app *Application) shutdown() {
	app.shutdownOnce.Do(func() {
		app.log.Info("Shutting down...")

		// Cancel context to stop all goroutines
		app.cancel()

		// Stop components in reverse order with timeouts
		done := make(chan struct{})
		go func() {
			if app.overlay != nil {
				app.overlay.Stop()
			}
			if app.hotkeys != nil {
				app.hotkeys.Stop()
			}
			if app.alerter != nil {
				app.alerter.Stop()
			}
			if app.collector != nil {
				app.collector.Stop()
			}
			close(done)
		}()

		// Wait max 2 seconds for graceful shutdown
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			app.log.Warn("Shutdown timeout, forcing exit")
		}

		// Save configuration
		if app.configMgr != nil {
			if err := app.configMgr.Save(); err != nil {
				app.log.Warnf("Failed to save config: %v", err)
			}
		}

		// Close logger
		if app.log != nil {
			app.log.Close()
		}

		// Quit tray (this will cause systray.Run to return)
		if app.tray != nil {
			app.tray.Quit()
		}

		// Force exit after a short delay
		go func() {
			time.Sleep(500 * time.Millisecond)
			os.Exit(0)
		}()
	})
}

// onShowDetails is called when "Show Details" is clicked.
func (app *Application) onShowDetails() {
	app.log.Info("Show Details clicked")
	// Get latest metrics and print summary
	if latest := app.collector.GetHistory().GetLatest(); latest != nil {
		fmt.Printf("\n=== EREZMonitor Details ===\n")
		fmt.Printf("CPU: %.1f%% (Cores: %d)\n", latest.CPU.UsagePercent, len(latest.CPU.PerCorePercent))
		fmt.Printf("RAM: %d/%d MB (%.1f%%)\n", latest.Memory.UsedMB, latest.Memory.TotalMB, latest.Memory.UsedPercent)
		if latest.GPU.Available {
			fmt.Printf("GPU: %.1f%% | %d°C | VRAM: %d/%d MB\n",
				latest.GPU.UsagePercent, latest.GPU.TemperatureC,
				latest.GPU.VRAMUsedMB, latest.GPU.VRAMTotalMB)
		}
		fmt.Printf("Network: ↓%.1f KB/s | ↑%.1f KB/s\n", latest.Network.DownloadKBps, latest.Network.UploadKBps)
		fmt.Printf("Disks:\n")
		for _, disk := range latest.Disk.Disks {
			fmt.Printf("  %s: %.1f%% used (%d/%d GB)\n",
				disk.Path, disk.UsedPercent, disk.UsedGB, disk.TotalGB)
		}
		fmt.Printf("===========================\n\n")
	}
}

// onToggleOverlay is called when "Toggle Overlay" is clicked.
func (app *Application) onToggleOverlay() {
	app.config.Overlay.Enabled = !app.config.Overlay.Enabled

	if app.config.Overlay.Enabled {
		// Start overlay if not running
		if err := app.overlay.Start(); err != nil {
			app.log.Errorf("Failed to start overlay: %v", err)
			return
		}
		app.overlay.Show()
		app.log.Info("Overlay enabled")
	} else {
		app.overlay.Hide()
		app.log.Info("Overlay disabled")
	}
}

// onMoveOverlay is called when "Move Overlay" is clicked.
func (app *Application) onMoveOverlay() {
	if !app.config.Overlay.Enabled {
		app.log.Warn("Cannot move overlay: overlay is disabled")
		app.tray.ShowNotification("EREZMonitor", "Enable overlay first to move it")
		return
	}

	isDragMode := app.overlay.ToggleDragMode()
	if isDragMode {
		app.log.Info("Overlay drag mode enabled - drag to reposition, click 'Move Overlay' again to lock")
		app.tray.ShowNotification("EREZMonitor", "Drag mode ON - drag overlay to reposition, then click 'Move Overlay' to lock")
	} else {
		app.log.Info("Overlay drag mode disabled - position locked")
		app.tray.ShowNotification("EREZMonitor", "Drag mode OFF - overlay position locked")
	}
}

// onSettings is called when "Settings" is clicked.
func (app *Application) onSettings() {
	app.log.Info("Settings clicked")

	// Open settings window in a separate goroutine
	go func() {
		settingsWnd := ui.NewSettingsWindow(app.config, app.configMgr)

		// Set callbacks for live updates
		settingsWnd.SetCallbacks(
			// onOverlayToggle
			func(enabled bool) {
				if enabled {
					if err := app.overlay.Start(); err != nil {
						app.log.Errorf("Failed to start overlay: %v", err)
						return
					}
					app.overlay.Show()
					app.log.Info("Overlay enabled via settings")
				} else {
					app.overlay.Hide()
					app.log.Info("Overlay disabled via settings")
				}
			},
			// onApply - for other settings
			func() {
				app.log.Info("Settings applied")
			},
		)

		settingsWnd.Show()
	}()
}

// onExportLogs is called when "Export Logs" is clicked.
func (app *Application) onExportLogs() {
	app.log.Debug("Export Logs clicked")

	// Get metrics history
	history := app.collector.GetHistory().GetAll()

	// Generate filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("erez-monitor-export-%s.csv", timestamp)

	// Get user's documents folder
	homeDir, err := os.UserHomeDir()
	if err != nil {
		app.log.Errorf("Failed to get home directory: %v", err)
		return
	}

	exportPath := filepath.Join(homeDir, "Documents", filename)

	// Export to CSV
	if err := app.log.ExportMetricsCSV(exportPath, history); err != nil {
		app.log.Errorf("Failed to export logs: %v", err)
		return
	}

	app.log.Infof("Metrics exported to: %s", exportPath)
	app.tray.ShowNotification("Export Complete", fmt.Sprintf("Metrics exported to %s", exportPath))
}

// onQuit is called when "Exit" is clicked.
func (app *Application) onQuit() {
	app.log.Debug("Quit clicked")
	app.shutdown()
}

// onAutostart is called when "Start with Windows" is clicked.
func (app *Application) onAutostart() bool {
	enabled, err := app.autostart.Toggle()
	if err != nil {
		app.log.Errorf("Failed to toggle autostart: %v", err)
		return false
	}

	if enabled {
		app.log.Info("Autostart enabled")
	} else {
		app.log.Info("Autostart disabled")
	}

	return enabled
}

// logSystemInfo logs detected hardware information.
func (app *Application) logSystemInfo() {
	info := app.collector.GetSystemInfo()
	if info == nil {
		return
	}

	app.log.Info("=== System Hardware Detected ===")
	if info.CPUModel != "" {
		app.log.Infof("CPU: %s (%d cores, %d threads)", info.CPUModel, info.CPUCores, info.CPUThreads)
	}
	if info.TotalRAM > 0 {
		app.log.Infof("RAM: %d MB (%.1f GB)", info.TotalRAM, float64(info.TotalRAM)/1024)
	}
	if info.GPUName != "" {
		app.log.Infof("GPU: %s", info.GPUName)
	}
	app.log.Info("================================")
}
