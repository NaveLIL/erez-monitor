//go:build windows

// Package ui provides user interface components for EREZMonitor.
// This file implements the Settings dialog window with full config integration.

package ui

import (
	"fmt"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	"github.com/NaveLIL/erez-monitor/autostart"
	"github.com/NaveLIL/erez-monitor/config"
)

var (
	procMessageBoxW          = user32.NewProc("MessageBoxW")
	procSendMessageW         = user32.NewProc("SendMessageW")
	procSetWindowTextW       = user32.NewProc("SetWindowTextW")
	procGetWindowTextW       = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
	procPeekMessageW         = user32.NewProc("PeekMessageW")
	procIsDialogMessageW     = user32.NewProc("IsDialogMessageW")
	procEnableWindow         = user32.NewProc("EnableWindow")
	procSetFocus             = user32.NewProc("SetFocus")
)

// Window style constants for settings dialog
const (
	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_VISIBLE          = 0x10000000
	WS_CHILD            = 0x40000000
	WS_TABSTOP          = 0x00010000
	WS_VSCROLL          = 0x00200000
	WS_CLIPCHILDREN     = 0x02000000

	WS_EX_CLIENTEDGE    = 0x00000200
	WS_EX_CONTROLPARENT = 0x00010000

	ES_LEFT        = 0x0000
	ES_AUTOHSCROLL = 0x0080
	ES_NUMBER      = 0x2000

	BS_PUSHBUTTON   = 0x00000000
	BS_AUTOCHECKBOX = 0x00000003
	BS_GROUPBOX     = 0x00000007

	CBS_DROPDOWNLIST = 0x0003
	CBS_HASSTRINGS   = 0x0200

	CB_ADDSTRING = 0x0143
	CB_SETCURSEL = 0x014E
	CB_GETCURSEL = 0x0147

	BM_SETCHECK = 0x00F1
	BM_GETCHECK = 0x00F0
	BST_CHECKED = 1

	WM_COMMAND = 0x0111
	WM_CREATE  = 0x0001
	WM_SETFONT = 0x0030
	WM_KEYDOWN = 0x0100

	SS_LEFT = 0x0000

	MB_OK          = 0x00000000
	MB_ICONINFO    = 0x00000040
	MB_ICONWARNING = 0x00000030
	MB_ICONERROR   = 0x00000010

	PM_REMOVE = 0x0001
	WM_QUIT   = 0x0012

	DEFAULT_GUI_FONT = 17

	VK_ESCAPE = 0x1B

	// Command notification codes
	EN_CHANGE     = 0x0300
	CBN_SELCHANGE = 0x0001
	BN_CLICKED    = 0x0000
)

// Control IDs for settings dialog
const (
	ID_OK              = 1
	ID_CANCEL          = 2
	ID_APPLY           = 3
	ID_OVERLAY_ENABLED = 100
	ID_OVERLAY_POS     = 101
	ID_OVERLAY_OPACITY = 102
	ID_ALERT_ENABLED   = 110
	ID_CPU_THRESHOLD   = 111
	ID_RAM_THRESHOLD   = 112
	ID_GPU_THRESHOLD   = 113
	ID_DISK_THRESHOLD  = 114
	ID_AUTOSTART       = 120
	ID_STATUS_LABEL    = 200
)

var (
	procGetStockObject = gdi32.NewProc("GetStockObject")
)

// SettingsWindow represents the settings dialog with full functionality.
type SettingsWindow struct {
	hwnd      uintptr
	hInstance uintptr
	config    *config.Config
	configMgr *config.Manager
	hFont     uintptr

	// Control handles
	controls map[int]uintptr

	// Callbacks for applying changes
	onOverlayToggle   func(enabled bool)
	onOverlayOpacity  func(opacity float64)
	onOverlayPosition func(position string)
	onApply           func()

	// Overlay reference for live preview
	overlay *Overlay

	// Autostart manager
	autostartMgr *autostart.Manager

	// State
	running    bool
	isDirty    bool    // Track if any control has been changed
	statusHwnd uintptr // Status label at bottom
}

var globalSettings *SettingsWindow

// NewSettingsWindow creates a new settings window with full integration.
func NewSettingsWindow(cfg *config.Config, mgr *config.Manager) *SettingsWindow {
	return &SettingsWindow{
		config:       cfg,
		configMgr:    mgr,
		controls:     make(map[int]uintptr),
		autostartMgr: autostart.New(),
	}
}

