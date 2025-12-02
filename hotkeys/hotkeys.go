// Package hotkeys provides global hotkey registration for Windows.
package hotkeys

import (
	"context"
	"sync"

	"github.com/NaveLIL/erez-monitor/logger"
	"github.com/NaveLIL/erez-monitor/utils"
)

// HotkeyID represents a unique hotkey identifier.
type HotkeyID int

const (
	HotkeyShowWindow HotkeyID = iota + 1
	HotkeyToggleOverlay
)

// HotkeyHandler is a function that handles a hotkey press.
type HotkeyHandler func()

// Manager manages global hotkey registration.
type Manager struct {
	handlers map[HotkeyID]HotkeyHandler
	mu       sync.RWMutex
	log      *logger.Logger
	hwnd     uintptr
	running  bool
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// New creates a new hotkey manager.
func New() *Manager {
	return &Manager{
		handlers: make(map[HotkeyID]HotkeyHandler),
		log:      logger.Get(),
	}
}

// Register registers a global hotkey.
func (m *Manager) Register(id HotkeyID, hotkey string, handler HotkeyHandler) error {
	modifiers, vk, ok := utils.ParseHotkey(hotkey)
	if !ok {
		m.log.Warnf("Failed to parse hotkey: %s", hotkey)
		return nil // Don't fail, just skip
	}

	err := utils.RegisterHotKey(m.hwnd, int(id), modifiers, vk)
	if err != nil {
		m.log.Warnf("Failed to register hotkey %s: %v", hotkey, err)
		return err
	}

	m.mu.Lock()
	m.handlers[id] = handler
	m.mu.Unlock()

	m.log.Infof("Registered hotkey: %s (ID: %d)", hotkey, id)
	return nil
}

// Unregister unregisters a global hotkey.
func (m *Manager) Unregister(id HotkeyID) error {
	err := utils.UnregisterHotKey(m.hwnd, int(id))
	if err != nil {
		return err
	}

	m.mu.Lock()
	delete(m.handlers, id)
	m.mu.Unlock()

	return nil
}

// Start starts listening for hotkey events.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = true
	m.mu.Unlock()

	ctx, m.cancel = context.WithCancel(ctx)

	m.wg.Add(1)
	go m.messageLoop(ctx)

	m.log.Info("Hotkey manager started")
	return nil
}

// Stop stops listening for hotkey events and unregisters all hotkeys.
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	m.mu.Unlock()

	if m.cancel != nil {
		m.cancel()
	}

	// Unregister all hotkeys
	m.mu.RLock()
	ids := make([]HotkeyID, 0, len(m.handlers))
	for id := range m.handlers {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	for _, id := range ids {
		m.Unregister(id)
	}

	m.wg.Wait()
	m.log.Info("Hotkey manager stopped")
}

// messageLoop processes Windows messages for hotkey events.
func (m *Manager) messageLoop(ctx context.Context) {
	defer m.wg.Done()

	msg := &utils.MSG{}
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Non-blocking message check
			// In a real implementation, we'd use a more sophisticated approach
			// that properly handles the message queue

			ok, err := utils.GetMessage(msg, 0, 0, 0)
			if err != nil || !ok {
				continue
			}

			if msg.Message == utils.WM_HOTKEY {
				id := HotkeyID(msg.WParam)
				m.mu.RLock()
				handler, exists := m.handlers[id]
				m.mu.RUnlock()

				if exists && handler != nil {
					go handler() // Run handler in separate goroutine
				}
			}
		}
	}
}

// RegisterDefaults registers the default hotkeys.
func (m *Manager) RegisterDefaults(showWindow, toggleOverlay string, onShowWindow, onToggleOverlay func()) {
	if showWindow != "" && onShowWindow != nil {
		m.Register(HotkeyShowWindow, showWindow, onShowWindow)
	}

	if toggleOverlay != "" && onToggleOverlay != nil {
		m.Register(HotkeyToggleOverlay, toggleOverlay, onToggleOverlay)
	}
}
