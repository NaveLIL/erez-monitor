// Package ui provides user interface components for EREZMonitor.
//go:build windows

package ui

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"

	"github.com/NaveLIL/erez-monitor/collector"
	"github.com/NaveLIL/erez-monitor/config"
	"github.com/NaveLIL/erez-monitor/models"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procCreateWindowExW         = user32.NewProc("CreateWindowExW")
	procDefWindowProcW          = user32.NewProc("DefWindowProcW")
	procRegisterClassExW        = user32.NewProc("RegisterClassExW")
	procShowWindow              = user32.NewProc("ShowWindow")
	procUpdateWindow            = user32.NewProc("UpdateWindow")
	procGetMessageW             = user32.NewProc("GetMessageW")
	procTranslateMessage        = user32.NewProc("TranslateMessage")
	procDispatchMessageW        = user32.NewProc("DispatchMessageW")
	procPostQuitMessage         = user32.NewProc("PostQuitMessage")
	procDestroyWindow           = user32.NewProc("DestroyWindow")
	procSetLayeredWindowAttribs = user32.NewProc("SetLayeredWindowAttributes")
	procInvalidateRect          = user32.NewProc("InvalidateRect")
	procRedrawWindow            = user32.NewProc("RedrawWindow")
	procBeginPaint              = user32.NewProc("BeginPaint")
	procEndPaint                = user32.NewProc("EndPaint")
	procFillRect                = user32.NewProc("FillRect")
	procSetBkMode               = gdi32.NewProc("SetBkMode")
	procSetTextColor            = gdi32.NewProc("SetTextColor")
	procCreateFontW             = gdi32.NewProc("CreateFontW")
	procSelectObject            = gdi32.NewProc("SelectObject")
	procDeleteObject            = gdi32.NewProc("DeleteObject")
	procTextOutW                = gdi32.NewProc("TextOutW")
	procCreateSolidBrush        = gdi32.NewProc("CreateSolidBrush")
	procGetModuleHandleW        = kernel32.NewProc("GetModuleHandleW")
	procPostMessageW            = user32.NewProc("PostMessageW")
	procSetTimer                = user32.NewProc("SetTimer")
	procKillTimer               = user32.NewProc("KillTimer")
	procGetSystemMetrics        = user32.NewProc("GetSystemMetrics")
	procGetAsyncKeyState        = user32.NewProc("GetAsyncKeyState")
	procGetWindowLongW          = user32.NewProc("GetWindowLongW")
	procSetWindowLongW          = user32.NewProc("SetWindowLongW")
	procGetWindowRect           = user32.NewProc("GetWindowRect")
)

// Virtual key codes
const (
	VK_CONTROL = 0x11
)

// Window long index (use uintptr-compatible value for -20)
var (
	GWL_EXSTYLE = uintptr(0xFFFFFFEC) // -20 in unsigned 32-bit
)

// Window style constants
const (
	WS_EX_LAYERED     = 0x00080000
	WS_EX_TRANSPARENT = 0x00000020
	WS_EX_TOPMOST     = 0x00000008
	WS_EX_TOOLWINDOW  = 0x00000080
	WS_EX_NOACTIVATE  = 0x08000000
	WS_POPUP          = 0x80000000

	SW_SHOW = 5
	SW_HIDE = 0

	LWA_ALPHA = 0x00000002

	WM_DESTROY     = 0x0002
	WM_PAINT       = 0x000F
	WM_TIMER       = 0x0113
	WM_CLOSE       = 0x0010
	WM_NCHITTEST   = 0x0084
	WM_LBUTTONDOWN = 0x0201
	WM_MOUSEMOVE   = 0x0200
	WM_LBUTTONUP   = 0x0202

	HTCAPTION     = 2
	HTTRANSPARENT = -1

	MK_CONTROL = 0x0008

	TRANSPARENT = 1
	CS_HREDRAW  = 0x0002
	CS_VREDRAW  = 0x0001

	// RedrawWindow flags
	RDW_INVALIDATE = 0x0001
	RDW_UPDATENOW  = 0x0100
	RDW_ERASE      = 0x0004
	RDW_FRAME      = 0x0400
)

