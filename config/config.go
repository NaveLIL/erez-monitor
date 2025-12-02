// Package config provides configuration management for EREZMonitor.
package config

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/spf13/viper"
)

//go:embed config.yaml
var defaultConfig embed.FS

// Config holds all application configuration.
type Config struct {
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
	Alerts     AlertsConfig     `mapstructure:"alerts"`
	UI         UIConfig         `mapstructure:"ui"`
	Overlay    OverlayConfig    `mapstructure:"overlay"`
	Logging    LoggingConfig    `mapstructure:"logging"`
}

// MonitoringConfig holds monitoring-related settings.
type MonitoringConfig struct {
	// UpdateInterval is how often metrics are collected.
	UpdateInterval time.Duration `mapstructure:"update_interval"`
	// HistoryDuration is how long to keep metrics history.
	HistoryDuration time.Duration `mapstructure:"history_duration"`
	// EnableGPU enables GPU monitoring (requires NVIDIA GPU with NVML).
	EnableGPU bool `mapstructure:"enable_gpu"`
	// EnableProcesses enables top processes monitoring.
	EnableProcesses bool `mapstructure:"enable_processes"`
	// TopProcessCount is how many top processes to track.
	TopProcessCount int `mapstructure:"top_process_count"`
}

// AlertsConfig holds alert threshold settings.
type AlertsConfig struct {
	// Enabled enables or disables alerts.
	Enabled bool `mapstructure:"enabled"`
	// CPUThreshold is the CPU usage percentage threshold for alerts.
	CPUThreshold float64 `mapstructure:"cpu_threshold"`
	// RAMThreshold is the RAM usage percentage threshold for alerts.
	RAMThreshold float64 `mapstructure:"ram_threshold"`
	// GPUThreshold is the GPU usage percentage threshold for alerts.
	GPUThreshold float64 `mapstructure:"gpu_threshold"`
	// GPUTempThreshold is the GPU temperature threshold in Celsius.
	GPUTempThreshold float64 `mapstructure:"gpu_temp_threshold"`
	// DiskThreshold is the disk usage percentage threshold for alerts.
	DiskThreshold float64 `mapstructure:"disk_threshold"`
	// Cooldown is the minimum time between repeated alerts of the same type.
	Cooldown time.Duration `mapstructure:"cooldown"`
	// SoundEnabled enables sound notifications.
	SoundEnabled bool `mapstructure:"sound_enabled"`
}

// UIConfig holds UI-related settings.
type UIConfig struct {
	// TrayEnabled enables the system tray icon.
	TrayEnabled bool `mapstructure:"tray_enabled"`
	// Autostart enables automatic startup with Windows.
	Autostart bool `mapstructure:"autostart"`
	// Hotkey is the global hotkey to show/hide the main window.
	Hotkey string `mapstructure:"hotkey"`
	// Theme is the UI theme ("dark" or "light").
	Theme string `mapstructure:"theme"`
	// Language is the UI language.
	Language string `mapstructure:"language"`
	// WindowWidth is the default window width.
	WindowWidth int `mapstructure:"window_width"`
	// WindowHeight is the default window height.
	WindowHeight int `mapstructure:"window_height"`
	// RefreshRate is how often the UI updates in milliseconds.
	RefreshRate time.Duration `mapstructure:"refresh_rate"`
}

// OverlayConfig holds overlay-related settings.
type OverlayConfig struct {
	// Enabled enables the overlay.
	Enabled bool `mapstructure:"enabled"`
	// Position is the overlay position ("top-right", "top-left", "bottom-right", "bottom-left", "custom").
	Position string `mapstructure:"position"`
	// CustomX is the custom X position (used when Position is "custom").
	CustomX int `mapstructure:"custom_x"`
	// CustomY is the custom Y position (used when Position is "custom").
	CustomY int `mapstructure:"custom_y"`
	// Opacity is the overlay opacity (0.0 to 1.0).
	Opacity float64 `mapstructure:"opacity"`
	// FontSize is the overlay font size in pixels.
	FontSize int `mapstructure:"font_size"`
	// ShowFPS enables FPS display in the overlay.
	ShowFPS bool `mapstructure:"show_fps"`
	// ShowCPU enables CPU display in the overlay.
	ShowCPU bool `mapstructure:"show_cpu"`
	// ShowRAM enables RAM display in the overlay.
	ShowRAM bool `mapstructure:"show_ram"`
	// ShowGPU enables GPU display in the overlay.
	ShowGPU bool `mapstructure:"show_gpu"`
	// ShowNet enables Network display in the overlay.
	ShowNet bool `mapstructure:"show_net"`
	// ShowDisk enables Disk display in the overlay.
	ShowDisk bool `mapstructure:"show_disk"`
	// BackgroundColor is the overlay background color.
	BackgroundColor string `mapstructure:"background_color"`
	// TextColor is the overlay text color.
	TextColor string `mapstructure:"text_color"`
	// Hotkey is the hotkey to toggle the overlay.
	Hotkey string `mapstructure:"hotkey"`
	// MoveHotkey is the hotkey to toggle overlay drag mode.
	MoveHotkey string `mapstructure:"move_hotkey"`
}

