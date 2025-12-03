//go:build windows

package ui

import (
	"fmt"
	"strconv"
	"syscall"
	"time"
	"unsafe"

	"github.com/NaveLIL/erez-monitor/config"
)

var (
	procMessageBoxW          = user32.NewProc("MessageBoxW")
	procSendMessageW         = user32.NewProc("SendMessageW")
	procSetWindowTextW       = user32.NewProc("SetWindowTextW")
	procGetWindowTextW       = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
	procMoveWindow           = user32.NewProc("MoveWindow")
	procPeekMessageW         = user32.NewProc("PeekMessageW")
	procIsDialogMessageW     = user32.NewProc("IsDialogMessageW")
)

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

	BS_PUSHBUTTON   = 0x00000000
	BS_AUTOCHECKBOX = 0x00000003

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

	SS_LEFT = 0x0000

	MB_OK       = 0x00000000
	MB_ICONINFO = 0x00000040

	PM_REMOVE = 0x0001
	WM_QUIT   = 0x0012

	DEFAULT_GUI_FONT = 17
)

// Control IDs
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
)

var (
	procGetStockObject = gdi32.NewProc("GetStockObject")
)

// SettingsWindow represents the settings dialog.
type SettingsWindow struct {
	hwnd      uintptr
	hInstance uintptr
	config    *config.Config
	configMgr *config.Manager
	hFont     uintptr

	// Control handles
	controls map[int]uintptr

	// Callbacks
	onOverlayToggle func(enabled bool)
	onApply         func()

	running bool
}

var globalSettings *SettingsWindow

// NewSettingsWindow creates a new settings window.
func NewSettingsWindow(cfg *config.Config, mgr *config.Manager) *SettingsWindow {
	return &SettingsWindow{
		config:    cfg,
		configMgr: mgr,
		controls:  make(map[int]uintptr),
	}
}

// SetCallbacks sets the callback functions.
func (s *SettingsWindow) SetCallbacks(onOverlayToggle func(bool), onApply func()) {
	s.onOverlayToggle = onOverlayToggle
	s.onApply = onApply
}