// SetOverlay sets the overlay reference for live preview.
func (s *SettingsWindow) SetOverlay(overlay *Overlay) {
	s.overlay = overlay
}

// SetCallbacks sets the callback functions for settings changes.
func (s *SettingsWindow) SetCallbacks(onOverlayToggle func(bool), onApply func()) {
	s.onOverlayToggle = onOverlayToggle
	s.onApply = onApply
}

// SetDetailedCallbacks sets detailed callbacks for individual settings.
func (s *SettingsWindow) SetDetailedCallbacks(
	onOverlayToggle func(bool),
	onOverlayOpacity func(float64),
	onOverlayPosition func(string),
	onApply func(),
) {
	s.onOverlayToggle = onOverlayToggle
	s.onOverlayOpacity = onOverlayOpacity
	s.onOverlayPosition = onOverlayPosition
	s.onApply = onApply
}

// Show displays the settings window.
func (s *SettingsWindow) Show() {
	if s.running {
		return
	}
	s.running = true
	s.isDirty = false
	globalSettings = s

	// Get module handle
	s.hInstance, _, _ = procGetModuleHandleW.Call(0)

	// Get system font
	s.hFont, _, _ = procGetStockObject.Call(DEFAULT_GUI_FONT)

	// Register window class
	className, _ := syscall.UTF16PtrFromString("EREZSettingsV2")

	var wc WNDCLASSEXW
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	wc.Style = CS_HREDRAW | CS_VREDRAW
	wc.LpfnWndProc = syscall.NewCallback(settingsWndProc)
	wc.HInstance = s.hInstance
	wc.LpszClassName = className
	wc.HbrBackground = 16 // COLOR_BTNFACE + 1

	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	// Get screen size for centering
	screenWidth, _, _ := procGetSystemMetrics.Call(0)
	screenHeight, _, _ := procGetSystemMetrics.Call(1)

	windowWidth := int32(400)
	windowHeight := int32(480)
	x := (int32(screenWidth) - windowWidth) / 2
	y := (int32(screenHeight) - windowHeight) / 2

	windowName, _ := syscall.UTF16PtrFromString("EREZMonitor - Настройки")

	hwnd, _, _ := procCreateWindowExW.Call(
		uintptr(WS_EX_CONTROLPARENT),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		uintptr(WS_OVERLAPPEDWINDOW&^0x00040000|WS_VISIBLE|WS_CLIPCHILDREN), // Remove WS_THICKFRAME
		uintptr(x), uintptr(y),
		uintptr(windowWidth), uintptr(windowHeight),
		0, 0, s.hInstance, 0,
	)

	if hwnd == 0 {
		s.running = false
		return
	}

	s.hwnd = hwnd
	s.createControls()
	s.loadSettings()
	s.updateControlStates() // Enable/disable based on checkboxes

	procShowWindow.Call(hwnd, SW_SHOW)
	procInvalidateRect.Call(hwnd, 0, 1)

	// Non-blocking message loop with Esc key support
	var msg MSG
	for s.running {
		ret, _, _ := procPeekMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
			0, 0, 0, PM_REMOVE,
		)

		if ret != 0 {
			if msg.Message == WM_QUIT {
				break
			}

			// Handle Esc key for cancel
			if msg.Message == WM_KEYDOWN && msg.WParam == VK_ESCAPE {
				s.close()
				break
			}

			// Check if it's a dialog message (handles Tab, etc.)
			isDialog, _, _ := procIsDialogMessageW.Call(s.hwnd, uintptr(unsafe.Pointer(&msg)))
			if isDialog == 0 {
				procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
				procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
			}
		} else {
			// No messages - sleep briefly to avoid CPU spin
			time.Sleep(10 * time.Millisecond)
		}
	}

	s.running = false
}