// LoggingConfig holds logging-related settings.
type LoggingConfig struct {
	// Level is the minimum log level ("debug", "info", "warn", "error").
	Level string `mapstructure:"level"`
	// ToFile enables logging to a file.
	ToFile bool `mapstructure:"to_file"`
	// FilePath is the path to the log file (relative to config dir if not absolute).
	FilePath string `mapstructure:"file_path"`
	// CSVExport enables CSV export of metrics.
	CSVExport bool `mapstructure:"csv_export"`
	// CSVPath is the path to the CSV file.
	CSVPath string `mapstructure:"csv_path"`
	// MaxFileSize is the maximum log file size before rotation.
	MaxFileSize string `mapstructure:"max_file_size"`
	// Rotation is the log rotation strategy ("daily", "size", "both").
	Rotation string `mapstructure:"rotation"`
	// MaxAge is the maximum age of log files in days.
	MaxAge int `mapstructure:"max_age"`
	// MaxBackups is the maximum number of old log files to retain.
	MaxBackups int `mapstructure:"max_backups"`
}

// Manager handles configuration loading and saving.
type Manager struct {
	mu       sync.RWMutex
	config   *Config
	viper    *viper.Viper
	filePath string
}

var (
	instance *Manager
	once     sync.Once
)

// GetManager returns the singleton configuration manager instance.
func GetManager() *Manager {
	once.Do(func() {
		instance = &Manager{
			viper: viper.New(),
		}
	})
	return instance
}

// Load loads the configuration from the specified file path.
// If the file doesn't exist, it creates a default configuration.
func (m *Manager) Load(configPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.filePath = configPath

	// Set up viper
	m.viper.SetConfigType("yaml")

	// Set defaults
	m.setDefaults()

	// Try to read the config file
	if configPath != "" {
		m.viper.SetConfigFile(configPath)
		if err := m.viper.ReadInConfig(); err != nil {
			if os.IsNotExist(err) {
				// Create default config file
				if err := m.createDefaultConfig(configPath); err != nil {
					return fmt.Errorf("failed to create default config: %w", err)
				}
			} else {
				return fmt.Errorf("failed to read config: %w", err)
			}
		}
	} else {
		// Use embedded default config
		data, err := defaultConfig.ReadFile("config.yaml")
		if err != nil {
			return fmt.Errorf("failed to read embedded config: %w", err)
		}
		if err := m.viper.ReadConfig(newByteReader(data)); err != nil {
			return fmt.Errorf("failed to parse embedded config: %w", err)
		}
	}

	// Unmarshal into config struct
	m.config = &Config{}
	if err := m.viper.Unmarshal(m.config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}

// Save saves the current configuration to the file.
func (m *Manager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.filePath == "" {
		return fmt.Errorf("no config file path set")
	}

	return m.viper.WriteConfig()
}

// SaveAs saves the configuration to a new file.
func (m *Manager) SaveAs(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.filePath = path
	m.viper.SetConfigFile(path)
	return m.viper.WriteConfig()
}

// Get returns the current configuration.
func (m *Manager) Get() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// Update updates the configuration with a modifier function.
func (m *Manager) Update(modifier func(*Config)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	modifier(m.config)

	// Update viper values
	m.viper.Set("monitoring", m.config.Monitoring)
	m.viper.Set("alerts", m.config.Alerts)
	m.viper.Set("ui", m.config.UI)
	m.viper.Set("overlay", m.config.Overlay)
	m.viper.Set("logging", m.config.Logging)

	return nil
}

// GetConfigDir returns the configuration directory path.
func GetConfigDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "EREZMonitor"), nil
}

// GetDefaultConfigPath returns the default configuration file path.
func GetDefaultConfigPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.yaml"), nil
}