// Color constants (BGR format)
const (
	COLOR_BG_DARK    = 0x1E1E1E // Darker background
	COLOR_BG_LIGHT   = 0x2D2D2D // Lighter background for bars
	COLOR_BG_BAR     = 0x3D3D3D // Progress bar background
	COLOR_TEXT_WHITE = 0xF0F0F0 // Bright white text
	COLOR_TEXT_GRAY  = 0x909090 // Gray text for labels
	COLOR_GREEN      = 0x50C878 // Emerald green (good)
	COLOR_YELLOW     = 0x00D4FF // Gold/yellow (warning)
	COLOR_ORANGE     = 0x0099FF // Orange (high)
	COLOR_RED        = 0x5050FF // Red (critical)
	COLOR_CYAN       = 0xFFDD44 // Cyan for network
	COLOR_PURPLE     = 0xDD77FF // Purple for disk
	COLOR_BLUE       = 0xFF9944 // Blue accent
	COLOR_ACCENT     = 0xFF7744 // Main accent color
)

// WNDCLASSEXW represents the WNDCLASSEXW structure.
type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

// PAINTSTRUCT represents the PAINTSTRUCT structure.
type PAINTSTRUCT struct {
	HDC         uintptr
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

// RECT represents a rectangle.
type RECT struct {
	Left, Top, Right, Bottom int32
}

// OverlayMSG represents a Windows message.
type OverlayMSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

// Overlay represents a transparent overlay window.
type Overlay struct {
	config    *config.OverlayConfig
	collector *collector.Collector

	hwnd      uintptr
	hInstance uintptr
	className *uint16
	fontLarge uintptr
	fontSmall uintptr

	visible  bool
	running  bool
	dragMode bool // When true, overlay can be dragged
	mu       sync.RWMutex

	currentMetrics *models.Metrics
	metricsMu      sync.RWMutex

	stopCh chan struct{}

	width  int32
	height int32

	// Callback for position changes (called when drag mode ends)
	onPositionChanged func(x, y int)
}

// Global overlay instance for window proc callback
var globalOverlay *Overlay

// NewOverlay creates a new overlay window.
func NewOverlay(cfg *config.OverlayConfig, coll *collector.Collector) *Overlay {
	return &Overlay{
		config:    cfg,
		collector: coll,
		stopCh:    make(chan struct{}),
		width:     240,
		height:    195,
	}
}

// Start starts the overlay window in a separate goroutine.
func (o *Overlay) Start() error {
	o.mu.Lock()
	if o.running {
		o.mu.Unlock()
		return nil
	}
	o.running = true
	o.mu.Unlock()

	globalOverlay = o

	go o.run()
	return nil
}

// Stop stops the overlay window.
func (o *Overlay) Stop() {
	o.mu.Lock()
	if !o.running {
		o.mu.Unlock()
		return
	}
	o.running = false
	o.mu.Unlock()

	close(o.stopCh)

	if o.hwnd != 0 {
		procPostMessageW.Call(o.hwnd, WM_CLOSE, 0, 0)
	}
}

// Toggle toggles the overlay visibility.
func (o *Overlay) Toggle() {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.visible = !o.visible
	if o.hwnd != 0 {
		if o.visible {
			procShowWindow.Call(o.hwnd, SW_SHOW)
		} else {
			procShowWindow.Call(o.hwnd, SW_HIDE)
		}
	}
}

// Show shows the overlay.
func (o *Overlay) Show() {
	o.mu.Lock()
	o.visible = true
	hwnd := o.hwnd
	opacity := o.config.Opacity
	o.mu.Unlock()

	if hwnd != 0 {
		// Re-apply layered window style first
		style, _, _ := procGetWindowLongW.Call(hwnd, GWL_EXSTYLE)
		style = style | WS_EX_LAYERED
		procSetWindowLongW.Call(hwnd, GWL_EXSTYLE, style)

		// Re-apply opacity
		alpha := byte(255 * opacity)
		if alpha < 80 {
			alpha = 80
		}
		if alpha > 220 {
			alpha = 220
		}
		procSetLayeredWindowAttribs.Call(hwnd, 0, uintptr(alpha), LWA_ALPHA)

		procShowWindow.Call(hwnd, SW_SHOW)

		// Force immediate full repaint
		procRedrawWindow.Call(hwnd, 0, 0, RDW_INVALIDATE|RDW_UPDATENOW|RDW_ERASE|RDW_FRAME)

		// Also trigger a timer message to force paint
		procPostMessageW.Call(hwnd, WM_TIMER, 1, 0)
	}
}

// Hide hides the overlay.
func (o *Overlay) Hide() {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.visible = false
	if o.hwnd != 0 {
		procShowWindow.Call(o.hwnd, SW_HIDE)
	}
}

// ToggleDragMode toggles the drag mode for repositioning overlay.
func (o *Overlay) ToggleDragMode() bool {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.dragMode = !o.dragMode

	if o.hwnd != 0 {
		// Get current extended style
		style, _, _ := procGetWindowLongW.Call(o.hwnd, GWL_EXSTYLE)

		if o.dragMode {
			// Remove WS_EX_TRANSPARENT to allow mouse interaction
			style = style &^ WS_EX_TRANSPARENT
		} else {
			// Add WS_EX_TRANSPARENT to make click-through
			style = style | WS_EX_TRANSPARENT
		}

		procSetWindowLongW.Call(o.hwnd, GWL_EXSTYLE, style)
	}

	// If turning off drag mode, save position
	if !o.dragMode && o.onPositionChanged != nil && o.hwnd != 0 {
		x, y := o.getWindowPosition()
		go o.onPositionChanged(x, y)
	}

	return o.dragMode
}

// IsDragMode returns whether drag mode is active.
func (o *Overlay) IsDragMode() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.dragMode
}