// createControls creates all dialog controls.
func (s *SettingsWindow) createControls() {
	staticClass, _ := syscall.UTF16PtrFromString("STATIC")
	editClass, _ := syscall.UTF16PtrFromString("EDIT")
	buttonClass, _ := syscall.UTF16PtrFromString("BUTTON")
	comboClass, _ := syscall.UTF16PtrFromString("COMBOBOX")

	y := int32(15)
	labelWidth := int32(150)
	inputWidth := int32(120)
	inputHeight := int32(24)
	spacing := int32(32)
	leftMargin := int32(20)
	inputX := leftMargin + labelWidth + 10

	// ═══════════════════════════════════════════════════════════════
	// OVERLAY SECTION
	// ═══════════════════════════════════════════════════════════════
	s.createGroupBox(buttonClass, "Оверлей", leftMargin-5, y-5, 355, 115)
	y += 18

	// Checkbox: Enable overlay
	s.controls[ID_OVERLAY_ENABLED] = s.createCheckbox(buttonClass, "Включить оверлей",
		leftMargin+5, y, 200, 20, ID_OVERLAY_ENABLED)
	y += spacing

	// ComboBox: Position
	s.createStatic(staticClass, "Позиция:", leftMargin+5, y+3, labelWidth, 18)
	s.controls[ID_OVERLAY_POS] = s.createComboBox(comboClass, inputX, y, inputWidth, 120, ID_OVERLAY_POS)
	s.addComboItem(s.controls[ID_OVERLAY_POS], "Сверху справа")
	s.addComboItem(s.controls[ID_OVERLAY_POS], "Сверху слева")
	s.addComboItem(s.controls[ID_OVERLAY_POS], "Снизу справа")
	s.addComboItem(s.controls[ID_OVERLAY_POS], "Снизу слева")
	y += spacing

	// Edit: Opacity
	s.createStatic(staticClass, "Прозрачность (%):", leftMargin+5, y+3, labelWidth, 18)
	s.controls[ID_OVERLAY_OPACITY] = s.createEdit(editClass, "", inputX, y, 60, inputHeight, ID_OVERLAY_OPACITY, true)
	y += spacing + 15

	// ═══════════════════════════════════════════════════════════════
	// ALERTS SECTION
	// ═══════════════════════════════════════════════════════════════
	s.createGroupBox(buttonClass, "Алерты", leftMargin-5, y-5, 355, 175)
	y += 18

	// Checkbox: Enable alerts
	s.controls[ID_ALERT_ENABLED] = s.createCheckbox(buttonClass, "Включить алерты",
		leftMargin+5, y, 200, 20, ID_ALERT_ENABLED)
	y += spacing

	// Edit: CPU threshold
	s.createStatic(staticClass, "Порог CPU (%):", leftMargin+5, y+3, labelWidth, 18)
	s.controls[ID_CPU_THRESHOLD] = s.createEdit(editClass, "", inputX, y, 60, inputHeight, ID_CPU_THRESHOLD, true)
	y += spacing

	// Edit: RAM threshold
	s.createStatic(staticClass, "Порог RAM (%):", leftMargin+5, y+3, labelWidth, 18)
	s.controls[ID_RAM_THRESHOLD] = s.createEdit(editClass, "", inputX, y, 60, inputHeight, ID_RAM_THRESHOLD, true)
	y += spacing

	// Edit: GPU threshold
	s.createStatic(staticClass, "Порог GPU (%):", leftMargin+5, y+3, labelWidth, 18)
	s.controls[ID_GPU_THRESHOLD] = s.createEdit(editClass, "", inputX, y, 60, inputHeight, ID_GPU_THRESHOLD, true)
	y += spacing

	// Edit: Disk threshold
	s.createStatic(staticClass, "Порог диска (%):", leftMargin+5, y+3, labelWidth, 18)
	s.controls[ID_DISK_THRESHOLD] = s.createEdit(editClass, "", inputX, y, 60, inputHeight, ID_DISK_THRESHOLD, true)
	y += spacing + 15

	// ═══════════════════════════════════════════════════════════════
	// GENERAL SECTION
	// ═══════════════════════════════════════════════════════════════
	s.createGroupBox(buttonClass, "Общие", leftMargin-5, y-5, 355, 55)
	y += 18

	// Checkbox: Autostart
	s.controls[ID_AUTOSTART] = s.createCheckbox(buttonClass, "Запуск с Windows",
		leftMargin+5, y, 200, 20, ID_AUTOSTART)
	y += spacing + 25

	// ═══════════════════════════════════════════════════════════════
	// BUTTONS
	// ═══════════════════════════════════════════════════════════════
	buttonWidth := int32(85)
	buttonHeight := int32(28)
	buttonY := int32(405)
	buttonSpacing := int32(95)
	buttonStartX := int32(50)

	s.createButton(buttonClass, "OK", buttonStartX, buttonY, buttonWidth, buttonHeight, ID_OK, BS_PUSHBUTTON)
	s.createButton(buttonClass, "Отмена", buttonStartX+buttonSpacing, buttonY, buttonWidth, buttonHeight, ID_CANCEL, BS_PUSHBUTTON)
	s.controls[ID_APPLY] = s.createButton(buttonClass, "Применить", buttonStartX+buttonSpacing*2, buttonY, buttonWidth, buttonHeight, ID_APPLY, BS_PUSHBUTTON)

	// Status label at bottom
	s.statusHwnd = s.createStatic(staticClass, "", leftMargin, buttonY+35, 300, 18)
}

