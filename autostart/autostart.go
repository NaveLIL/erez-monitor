// Package autostart provides Windows autostart (registry) functionality.
package autostart

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"

	"github.com/NaveLIL/erez-monitor/logger"
)

const (
	// Registry key for current user autostart
	registryPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	// Application name in registry
	appName = "EREZMonitor"
)

// Manager manages Windows autostart functionality.
type Manager struct {
	log *logger.Logger
}

// New creates a new autostart manager.
func New() *Manager {
	return &Manager{
		log: logger.Get(),
	}
}

// IsEnabled checks if autostart is enabled.
func (m *Manager) IsEnabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.QUERY_VALUE)
	if err != nil {
		return false, fmt.Errorf("failed to open registry key: %w", err)
	}
	defer key.Close()

	_, _, err = key.GetStringValue(appName)
	if err == registry.ErrNotExist {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to read registry value: %w", err)
	}

	return true, nil
}

// Enable enables autostart by adding a registry entry.
func (m *Manager) Enable() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Get absolute path
	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Open registry key with write access
	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer key.Close()

	// Set the value (with quoted path in case of spaces)
	value := fmt.Sprintf(`"%s" --tray-only`, exePath)
	err = key.SetStringValue(appName, value)
	if err != nil {
		return fmt.Errorf("failed to set registry value: %w", err)
	}

	m.log.Infof("Autostart enabled: %s", value)
	return nil
}

// Disable disables autostart by removing the registry entry.
func (m *Manager) Disable() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer key.Close()

	err = key.DeleteValue(appName)
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("failed to delete registry value: %w", err)
	}

	m.log.Info("Autostart disabled")
	return nil
}

// Toggle toggles the autostart setting.
func (m *Manager) Toggle() (bool, error) {
	enabled, err := m.IsEnabled()
	if err != nil {
		return false, err
	}

	if enabled {
		err = m.Disable()
		return false, err
	}

	err = m.Enable()
	return true, err
}

// GetRegistryValue returns the current registry value for autostart.
func (m *Manager) GetRegistryValue() (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("failed to open registry key: %w", err)
	}
	defer key.Close()

	value, _, err := key.GetStringValue(appName)
	if err == registry.ErrNotExist {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to read registry value: %w", err)
	}

	return value, nil
}

// SetStartupArgs sets custom startup arguments for autostart.
func (m *Manager) SetStartupArgs(args string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer key.Close()

	value := fmt.Sprintf(`"%s" %s`, exePath, args)
	err = key.SetStringValue(appName, value)
	if err != nil {
		return fmt.Errorf("failed to set registry value: %w", err)
	}

	return nil
}
