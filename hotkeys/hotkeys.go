// Package hotkeys provides global hotkey registration for Windows.
package hotkeys

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/NaveLIL/erez-monitor/logger"
	"github.com/NaveLIL/erez-monitor/utils"
)

// HotkeyID represents a unique hotkey identifier.
type HotkeyID int

const (
	HotkeyShowWindow HotkeyID = iota + 1
	HotkeyToggleOverlay
	HotkeyMoveOverlay
)

// HotkeyHandler is a function that handles a hotkey press.
type HotkeyHandler func()

// hotkeyRegistration holds info needed to register a hotkey.
type hotkeyRegistration struct {
	id       HotkeyID
	hotkey   string
	handler  HotkeyHandler
	resultCh chan error
}

// Manager manages global hotkey registration.
type Manager struct {
	handlers   map[HotkeyID]HotkeyHandler
	mu         sync.RWMutex
	log        *logger.Logger
	running    bool
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	registerCh chan hotkeyRegistration
}

// New creates a new hotkey manager.
func New() *Manager {
	return &Manager{
		handlers:   make(map[HotkeyID]HotkeyHandler),
		log:        logger.Get(),
		registerCh: make(chan hotkeyRegistration, 10),
	}
}

// Register registers a global hotkey.
// This sends the registration to the message loop goroutine.
func (m *Manager) Register(id HotkeyID, hotkey string, handler HotkeyHandler) error {
	reg := hotkeyRegistration{
		id:       id,
		hotkey:   hotkey,
		handler:  handler,
		resultCh: make(chan error, 1),
	}

	select {
	case m.registerCh <- reg:
		return <-reg.resultCh
	default:
		m.log.Warnf("Failed to queue hotkey registration: %s", hotkey)
		return nil
	}
}

// registerInternal actually registers the hotkey (must be called from message loop thread).
func (m *Manager) registerInternal(id HotkeyID, hotkey string, handler HotkeyHandler) error {
	modifiers, vk, ok := utils.ParseHotkey(hotkey)
	if !ok {
		m.log.Warnf("Failed to parse hotkey: %s", hotkey)
		return nil
	}

	m.log.Infof("Registering hotkey: %s (modifiers=%d, vk=%d)", hotkey, modifiers, vk)

	err := utils.RegisterHotKey(0, int(id), modifiers, vk)
	if err != nil {
		m.log.Errorf("RegisterHotKey failed for %s: %v", hotkey, err)
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
	err := utils.UnregisterHotKey(0, int(id))
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
// IMPORTANT: Hotkey registration and message processing MUST be in the same OS thread.
func (m *Manager) messageLoop(ctx context.Context) {
	defer m.wg.Done()

	// Lock this goroutine to the current OS thread
	// This is required because Windows hotkeys are per-thread
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	msg := &utils.MSG{}
	for {
		// Use GetMessage with timeout simulation via select
		// GetMessage blocks until a message is available
		select {
		case <-ctx.Done():
			return
		case reg := <-m.registerCh:
			// Register hotkey in this thread
			err := m.registerInternal(reg.id, reg.hotkey, reg.handler)
			reg.resultCh <- err
		default:
			// Check for messages - use GetMessage which properly waits
			// But we need to make it non-blocking for ctx.Done() check
			if utils.PeekMessage(msg, 0, 0, 0, 1) { // PM_REMOVE = 1
				if msg.Message == utils.WM_HOTKEY {
					id := HotkeyID(msg.WParam)
					m.log.Infof("Hotkey pressed: ID=%d", id)
					m.mu.RLock()
					handler, exists := m.handlers[id]
					m.mu.RUnlock()

					if exists && handler != nil {
						go handler() // Run handler in separate goroutine
					}
				}
			} else {
				// No message, sleep briefly to avoid busy loop
				time.Sleep(5 * time.Millisecond)
			}
		}
	}
}

// RegisterDefaults registers the default hotkeys.
func (m *Manager) RegisterDefaults(showWindow, toggleOverlay, moveOverlay string, onShowWindow, onToggleOverlay, onMoveOverlay func()) {
	if showWindow != "" && onShowWindow != nil {
		m.Register(HotkeyShowWindow, showWindow, onShowWindow)
	}

	if toggleOverlay != "" && onToggleOverlay != nil {
		m.Register(HotkeyToggleOverlay, toggleOverlay, onToggleOverlay)
	}

	if moveOverlay != "" && onMoveOverlay != nil {
		m.Register(HotkeyMoveOverlay, moveOverlay, onMoveOverlay)
	}
}