// Show displays the settings window.
func (s *SettingsWindow) Show() {
	if s.running {
		return
	}
	s.running = true
	globalSettings = s

	// Get module handle
	s.hInstance, _, _ = procGetModuleHandleW.Call(0)

	// Get system font
	s.hFont, _, _ = procGetStockObject.Call(DEFAULT_GUI_FONT)

	// Register window class
	className, _ := syscall.UTF16PtrFromString("EREZSettings")

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

	windowWidth := int32(380)
	windowHeight := int32(420)
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

	procShowWindow.Call(hwnd, SW_SHOW)
	procInvalidateRect.Call(hwnd, 0, 1)

	// Non-blocking message loop
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

func (s *SettingsWindow) createControls() {
	staticClass, _ := syscall.UTF16PtrFromString("STATIC")
	editClass, _ := syscall.UTF16PtrFromString("EDIT")
	buttonClass, _ := syscall.UTF16PtrFromString("BUTTON")
	comboClass, _ := syscall.UTF16PtrFromString("COMBOBOX")

	y := int32(15)
	labelWidth := int32(140)
	inputWidth := int32(180)
	inputHeight := int32(22)
	spacing := int32(30)
	leftMargin := int32(20)
	inputX := leftMargin + labelWidth + 10

	// === Overlay Section ===
	s.createStatic(staticClass, "── Оверлей ──", leftMargin, y, 320, 18)
	y += 25

	// Overlay enabled checkbox
	s.controls[ID_OVERLAY_ENABLED] = s.createButton(buttonClass, "Включить оверлей",
		leftMargin, y, 200, 20, ID_OVERLAY_ENABLED, BS_AUTOCHECKBOX)
	y += spacing

	// Position
	s.createStatic(staticClass, "Позиция:", leftMargin, y+2, labelWidth, 18)
	s.controls[ID_OVERLAY_POS] = s.createComboBox(comboClass, inputX, y, inputWidth, 100, ID_OVERLAY_POS)
	s.addComboItem(s.controls[ID_OVERLAY_POS], "Сверху справа")
	s.addComboItem(s.controls[ID_OVERLAY_POS], "Сверху слева")
	s.addComboItem(s.controls[ID_OVERLAY_POS], "Снизу справа")
	s.addComboItem(s.controls[ID_OVERLAY_POS], "Снизу слева")
	y += spacing

	// Opacity
	s.createStatic(staticClass, "Прозрачность (%):", leftMargin, y+2, labelWidth, 18)
	s.controls[ID_OVERLAY_OPACITY] = s.createEdit(editClass, "", inputX, y, 50, inputHeight, ID_OVERLAY_OPACITY)
	y += spacing + 10

	// === Alerts Section ===
	s.createStatic(staticClass, "── Алерты ──", leftMargin, y, 320, 18)
	y += 25

	// Alerts enabled
	s.controls[ID_ALERT_ENABLED] = s.createButton(buttonClass, "Включить алерты",
		leftMargin, y, 200, 20, ID_ALERT_ENABLED, BS_AUTOCHECKBOX)
	y += spacing

	// CPU threshold
	s.createStatic(staticClass, "Порог CPU (%):", leftMargin, y+2, labelWidth, 18)
	s.controls[ID_CPU_THRESHOLD] = s.createEdit(editClass, "", inputX, y, 50, inputHeight, ID_CPU_THRESHOLD)
	y += spacing

	// RAM threshold
	s.createStatic(staticClass, "Порог RAM (%):", leftMargin, y+2, labelWidth, 18)
	s.controls[ID_RAM_THRESHOLD] = s.createEdit(editClass, "", inputX, y, 50, inputHeight, ID_RAM_THRESHOLD)
	y += spacing

	// GPU threshold
	s.createStatic(staticClass, "Порог GPU (%):", leftMargin, y+2, labelWidth, 18)
	s.controls[ID_GPU_THRESHOLD] = s.createEdit(editClass, "", inputX, y, 50, inputHeight, ID_GPU_THRESHOLD)
	y += spacing

	// Disk threshold
	s.createStatic(staticClass, "Порог диска (%):", leftMargin, y+2, labelWidth, 18)
	s.controls[ID_DISK_THRESHOLD] = s.createEdit(editClass, "", inputX, y, 50, inputHeight, ID_DISK_THRESHOLD)
	y += spacing + 10

	// === General Section ===
	s.createStatic(staticClass, "── Общие ──", leftMargin, y, 320, 18)
	y += 25

	// Autostart
	s.controls[ID_AUTOSTART] = s.createButton(buttonClass, "Запуск с Windows",
		leftMargin, y, 200, 20, ID_AUTOSTART, BS_AUTOCHECKBOX)

	// Buttons at bottom
	buttonWidth := int32(80)
	buttonHeight := int32(28)
	buttonY := int32(350)
	buttonSpacing := int32(90)

	s.createButton(buttonClass, "OK", 70, buttonY, buttonWidth, buttonHeight, ID_OK, BS_PUSHBUTTON)
	s.createButton(buttonClass, "Отмена", 70+buttonSpacing, buttonY, buttonWidth, buttonHeight, ID_CANCEL, BS_PUSHBUTTON)
	s.createButton(buttonClass, "Применить", 70+buttonSpacing*2, buttonY, buttonWidth, buttonHeight, ID_APPLY, BS_PUSHBUTTON)
}

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

func (s *SettingsWindow) createEdit(class *uint16, text string, x, y, w, h int32, id int) uintptr {
	textPtr, _ := syscall.UTF16PtrFromString(text)
	hwnd, _, _ := procCreateWindowExW.Call(
		uintptr(WS_EX_CLIENTEDGE),
		uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(textPtr)),
		uintptr(WS_CHILD|WS_VISIBLE|WS_TABSTOP|ES_LEFT|ES_AUTOHSCROLL),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		s.hwnd, uintptr(id), s.hInstance, 0,
	)
	procSendMessageW.Call(hwnd, WM_SETFONT, s.hFont, 1)
	return hwnd
}

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

func (s *SettingsWindow) addComboItem(hwnd uintptr, text string) {
	textPtr, _ := syscall.UTF16PtrFromString(text)
	procSendMessageW.Call(hwnd, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(textPtr)))
}

func (s *SettingsWindow) loadSettings() {
	// Overlay
	if s.config.Overlay.Enabled {
		procSendMessageW.Call(s.controls[ID_OVERLAY_ENABLED], BM_SETCHECK, BST_CHECKED, 0)
	}

	// Position combo
	positions := []string{"top-right", "top-left", "bottom-right", "bottom-left"}
	for i, pos := range positions {
		if pos == s.config.Overlay.Position {
			procSendMessageW.Call(s.controls[ID_OVERLAY_POS], CB_SETCURSEL, uintptr(i), 0)
			break
		}
	}

	// Opacity
	opacity := int(s.config.Overlay.Opacity * 100)
	s.setEditText(s.controls[ID_OVERLAY_OPACITY], fmt.Sprintf("%d", opacity))

	// Alerts
	if s.config.Alerts.Enabled {
		procSendMessageW.Call(s.controls[ID_ALERT_ENABLED], BM_SETCHECK, BST_CHECKED, 0)
	}

	s.setEditText(s.controls[ID_CPU_THRESHOLD], fmt.Sprintf("%.0f", s.config.Alerts.CPUThreshold))
	s.setEditText(s.controls[ID_RAM_THRESHOLD], fmt.Sprintf("%.0f", s.config.Alerts.RAMThreshold))
	s.setEditText(s.controls[ID_GPU_THRESHOLD], fmt.Sprintf("%.0f", s.config.Alerts.GPUThreshold))
	s.setEditText(s.controls[ID_DISK_THRESHOLD], fmt.Sprintf("%.0f", s.config.Alerts.DiskThreshold))

	// Autostart
	if s.config.UI.Autostart {
		procSendMessageW.Call(s.controls[ID_AUTOSTART], BM_SETCHECK, BST_CHECKED, 0)
	}
}