// setDefaults sets default configuration values.
func (m *Manager) setDefaults() {
	// Monitoring defaults
	m.viper.SetDefault("monitoring.update_interval", "1s")
	m.viper.SetDefault("monitoring.history_duration", "60s")
	m.viper.SetDefault("monitoring.enable_gpu", true)
	m.viper.SetDefault("monitoring.enable_processes", true)
	m.viper.SetDefault("monitoring.top_process_count", 10)

	// Alerts defaults
	m.viper.SetDefault("alerts.enabled", true)
	m.viper.SetDefault("alerts.cpu_threshold", 80.0)
	m.viper.SetDefault("alerts.ram_threshold", 85.0)
	m.viper.SetDefault("alerts.gpu_threshold", 85.0)
	m.viper.SetDefault("alerts.gpu_temp_threshold", 85.0)
	m.viper.SetDefault("alerts.disk_threshold", 90.0)
	m.viper.SetDefault("alerts.cooldown", "30s")
	m.viper.SetDefault("alerts.sound_enabled", true)

	// UI defaults
	m.viper.SetDefault("ui.tray_enabled", true)
	m.viper.SetDefault("ui.autostart", false)
	m.viper.SetDefault("ui.hotkey", "Ctrl+Shift+M")
	m.viper.SetDefault("ui.theme", "dark")
	m.viper.SetDefault("ui.language", "en")
	m.viper.SetDefault("ui.window_width", 800)
	m.viper.SetDefault("ui.window_height", 600)
	m.viper.SetDefault("ui.refresh_rate", "500ms")

	// Overlay defaults
	m.viper.SetDefault("overlay.enabled", false)
	m.viper.SetDefault("overlay.position", "top-right")
	m.viper.SetDefault("overlay.custom_x", 0)
	m.viper.SetDefault("overlay.custom_y", 0)
	m.viper.SetDefault("overlay.opacity", 0.8)
	m.viper.SetDefault("overlay.font_size", 16)
	m.viper.SetDefault("overlay.show_fps", true)
	m.viper.SetDefault("overlay.show_cpu", true)
	m.viper.SetDefault("overlay.show_ram", true)
	m.viper.SetDefault("overlay.show_gpu", true)
	m.viper.SetDefault("overlay.show_net", true)
	m.viper.SetDefault("overlay.show_disk", true)
	m.viper.SetDefault("overlay.background_color", "#000000")
	m.viper.SetDefault("overlay.text_color", "#FFFFFF")
	m.viper.SetDefault("overlay.hotkey", "Ctrl+Shift+O")
	m.viper.SetDefault("overlay.move_hotkey", "Ctrl+Shift+P")

	// Logging defaults
	m.viper.SetDefault("logging.level", "info")
	m.viper.SetDefault("logging.to_file", true)
	m.viper.SetDefault("logging.file_path", "logs/erez-monitor.log")
	m.viper.SetDefault("logging.csv_export", true)
	m.viper.SetDefault("logging.csv_path", "logs/metrics.csv")
	m.viper.SetDefault("logging.max_file_size", "10MB")
	m.viper.SetDefault("logging.rotation", "daily")
	m.viper.SetDefault("logging.max_age", 7)
	m.viper.SetDefault("logging.max_backups", 5)
}

// createDefaultConfig creates a default configuration file.
func (m *Manager) createDefaultConfig(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Read embedded default config
	data, err := defaultConfig.ReadFile("config.yaml")
	if err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(path, data, 0644)
}

// byteReader implements io.Reader for []byte
type byteReader struct {
	data []byte
	pos  int
}

func newByteReader(data []byte) *byteReader {
	return &byteReader{data: data}
}

func (r *byteReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, fmt.Errorf("EOF")
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// Validate validates the configuration and returns any errors.
func (c *Config) Validate() []error {
	var errs []error

	// Validate monitoring config
	if c.Monitoring.UpdateInterval < 100*time.Millisecond {
		errs = append(errs, fmt.Errorf("update_interval must be at least 100ms"))
	}
	if c.Monitoring.HistoryDuration < time.Second {
		errs = append(errs, fmt.Errorf("history_duration must be at least 1s"))
	}
	if c.Monitoring.TopProcessCount < 1 || c.Monitoring.TopProcessCount > 50 {
		errs = append(errs, fmt.Errorf("top_process_count must be between 1 and 50"))
	}

	// Validate alert thresholds
	if c.Alerts.CPUThreshold < 0 || c.Alerts.CPUThreshold > 100 {
		errs = append(errs, fmt.Errorf("cpu_threshold must be between 0 and 100"))
	}
	if c.Alerts.RAMThreshold < 0 || c.Alerts.RAMThreshold > 100 {
		errs = append(errs, fmt.Errorf("ram_threshold must be between 0 and 100"))
	}
	if c.Alerts.GPUThreshold < 0 || c.Alerts.GPUThreshold > 100 {
		errs = append(errs, fmt.Errorf("gpu_threshold must be between 0 and 100"))
	}
	if c.Alerts.Cooldown < time.Second {
		errs = append(errs, fmt.Errorf("cooldown must be at least 1s"))
	}

	// Validate overlay config
	validPositions := map[string]bool{
		"top-right": true, "top-left": true,
		"bottom-right": true, "bottom-left": true,
	}
	if !validPositions[c.Overlay.Position] {
		errs = append(errs, fmt.Errorf("invalid overlay position: %s", c.Overlay.Position))
	}
	if c.Overlay.Opacity < 0 || c.Overlay.Opacity > 1 {
		errs = append(errs, fmt.Errorf("overlay opacity must be between 0 and 1"))
	}
	if c.Overlay.FontSize < 8 || c.Overlay.FontSize > 72 {
		errs = append(errs, fmt.Errorf("font_size must be between 8 and 72"))
	}

	// Validate logging config
	validLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLevels[c.Logging.Level] {
		errs = append(errs, fmt.Errorf("invalid log level: %s", c.Logging.Level))
	}

	return errs
}