// createGroupBox creates a group box.
func (s *SettingsWindow) createGroupBox(class *uint16, text string, x, y, w, h int32) uintptr {
	textPtr, _ := syscall.UTF16PtrFromString(text)
	hwnd, _, _ := procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(textPtr)),
		uintptr(WS_CHILD|WS_VISIBLE|BS_GROUPBOX),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		s.hwnd, 0, s.hInstance, 0,
	)
	procSendMessageW.Call(hwnd, WM_SETFONT, s.hFont, 1)
	return hwnd
}

// createStatic creates a static text label.
func (s *SettingsWindow) createStatic(class *uint16, text string, x, y, w, h int32) uintptr {
	textPtr, _ := syscall.UTF16PtrFromString(text)
	hwnd, _, _ := procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(textPtr)),
		uintptr(WS_CHILD|WS_VISIBLE|SS_LEFT),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		s.hwnd, 0, s.hInstance, 0,
	)
	procSendMessageW.Call(hwnd, WM_SETFONT, s.hFont, 1)
	return hwnd
}

// createEdit creates an edit control. If numbersOnly is true, only digits are allowed.
func (s *SettingsWindow) createEdit(class *uint16, text string, x, y, w, h int32, id int, numbersOnly bool) uintptr {
	textPtr, _ := syscall.UTF16PtrFromString(text)
	style := uintptr(WS_CHILD | WS_VISIBLE | WS_TABSTOP | ES_LEFT | ES_AUTOHSCROLL)
	if numbersOnly {
		style |= ES_NUMBER
	}
	hwnd, _, _ := procCreateWindowExW.Call(
		uintptr(WS_EX_CLIENTEDGE),
		uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(textPtr)),
		style,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		s.hwnd, uintptr(id), s.hInstance, 0,
	)
	procSendMessageW.Call(hwnd, WM_SETFONT, s.hFont, 1)
	return hwnd
}

// createCheckbox creates a checkbox control.
func (s *SettingsWindow) createCheckbox(class *uint16, text string, x, y, w, h int32, id int) uintptr {
	textPtr, _ := syscall.UTF16PtrFromString(text)
	hwnd, _, _ := procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(textPtr)),
		uintptr(WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_AUTOCHECKBOX),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		s.hwnd, uintptr(id), s.hInstance, 0,
	)
	procSendMessageW.Call(hwnd, WM_SETFONT, s.hFont, 1)
	return hwnd
}

// createButton creates a button control.
func (s *SettingsWindow) createButton(class *uint16, text string, x, y, w, h int32, id int, style uintptr) uintptr {
	textPtr, _ := syscall.UTF16PtrFromString(text)
	hwnd, _, _ := procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(textPtr)),
		uintptr(WS_CHILD|WS_VISIBLE|WS_TABSTOP)|style,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		s.hwnd, uintptr(id), s.hInstance, 0,
	)
	procSendMessageW.Call(hwnd, WM_SETFONT, s.hFont, 1)
	return hwnd
}

// createComboBox creates a dropdown combobox.
func (s *SettingsWindow) createComboBox(class *uint16, x, y, w, h int32, id int) uintptr {
	hwnd, _, _ := procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(class)), 0,
		uintptr(WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_VSCROLL|CBS_DROPDOWNLIST|CBS_HASSTRINGS),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		s.hwnd, uintptr(id), s.hInstance, 0,
	)
	procSendMessageW.Call(hwnd, WM_SETFONT, s.hFont, 1)
	return hwnd
}