func (s *SettingsWindow) saveSettings() {
	// Overlay enabled
	checked, _, _ := procSendMessageW.Call(s.controls[ID_OVERLAY_ENABLED], BM_GETCHECK, 0, 0)
	s.config.Overlay.Enabled = checked == BST_CHECKED

	// Position
	sel, _, _ := procSendMessageW.Call(s.controls[ID_OVERLAY_POS], CB_GETCURSEL, 0, 0)
	positions := []string{"top-right", "top-left", "bottom-right", "bottom-left"}
	if int(sel) >= 0 && int(sel) < len(positions) {
		s.config.Overlay.Position = positions[sel]
	}

	// Opacity
	if opacity, err := strconv.Atoi(s.getEditText(s.controls[ID_OVERLAY_OPACITY])); err == nil {
		if opacity >= 0 && opacity <= 100 {
			s.config.Overlay.Opacity = float64(opacity) / 100.0
		}
	}

	// Alerts enabled
	checked, _, _ = procSendMessageW.Call(s.controls[ID_ALERT_ENABLED], BM_GETCHECK, 0, 0)
	s.config.Alerts.Enabled = checked == BST_CHECKED

	// Thresholds
	if v, err := strconv.ParseFloat(s.getEditText(s.controls[ID_CPU_THRESHOLD]), 64); err == nil {
		s.config.Alerts.CPUThreshold = v
	}
	if v, err := strconv.ParseFloat(s.getEditText(s.controls[ID_RAM_THRESHOLD]), 64); err == nil {
		s.config.Alerts.RAMThreshold = v
	}
	if v, err := strconv.ParseFloat(s.getEditText(s.controls[ID_GPU_THRESHOLD]), 64); err == nil {
		s.config.Alerts.GPUThreshold = v
	}
	if v, err := strconv.ParseFloat(s.getEditText(s.controls[ID_DISK_THRESHOLD]), 64); err == nil {
		s.config.Alerts.DiskThreshold = v
	}

	// Autostart
	checked, _, _ = procSendMessageW.Call(s.controls[ID_AUTOSTART], BM_GETCHECK, 0, 0)
	s.config.UI.Autostart = checked == BST_CHECKED

	// Save to file
	if s.configMgr != nil {
		s.configMgr.Save()
	}

	// Apply overlay changes immediately
	if s.onOverlayToggle != nil {
		s.onOverlayToggle(s.config.Overlay.Enabled)
	}

	// Call general apply callback
	if s.onApply != nil {
		s.onApply()
	}
}

func (s *SettingsWindow) setEditText(hwnd uintptr, text string) {
	textPtr, _ := syscall.UTF16PtrFromString(text)
	procSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(textPtr)))
}

func (s *SettingsWindow) getEditText(hwnd uintptr) string {
	length, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if length == 0 {
		return ""
	}

	buf := make([]uint16, length+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), length+1)
	return syscall.UTF16ToString(buf)
}

func (s *SettingsWindow) showMessage(title, text string) {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	textPtr, _ := syscall.UTF16PtrFromString(text)
	procMessageBoxW.Call(s.hwnd, uintptr(unsafe.Pointer(textPtr)), uintptr(unsafe.Pointer(titlePtr)), MB_OK|MB_ICONINFO)
}

func (s *SettingsWindow) close() {
	s.running = false
	if s.hwnd != 0 {
		procDestroyWindow.Call(s.hwnd)
		s.hwnd = 0
	}
}

func settingsWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_COMMAND:
		id := int(wParam & 0xFFFF)

		switch id {
		case ID_OK:
			if globalSettings != nil {
				globalSettings.saveSettings()
				globalSettings.showMessage("Настройки", "Настройки сохранены!")
				globalSettings.close()
			}
			return 0

		case ID_CANCEL:
			if globalSettings != nil {
				globalSettings.close()
			}
			return 0

		case ID_APPLY:
			if globalSettings != nil {
				globalSettings.saveSettings()
				globalSettings.showMessage("Настройки", "Настройки применены!")
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