// SetOnPositionChanged sets the callback for position changes.
func (o *Overlay) SetOnPositionChanged(callback func(x, y int)) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.onPositionChanged = callback
}

// getWindowPosition returns the current window position.
func (o *Overlay) getWindowPosition() (int, int) {
	if o.hwnd == 0 {
		return 0, 0
	}
	var rect RECT
	procGetWindowRect.Call(o.hwnd, uintptr(unsafe.Pointer(&rect)))
	return int(rect.Left), int(rect.Top)
}

// GetPosition returns the current overlay position.
func (o *Overlay) GetPosition() (int, int) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.getWindowPosition()
}

// IsVisible returns whether the overlay is visible.
func (o *Overlay) IsVisible() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.visible
}

// run runs the overlay window message loop.
func (o *Overlay) run() {
	o.hInstance, _, _ = procGetModuleHandleW.Call(0)

	className, _ := syscall.UTF16PtrFromString("EREZMonitorOverlayV2")
	o.className = className

	wc := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		Style:         CS_HREDRAW | CS_VREDRAW,
		LpfnWndProc:   syscall.NewCallback(overlayWndProc),
		HInstance:     o.hInstance,
		HbrBackground: 0,
		LpszClassName: className,
	}

	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	// Get screen dimensions
	screenWidth, _, _ := procGetSystemMetrics.Call(0)
	screenHeight, _, _ := procGetSystemMetrics.Call(1)

	padding := int32(15)
	var x, y int32

	switch o.config.Position {
	case "custom":
		// Use custom coordinates from config
		x, y = int32(o.config.CustomX), int32(o.config.CustomY)
	case "top-left":
		x, y = padding, padding
	case "bottom-left":
		x, y = padding, int32(screenHeight)-o.height-padding-50
	case "bottom-right":
		x, y = int32(screenWidth)-o.width-padding, int32(screenHeight)-o.height-padding-50
	default: // top-right
		x, y = int32(screenWidth)-o.width-padding, padding
	}

	windowName, _ := syscall.UTF16PtrFromString("EREZMonitor Overlay")

	// WS_EX_TRANSPARENT makes overlay click-through (doesn't intercept mouse)
	exStyle := uintptr(WS_EX_LAYERED | WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE | WS_EX_TRANSPARENT)
	style := uintptr(WS_POPUP)

	hwnd, _, _ := procCreateWindowExW.Call(
		exStyle,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		style,
		uintptr(x), uintptr(y),
		uintptr(o.width), uintptr(o.height),
		0, 0, o.hInstance, 0,
	)

	if hwnd == 0 {
		return
	}

	o.hwnd = hwnd

	alpha := byte(255 * o.config.Opacity)
	if alpha < 80 {
		alpha = 80 // More transparent minimum
	}
	if alpha > 220 {
		alpha = 220 // Keep some transparency
	}
	procSetLayeredWindowAttribs.Call(hwnd, 0, uintptr(alpha), LWA_ALPHA)

	// Create fonts
	fontName, _ := syscall.UTF16PtrFromString("Consolas")

	o.fontLarge, _, _ = procCreateFontW.Call(
		uintptr(uint32(0xFFFFFFEA)), // -22 height
		0, 0, 0, 700, 0, 0, 0, 0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(fontName)),
	)

	o.fontSmall, _, _ = procCreateFontW.Call(
		uintptr(uint32(0xFFFFFFF2)), // -14 height
		0, 0, 0, 400, 0, 0, 0, 0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(fontName)),
	)

	if o.config.Enabled {
		o.visible = true
		procShowWindow.Call(hwnd, SW_SHOW)
		procUpdateWindow.Call(hwnd)
	}

	procSetTimer.Call(hwnd, 1, 500, 0)

	// Don't use channel subscription - just poll directly in WM_TIMER
	// This avoids any blocking issues with channels

	var msg OverlayMSG
	for {
		ret, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
			0, 0, 0,
		)

		if ret == 0 || int32(ret) == -1 {
			break
		}

		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	if o.fontLarge != 0 {
		procDeleteObject.Call(o.fontLarge)
	}
	if o.fontSmall != 0 {
		procDeleteObject.Call(o.fontSmall)
	}
	procKillTimer.Call(o.hwnd, 1)
}

func overlayWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_PAINT:
		if globalOverlay != nil {
			globalOverlay.paint(hwnd)
		}
		return 0

	case WM_TIMER:
		// Timer fires every 500ms - just repaint, metrics fetched in paint()
		if globalOverlay != nil && globalOverlay.hwnd != 0 {
			globalOverlay.mu.RLock()
			visible := globalOverlay.visible
			globalOverlay.mu.RUnlock()
			if visible {
				procInvalidateRect.Call(hwnd, 0, 1)
			}
		}
		return 0

	case WM_NCHITTEST:
		// Only allow dragging in drag mode
		if globalOverlay != nil && globalOverlay.dragMode {
			ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
			if ret == 1 { // HTCLIENT
				return HTCAPTION
			}
			return ret
		}
		// Not in drag mode - pass through
		break

	case WM_DESTROY:
		procPostQuitMessage.Call(0)
		return 0

	case WM_CLOSE:
		procDestroyWindow.Call(hwnd)
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return ret
}

func getValueColor(percent float64) uintptr {
	if percent < 50 {
		return COLOR_GREEN
	} else if percent < 70 {
		return COLOR_YELLOW
	} else if percent < 85 {
		return COLOR_ORANGE
	}
	return COLOR_RED
}

func getTempColor(temp uint32) uintptr {
	if temp < 50 {
		return COLOR_GREEN
	} else if temp < 70 {
		return COLOR_YELLOW
	} else if temp < 85 {
		return COLOR_ORANGE
	}
	return COLOR_RED
}

func getPingColor(pingMs float64) uintptr {
	if pingMs < 30 {
		return COLOR_GREEN
	} else if pingMs < 60 {
		return COLOR_YELLOW
	} else if pingMs < 100 {
		return COLOR_ORANGE
	}
	return COLOR_RED
}