// addComboItem adds an item to a combobox.
func (s *SettingsWindow) addComboItem(hwnd uintptr, text string) {
	textPtr, _ := syscall.UTF16PtrFromString(text)
	procSendMessageW.Call(hwnd, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(textPtr)))
}

// loadSettings loads current config values into controls.
func (s *SettingsWindow) loadSettings() {
	// ═══════════════════════════════════════════════════════════════
	// OVERLAY SETTINGS
	// ═══════════════════════════════════════════════════════════════

	// Checkbox: Overlay enabled
	if s.config.Overlay.Enabled {
		procSendMessageW.Call(s.controls[ID_OVERLAY_ENABLED], BM_SETCHECK, BST_CHECKED, 0)
	}

	// ComboBox: Position - map config position string to combo index
	positions := []string{"top-right", "top-left", "bottom-right", "bottom-left"}
	for i, pos := range positions {
		if pos == s.config.Overlay.Position {
			procSendMessageW.Call(s.controls[ID_OVERLAY_POS], CB_SETCURSEL, uintptr(i), 0)
			break
		}
	}

	// Edit: Opacity (convert from 0.0-1.0 to 0-100)
	opacity := int(s.config.Overlay.Opacity * 100)
	if opacity < 20 {
		opacity = 20
	}
	if opacity > 100 {
		opacity = 100
	}
	s.setEditText(s.controls[ID_OVERLAY_OPACITY], fmt.Sprintf("%d", opacity))

	// ═══════════════════════════════════════════════════════════════
	// ALERTS SETTINGS
	// ═══════════════════════════════════════════════════════════════

	// Checkbox: Alerts enabled
	if s.config.Alerts.Enabled {
		procSendMessageW.Call(s.controls[ID_ALERT_ENABLED], BM_SETCHECK, BST_CHECKED, 0)
	}

	// Edit: Thresholds
	s.setEditText(s.controls[ID_CPU_THRESHOLD], fmt.Sprintf("%.0f", s.config.Alerts.CPUThreshold))
	s.setEditText(s.controls[ID_RAM_THRESHOLD], fmt.Sprintf("%.0f", s.config.Alerts.RAMThreshold))
	s.setEditText(s.controls[ID_GPU_THRESHOLD], fmt.Sprintf("%.0f", s.config.Alerts.GPUThreshold))
	s.setEditText(s.controls[ID_DISK_THRESHOLD], fmt.Sprintf("%.0f", s.config.Alerts.DiskThreshold))

	// ═══════════════════════════════════════════════════════════════
	// GENERAL SETTINGS
	// ═══════════════════════════════════════════════════════════════

	// Checkbox: Autostart (check actual registry state)
	if isEnabled, _ := s.autostartMgr.IsEnabled(); isEnabled {
		procSendMessageW.Call(s.controls[ID_AUTOSTART], BM_SETCHECK, BST_CHECKED, 0)
	}

	// Disable Apply button initially (no changes yet)
	s.setApplyEnabled(false)
}

// updateControlStates enables/disables controls based on checkbox states.
func (s *SettingsWindow) updateControlStates() {
	// Overlay controls: disable if overlay is disabled
	overlayEnabled := s.isChecked(ID_OVERLAY_ENABLED)
	s.enableControl(ID_OVERLAY_POS, overlayEnabled)
	s.enableControl(ID_OVERLAY_OPACITY, overlayEnabled)

	// Alert controls: disable if alerts are disabled
	alertsEnabled := s.isChecked(ID_ALERT_ENABLED)
	s.enableControl(ID_CPU_THRESHOLD, alertsEnabled)
	s.enableControl(ID_RAM_THRESHOLD, alertsEnabled)
	s.enableControl(ID_GPU_THRESHOLD, alertsEnabled)
	s.enableControl(ID_DISK_THRESHOLD, alertsEnabled)
}

// enableControl enables or disables a control.
func (s *SettingsWindow) enableControl(id int, enable bool) {
	if hwnd, ok := s.controls[id]; ok {
		var flag uintptr = 0
		if enable {
			flag = 1
		}
		procEnableWindow.Call(hwnd, flag)
	}
}

// isChecked returns true if a checkbox is checked.
func (s *SettingsWindow) isChecked(id int) bool {
	if hwnd, ok := s.controls[id]; ok {
		ret, _, _ := procSendMessageW.Call(hwnd, BM_GETCHECK, 0, 0)
		return ret == BST_CHECKED
	}
	return false
}

// markDirty marks the config as changed and enables Apply button.
func (s *SettingsWindow) markDirty() {
	if !s.isDirty {
		s.isDirty = true
		s.setApplyEnabled(true)
		s.setStatus("")
	}
}

// setApplyEnabled enables or disables the Apply button.
func (s *SettingsWindow) setApplyEnabled(enabled bool) {
	if hwnd, ok := s.controls[ID_APPLY]; ok {
		var flag uintptr = 0
		if enabled {
			flag = 1
		}
		procEnableWindow.Call(hwnd, flag)
	}
}

// setStatus sets the status text at the bottom of the dialog.
func (s *SettingsWindow) setStatus(text string) {
	if s.statusHwnd != 0 {
		s.setWindowText(s.statusHwnd, text)
	}
}

// setWindowText sets the text of a window/control.
func (s *SettingsWindow) setWindowText(hwnd uintptr, text string) {
	textPtr, _ := syscall.UTF16PtrFromString(text)
	procSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(textPtr)))
}

// validateAndSave validates all inputs and saves if valid.
// Returns true if save was successful.
func (s *SettingsWindow) validateAndSave() bool {
	// ═══════════════════════════════════════════════════════════════
	// VALIDATION
	// ═══════════════════════════════════════════════════════════════

	// Validate opacity (20-100)
	opacity, ok := s.parsePercent(ID_OVERLAY_OPACITY, 20, 100, "Прозрачность")
	if !ok {
		return false
	}

	// Validate thresholds (1-100) - only if alerts are enabled
	var cpuThreshold, ramThreshold, gpuThreshold, diskThreshold int
	if s.isChecked(ID_ALERT_ENABLED) {
		cpuThreshold, ok = s.parsePercent(ID_CPU_THRESHOLD, 1, 100, "Порог CPU")
		if !ok {
			return false
		}
		ramThreshold, ok = s.parsePercent(ID_RAM_THRESHOLD, 1, 100, "Порог RAM")
		if !ok {
			return false
		}
		gpuThreshold, ok = s.parsePercent(ID_GPU_THRESHOLD, 1, 100, "Порог GPU")
		if !ok {
			return false
		}
		diskThreshold, ok = s.parsePercent(ID_DISK_THRESHOLD, 1, 100, "Порог диска")
		if !ok {
			return false
		}
	} else {
		// Use existing values if alerts are disabled
		cpuThreshold = int(s.config.Alerts.CPUThreshold)
		ramThreshold = int(s.config.Alerts.RAMThreshold)
		gpuThreshold = int(s.config.Alerts.GPUThreshold)
		diskThreshold = int(s.config.Alerts.DiskThreshold)
	}

	// ═══════════════════════════════════════════════════════════════
	// SAVE CONFIG
	// ═══════════════════════════════════════════════════════════════

	// Track what changed for callbacks
	oldOverlayEnabled := s.config.Overlay.Enabled
	oldOverlayPosition := s.config.Overlay.Position
	oldOverlayOpacity := s.config.Overlay.Opacity

	// Update overlay settings
	s.config.Overlay.Enabled = s.isChecked(ID_OVERLAY_ENABLED)

	// Get position from combo
	sel, _, _ := procSendMessageW.Call(s.controls[ID_OVERLAY_POS], CB_GETCURSEL, 0, 0)
	positions := []string{"top-right", "top-left", "bottom-right", "bottom-left"}
	if int(sel) >= 0 && int(sel) < len(positions) {
		s.config.Overlay.Position = positions[sel]
	}

	s.config.Overlay.Opacity = float64(opacity) / 100.0

	// Update alerts settings
	s.config.Alerts.Enabled = s.isChecked(ID_ALERT_ENABLED)
	s.config.Alerts.CPUThreshold = float64(cpuThreshold)
	s.config.Alerts.RAMThreshold = float64(ramThreshold)
	s.config.Alerts.GPUThreshold = float64(gpuThreshold)
	s.config.Alerts.DiskThreshold = float64(diskThreshold)

	// Update UI settings
	newAutostart := s.isChecked(ID_AUTOSTART)
	oldAutostart := s.config.UI.Autostart
	s.config.UI.Autostart = newAutostart

	// Save to file
	if s.configMgr != nil {
		if err := s.configMgr.Save(); err != nil {
			s.showError("Ошибка сохранения", fmt.Sprintf("Не удалось сохранить настройки:\n%v", err))
			return false
		}
	}

	// ═══════════════════════════════════════════════════════════════
	// APPLY CHANGES
	// ═══════════════════════════════════════════════════════════════

	// Handle overlay enable/disable
	if s.config.Overlay.Enabled != oldOverlayEnabled {
		if s.onOverlayToggle != nil {
			s.onOverlayToggle(s.config.Overlay.Enabled)
		}
		if s.overlay != nil {
			if s.config.Overlay.Enabled {
				s.overlay.Show()
			} else {
				s.overlay.Hide()
			}
		}
	}

	// Handle overlay position change
	if s.config.Overlay.Position != oldOverlayPosition {
		if s.onOverlayPosition != nil {
			s.onOverlayPosition(s.config.Overlay.Position)
		}
		if s.overlay != nil {
			s.overlay.UpdatePosition(s.config.Overlay.Position)
		}
	}

	// Handle overlay opacity change
	if s.config.Overlay.Opacity != oldOverlayOpacity {
		if s.onOverlayOpacity != nil {
			s.onOverlayOpacity(s.config.Overlay.Opacity)
		}
		if s.overlay != nil {
			s.overlay.SetOpacity(s.config.Overlay.Opacity)
		}
	}

	// Handle autostart change
	if newAutostart != oldAutostart {
		if newAutostart {
			if err := s.autostartMgr.Enable(); err != nil {
				s.showWarning("Автозапуск", fmt.Sprintf("Не удалось включить автозапуск:\n%v", err))
			}
		} else {
			if err := s.autostartMgr.Disable(); err != nil {
				s.showWarning("Автозапуск", fmt.Sprintf("Не удалось отключить автозапуск:\n%v", err))
			}
		}
	}

	// Call general apply callback
	if s.onApply != nil {
		s.onApply()
	}

	// Mark as clean
	s.isDirty = false
	s.setApplyEnabled(false)
	s.setStatus("✓ Настройки сохранены")

	return true
}