func (o *Overlay) paint(hwnd uintptr) {
	var ps PAINTSTRUCT
	hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))

	if hdc == 0 {
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return
	}

	// Get metrics directly from collector - no locks
	var metrics *models.Metrics
	if o.collector != nil {
		metrics = o.collector.GetLatest()
	}

	// Main background with rounded effect (dark edges)
	bgBrush, _, _ := procCreateSolidBrush.Call(COLOR_BG_DARK)
	rect := RECT{Left: 0, Top: 0, Right: o.width, Bottom: o.height}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rect)), bgBrush)
	procDeleteObject.Call(bgBrush)

	// Left accent bar - changes color based on mode
	o.mu.RLock()
	isDragMode := o.dragMode
	o.mu.RUnlock()

	accentColor := uintptr(COLOR_ACCENT)
	if isDragMode {
		accentColor = uintptr(COLOR_ORANGE)
	}
	accentBrush, _, _ := procCreateSolidBrush.Call(accentColor)
	accentRect := RECT{Left: 0, Top: 0, Right: 4, Bottom: o.height}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&accentRect)), accentBrush)
	procDeleteObject.Call(accentBrush)

	// If in drag mode, draw border
	if isDragMode {
		borderBrush, _, _ := procCreateSolidBrush.Call(COLOR_ORANGE)
		topRect := RECT{Left: 0, Top: 0, Right: o.width, Bottom: 2}
		procFillRect.Call(hdc, uintptr(unsafe.Pointer(&topRect)), borderBrush)
		bottomRect := RECT{Left: 0, Top: o.height - 2, Right: o.width, Bottom: o.height}
		procFillRect.Call(hdc, uintptr(unsafe.Pointer(&bottomRect)), borderBrush)
		rightRect := RECT{Left: o.width - 2, Top: 0, Right: o.width, Bottom: o.height}
		procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rightRect)), borderBrush)
		procDeleteObject.Call(borderBrush)
	}

	procSetBkMode.Call(hdc, TRANSPARENT)

	// Layout constants
	y := int32(10)
	rowHeight := int32(28)
	labelX := int32(12)
	barX := int32(52)
	barWidth := int32(120)
	barHeight := int32(8)
	valueX := int32(180)

	if metrics != nil {
		// === CPU with progress bar ===
		if o.config.ShowCPU {
			o.drawMetricRow(hdc, "CPU", metrics.CPU.UsagePercent, y, labelX, barX, barWidth, barHeight, valueX)
			y += rowHeight
		}

		// === RAM with progress bar ===
		if o.config.ShowRAM {
			o.drawMetricRow(hdc, "RAM", metrics.Memory.UsedPercent, y, labelX, barX, barWidth, barHeight, valueX)
			// Draw memory info below bar
			procSelectObject.Call(hdc, o.fontSmall)
			procSetTextColor.Call(hdc, COLOR_TEXT_GRAY)
			memText := fmt.Sprintf("%dG / %dG", metrics.Memory.UsedMB/1024, metrics.Memory.TotalMB/1024)
			o.drawText(hdc, memText, barX, y+12)
			y += rowHeight + 4
		}

		// === GPU with progress bar ===
		if o.config.ShowGPU && metrics.GPU.Available {
			o.drawMetricRow(hdc, "GPU", metrics.GPU.UsagePercent, y, labelX, barX, barWidth, barHeight, valueX)
			// Draw VRAM and temperature info below bar
			procSelectObject.Call(hdc, o.fontSmall)
			procSetTextColor.Call(hdc, COLOR_TEXT_GRAY)
			// Shorter VRAM format
			vramGB := float64(metrics.GPU.VRAMUsedMB) / 1024.0
			totalGB := float64(metrics.GPU.VRAMTotalMB) / 1024.0
			vramText := fmt.Sprintf("%.1fG/%.0fG", vramGB, totalGB)
			o.drawText(hdc, vramText, barX, y+12)
			// Show temperature if available
			if metrics.GPU.TemperatureC > 0 {
				procSetTextColor.Call(hdc, getTempColor(metrics.GPU.TemperatureC))
				tempText := fmt.Sprintf("%d°C", metrics.GPU.TemperatureC)
				o.drawText(hdc, tempText, barX+75, y+12)
			}
			y += rowHeight + 4
		}

		// Separator line
		if o.config.ShowNet || o.config.ShowDisk {
			y += 2
			sepBrush, _, _ := procCreateSolidBrush.Call(COLOR_BG_BAR)
			sepRect := RECT{Left: 12, Top: y, Right: o.width - 12, Bottom: y + 1}
			procFillRect.Call(hdc, uintptr(unsafe.Pointer(&sepRect)), sepBrush)
			procDeleteObject.Call(sepBrush)
			y += 8
		}

		// === Network ===
		if o.config.ShowNet {
			procSelectObject.Call(hdc, o.fontSmall)
			procSetTextColor.Call(hdc, COLOR_TEXT_GRAY)
			o.drawText(hdc, "NET", labelX, y)

			// Download
			procSetTextColor.Call(hdc, COLOR_CYAN)
			var dlText string
			if metrics.Network.DownloadKBps >= 1024 {
				dlText = fmt.Sprintf("↓%.1f MB/s", metrics.Network.DownloadKBps/1024)
			} else {
				dlText = fmt.Sprintf("↓%.0f KB/s", metrics.Network.DownloadKBps)
			}
			o.drawText(hdc, dlText, barX, y)

			// Upload
			var ulText string
			if metrics.Network.UploadKBps >= 1024 {
				ulText = fmt.Sprintf("↑%.1f MB/s", metrics.Network.UploadKBps/1024)
			} else {
				ulText = fmt.Sprintf("↑%.0f KB/s", metrics.Network.UploadKBps)
			}
			o.drawText(hdc, ulText, barX+85, y)
			y += 18

			// === Ping ===
			if metrics.Network.PingMs > 0 {
				procSetTextColor.Call(hdc, COLOR_TEXT_GRAY)
				o.drawText(hdc, "PING", labelX, y)

				procSetTextColor.Call(hdc, getPingColor(metrics.Network.PingMs))
				pingText := fmt.Sprintf("%.0f ms", metrics.Network.PingMs)
				o.drawText(hdc, pingText, barX, y)

				procSetTextColor.Call(hdc, COLOR_TEXT_GRAY)
				o.drawText(hdc, metrics.Network.PingTarget, barX+55, y)
				y += 18
			}
		}

		// === Disk I/O (only if active) ===
		if o.config.ShowDisk && (metrics.Disk.ReadMBps > 0.05 || metrics.Disk.WriteMBps > 0.05) {
			procSelectObject.Call(hdc, o.fontSmall)
			procSetTextColor.Call(hdc, COLOR_TEXT_GRAY)
			o.drawText(hdc, "DISK", labelX, y)

			procSetTextColor.Call(hdc, COLOR_PURPLE)
			diskText := fmt.Sprintf("R:%.1f  W:%.1f MB/s", metrics.Disk.ReadMBps, metrics.Disk.WriteMBps)
			o.drawText(hdc, diskText, barX, y)
		}

	} else {
		procSelectObject.Call(hdc, o.fontLarge)
		procSetTextColor.Call(hdc, COLOR_TEXT_WHITE)
		o.drawText(hdc, "Loading...", 12, 80)
	}

	procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
}