// parsePercent parses and validates a percentage value from an edit control.
// Shows error message and focuses the control if validation fails.
func (s *SettingsWindow) parsePercent(controlID int, min, max int, fieldName string) (int, bool) {
	hwnd := s.controls[controlID]
	text := s.getEditText(hwnd)

	value, err := strconv.Atoi(text)
	if err != nil {
		s.showError("Ошибка ввода",
			fmt.Sprintf("%s: введите целое число от %d до %d", fieldName, min, max))
		procSetFocus.Call(hwnd)
		return 0, false
	}

	if value < min || value > max {
		s.showError("Ошибка ввода",
			fmt.Sprintf("%s: значение должно быть от %d до %d\nВведено: %d", fieldName, min, max, value))
		procSetFocus.Call(hwnd)
		return 0, false
	}

	return value, true
}

// setEditText sets the text of an edit control.
func (s *SettingsWindow) setEditText(hwnd uintptr, text string) {
	textPtr, _ := syscall.UTF16PtrFromString(text)
	procSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(textPtr)))
}

// getEditText gets the text from an edit control.
func (s *SettingsWindow) getEditText(hwnd uintptr) string {
	length, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if length == 0 {
		return ""
	}

	buf := make([]uint16, length+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), length+1)
	return syscall.UTF16ToString(buf)
}

// showError shows an error message box.
func (s *SettingsWindow) showError(title, text string) {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	textPtr, _ := syscall.UTF16PtrFromString(text)
	procMessageBoxW.Call(s.hwnd, uintptr(unsafe.Pointer(textPtr)), uintptr(unsafe.Pointer(titlePtr)), MB_OK|MB_ICONERROR)
}

// showWarning shows a warning message box.
func (s *SettingsWindow) showWarning(title, text string) {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	textPtr, _ := syscall.UTF16PtrFromString(text)
	procMessageBoxW.Call(s.hwnd, uintptr(unsafe.Pointer(textPtr)), uintptr(unsafe.Pointer(titlePtr)), MB_OK|MB_ICONWARNING)
}