// drawMetricRow draws a metric row with label, progress bar, and value
func (o *Overlay) drawMetricRow(hdc uintptr, label string, percent float64, y, labelX, barX, barWidth, barHeight, valueX int32) {
	// Draw label
	procSelectObject.Call(hdc, o.fontSmall)
	procSetTextColor.Call(hdc, COLOR_TEXT_GRAY)
	o.drawText(hdc, label, labelX, y)

	// Draw progress bar background
	barY := y + 2
	bgBrush, _, _ := procCreateSolidBrush.Call(COLOR_BG_BAR)
	bgRect := RECT{Left: barX, Top: barY, Right: barX + barWidth, Bottom: barY + barHeight}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&bgRect)), bgBrush)
	procDeleteObject.Call(bgBrush)

	// Draw progress bar fill - always show at least a small amount if > 0
	if percent > 0.5 {
		fillWidth := int32(float64(barWidth) * percent / 100.0)
		if fillWidth < 4 {
			fillWidth = 4 // Minimum visible width
		}
		if fillWidth > barWidth {
			fillWidth = barWidth
		}

		fillColor := getValueColor(percent)
		fillBrush, _, _ := procCreateSolidBrush.Call(fillColor)
		fillRect := RECT{Left: barX, Top: barY, Right: barX + fillWidth, Bottom: barY + barHeight}
		procFillRect.Call(hdc, uintptr(unsafe.Pointer(&fillRect)), fillBrush)
		procFillRect.Call(hdc, uintptr(unsafe.Pointer(&fillRect)), fillBrush)
		procDeleteObject.Call(fillBrush)
	}

	// Draw value text
	procSelectObject.Call(hdc, o.fontLarge)
	procSetTextColor.Call(hdc, getValueColor(percent))
	valueText := fmt.Sprintf("%.0f%%", percent)
	o.drawText(hdc, valueText, valueX, y-2)
}

func (o *Overlay) drawText(hdc uintptr, text string, x, y int32) {
	textW, _ := syscall.UTF16FromString(text)
	procTextOutW.Call(hdc, uintptr(x), uintptr(y), uintptr(unsafe.Pointer(&textW[0])), uintptr(len(textW)-1))
}