// showInfo shows an info message box.
func (s *SettingsWindow) showInfo(title, text string) {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	textPtr, _ := syscall.UTF16PtrFromString(text)
	procMessageBoxW.Call(s.hwnd, uintptr(unsafe.Pointer(textPtr)), uintptr(unsafe.Pointer(titlePtr)), MB_OK|MB_ICONINFO)
}

// close closes the settings window.
func (s *SettingsWindow) close() {
	s.running = false
	if s.hwnd != 0 {
		procDestroyWindow.Call(s.hwnd)
		s.hwnd = 0
	}
}

// settingsWndProc is the window procedure for the settings dialog.
func settingsWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_COMMAND:
		id := int(wParam & 0xFFFF)
		notifyCode := int((wParam >> 16) & 0xFFFF)

		switch id {
		case ID_OK:
			// Save and close
			if globalSettings != nil {
				if globalSettings.validateAndSave() {
					globalSettings.close()
				}
			}
			return 0

		case ID_CANCEL:
			// Just close without saving
			if globalSettings != nil {
				globalSettings.close()
			}
			return 0

		case ID_APPLY:
			// Save but don't close
			if globalSettings != nil {
				globalSettings.validateAndSave()
			}
			return 0

		case ID_OVERLAY_ENABLED:
			// Checkbox clicked - update control states and mark dirty
			if notifyCode == BN_CLICKED && globalSettings != nil {
				globalSettings.updateControlStates()
				globalSettings.markDirty()

				// Live preview: toggle overlay visibility immediately
				if globalSettings.overlay != nil {
					if globalSettings.isChecked(ID_OVERLAY_ENABLED) {
						globalSettings.overlay.Show()
					} else {
						globalSettings.overlay.Hide()
					}
				}
			}
			return 0

		case ID_ALERT_ENABLED:
			// Checkbox clicked - update control states and mark dirty
			if notifyCode == BN_CLICKED && globalSettings != nil {
				globalSettings.updateControlStates()
				globalSettings.markDirty()
			}
			return 0

		case ID_AUTOSTART:
			// Checkbox clicked - mark dirty
			if notifyCode == BN_CLICKED && globalSettings != nil {
				globalSettings.markDirty()
			}
			return 0

		case ID_OVERLAY_POS:
			// ComboBox selection changed - mark dirty and live preview
			if notifyCode == CBN_SELCHANGE && globalSettings != nil {
				globalSettings.markDirty()

				// Live preview: update position immediately
				if globalSettings.overlay != nil {
					sel, _, _ := procSendMessageW.Call(globalSettings.controls[ID_OVERLAY_POS], CB_GETCURSEL, 0, 0)
					positions := []string{"top-right", "top-left", "bottom-right", "bottom-left"}
					if int(sel) >= 0 && int(sel) < len(positions) {
						globalSettings.overlay.UpdatePosition(positions[sel])
					}
				}
			}
			return 0

		case ID_OVERLAY_OPACITY:
			// Edit changed - mark dirty and live preview
			if notifyCode == EN_CHANGE && globalSettings != nil {
				globalSettings.markDirty()

				// Live preview: update opacity immediately (only if valid)
				if globalSettings.overlay != nil {
					text := globalSettings.getEditText(globalSettings.controls[ID_OVERLAY_OPACITY])
					if opacity, err := strconv.Atoi(text); err == nil && opacity >= 20 && opacity <= 100 {
						globalSettings.overlay.SetOpacity(float64(opacity) / 100.0)
					}
				}
			}
			return 0

		case ID_CPU_THRESHOLD, ID_RAM_THRESHOLD, ID_GPU_THRESHOLD, ID_DISK_THRESHOLD:
			// Edit changed - mark dirty
			if notifyCode == EN_CHANGE && globalSettings != nil {
				globalSettings.markDirty()
			}
			return 0
		}

	case WM_DESTROY:
		if globalSettings != nil {
			globalSettings.running = false
		}
		procPostQuitMessage.Call(0)
		return 0

	case WM_CLOSE:
		if globalSettings != nil {
			globalSettings.close()
		}
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return ret
}
