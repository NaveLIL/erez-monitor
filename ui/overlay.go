// Package ui provides user interface components for EREZMonitor.
//go:build windows

package ui

import (
	"fmt"
	"math"
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/NaveLIL/erez-monitor/collector"
	"github.com/NaveLIL/erez-monitor/config"
	"github.com/NaveLIL/erez-monitor/logger"
	"github.com/NaveLIL/erez-monitor/models"
)

// Windows DLLs and procedures
var (
	user32                         = syscall.NewLazyDLL("user32.dll")
	gdi32                          = syscall.NewLazyDLL("gdi32.dll")
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procGetModuleHandleW           = kernel32.NewProc("GetModuleHandleW")
	procRegisterClassExW           = user32.NewProc("RegisterClassExW")
	procCreateWindowExW            = user32.NewProc("CreateWindowExW")
	procDefWindowProcW             = user32.NewProc("DefWindowProcW")
	procGetMessageW                = user32.NewProc("GetMessageW")
	procTranslateMessage           = user32.NewProc("TranslateMessage")
	procDispatchMessageW           = user32.NewProc("DispatchMessageW")
	procPostMessageW               = user32.NewProc("PostMessageW")
	procDestroyWindow              = user32.NewProc("DestroyWindow")
	procPostQuitMessage            = user32.NewProc("PostQuitMessage")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procSetWindowPos               = user32.NewProc("SetWindowPos")
	procGetWindowRect              = user32.NewProc("GetWindowRect")
	procInvalidateRect             = user32.NewProc("InvalidateRect")
	procSetTimer                   = user32.NewProc("SetTimer")
	procKillTimer                  = user32.NewProc("KillTimer")
	procBeginPaint                 = user32.NewProc("BeginPaint")
	procEndPaint                   = user32.NewProc("EndPaint")
	procGetDC                      = user32.NewProc("GetDC")
	procReleaseDC                  = user32.NewProc("ReleaseDC")
	procSetCursor                  = user32.NewProc("SetCursor")
	procLoadCursorW                = user32.NewProc("LoadCursorW")
	procReleaseCapture             = user32.NewProc("ReleaseCapture")
	procSetCapture                 = user32.NewProc("SetCapture")
	procGetCursorPos               = user32.NewProc("GetCursorPos")
	procScreenToClient             = user32.NewProc("ScreenToClient")
	procCreateFontW                = gdi32.NewProc("CreateFontW")
	procDeleteObject               = gdi32.NewProc("DeleteObject")
	procSelectObject               = gdi32.NewProc("SelectObject")
	procSetBkMode                  = gdi32.NewProc("SetBkMode")
	procSetTextColor               = gdi32.NewProc("SetTextColor")
	procTextOutW                   = gdi32.NewProc("TextOutW")
	procCreateSolidBrush           = gdi32.NewProc("CreateSolidBrush")
	procFillRect                   = user32.NewProc("FillRect")
	procCreatePen                  = gdi32.NewProc("CreatePen")
	procMoveToEx                   = gdi32.NewProc("MoveToEx")
	procLineTo                     = gdi32.NewProc("LineTo")
	procPolyline                   = gdi32.NewProc("Polyline")
	procRoundRect                  = gdi32.NewProc("RoundRect")
	procGetTextExtentPoint32W      = gdi32.NewProc("GetTextExtentPoint32W")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procGetSystemMetrics           = user32.NewProc("GetSystemMetrics")
	procGetWindowLongW             = user32.NewProc("GetWindowLongW")
	procSetWindowLongW             = user32.NewProc("SetWindowLongW")
	procCreateRoundRectRgn         = gdi32.NewProc("CreateRoundRectRgn")
	procSetWindowRgn               = user32.NewProc("SetWindowRgn")
	procFillRgn                    = gdi32.NewProc("FillRgn")
	procFrameRgn                   = gdi32.NewProc("FrameRgn")
	procRectangle                  = gdi32.NewProc("Rectangle")
)

// Window style constants
const (
	WS_EX_LAYERED     = 0x00080000
	WS_EX_TOPMOST     = 0x00000008
	WS_EX_TOOLWINDOW  = 0x00000080
	WS_EX_TRANSPARENT = 0x00000020
	WS_EX_NOACTIVATE  = 0x08000000
	WS_POPUP          = 0x80000000

	CS_HREDRAW = 0x0002
	CS_VREDRAW = 0x0001

	SW_SHOW = 5
	SW_HIDE = 0

	HWND_TOPMOST = ^uintptr(0) // -1

	SWP_NOMOVE     = 0x0002
	SWP_NOSIZE     = 0x0001
	SWP_NOZORDER   = 0x0004
	SWP_SHOWWINDOW = 0x0040
	SWP_NOACTIVATE = 0x0010

	WM_DESTROY     = 0x0002
	WM_PAINT       = 0x000F
	WM_TIMER       = 0x0113
	WM_LBUTTONDOWN = 0x0201
	WM_LBUTTONUP   = 0x0202
	WM_MOUSEMOVE   = 0x0200
	WM_SETCURSOR   = 0x0020
	WM_NCHITTEST   = 0x0084
	WM_CLOSE       = 0x0010

	SM_CXSCREEN = 0
	SM_CYSCREEN = 1

	LWA_ALPHA    = 0x00000002
	LWA_COLORKEY = 0x00000001

	TRANSPARENT_BG = 1

	IDC_HAND    = 32649
	IDC_SIZEALL = 32646
	IDC_ARROW   = 32512

	FW_NORMAL = 400
	FW_BOLD   = 700

	DEFAULT_CHARSET     = 1
	OUT_DEFAULT_PRECIS  = 0
	CLIP_DEFAULT_PRECIS = 0
	DEFAULT_QUALITY     = 0
	DEFAULT_PITCH       = 0
	FF_DONTCARE         = 0

	PS_SOLID = 0

	TIMER_UPDATE_ID    = 1
	TIMER_ANIMATION_ID = 2
	TIMER_UPDATE_MS    = 500
	TIMER_ANIMATION_MS = 16 // ~60 FPS

	GWL_EXSTYLE = -20
	HTCAPTION   = 2

	// History settings
	HISTORY_SIZE     = 60 // 60 samples = 30 seconds at 500ms interval
	SPARKLINE_WIDTH  = 60
	SPARKLINE_HEIGHT = 16

	// Colors (COLORREF: 0x00BBGGRR)
	COLOR_BG_DARK   = 0x00282828 // Dark gray background
	COLOR_BG_BAR    = 0x00404040 // Bar background
	COLOR_BG_GRAPH  = 0x00353535 // Graph background
	COLOR_TEXT      = 0x00FFFFFF // White text
	COLOR_TEXT_GRAY = 0x00AAAAAA // Gray text
	COLOR_ACCENT    = 0x00FF9900 // Orange accent (for CPU)
	COLOR_CPU       = 0x0000AAFF // Orange for CPU graph
	COLOR_RAM       = 0x0000DD00 // Green for RAM graph
	COLOR_GPU_GRAPH = 0x00FFAA00 // Blue for GPU graph
	COLOR_GREEN     = 0x0000AA00 // Green (for RAM)
	COLOR_BLUE      = 0x00FF6600 // Blue (for GPU)
	COLOR_ORANGE    = 0x000080FF // Orange warning
	COLOR_RED       = 0x000000FF // Red critical
	COLOR_CYAN      = 0x00FFFF00 // Cyan (for network)
	COLOR_PURPLE    = 0x00FF00FF // Purple (for disk)
	COLOR_GLOW      = 0x00FF9900 // Glow/border color
	COLOR_BORDER    = 0x00505050 // Subtle border
	TRANSPARENT     = 1          // Transparent background mode

	// Corner radius for rounded window
	CORNER_RADIUS = 12
)

// RECT structure for Windows API
type RECT struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

// POINT structure for Windows API
type POINT struct {
	X int32
	Y int32
}

// SIZE structure for Windows API
type SIZE struct {
	Cx int32
	Cy int32
}

// MSG structure for Windows messages
type MSG struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

// WNDCLASSEXW structure for window class registration
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

// PAINTSTRUCT for WM_PAINT
type PAINTSTRUCT struct {
	Hdc         uintptr
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

// lerp performs linear interpolation between two values.
func lerp(current, target, factor float64) float64 {
	return current + (target-current)*factor
}

// getTempColor returns color based on GPU temperature (uint32).
func getTempColor(temp uint32) uintptr {
	if temp >= 80 {
		return COLOR_RED
	} else if temp >= 65 {
		return COLOR_ORANGE
	}
	return COLOR_GREEN
}

// getCPUTempColor returns color based on CPU temperature (float64).
func getCPUTempColor(temp float64) uintptr {
	if temp >= 85 {
		return COLOR_RED
	} else if temp >= 70 {
		return COLOR_ORANGE
	} else if temp >= 55 {
		return COLOR_GREEN
	}
	return COLOR_CYAN // Cool temperature
}

// getPingColor returns color based on ping latency.
func getPingColor(pingMs float64) uintptr {
	if pingMs >= 100 {
		return COLOR_RED
	} else if pingMs >= 50 {
		return COLOR_ORANGE
	}
	return COLOR_GREEN
}

// getValueColor returns color based on percentage value.
func getValueColor(percent float64) uintptr {
	if percent >= 90 {
		return COLOR_RED
	} else if percent >= 75 {
		return COLOR_ORANGE
	}
	return COLOR_GREEN
}

// blendColors blends two colors based on factor (0.0 to 1.0).
func blendColors(color1, color2 uintptr, factor float64) uintptr {
	r1 := int(color1 & 0xFF)
	g1 := int((color1 >> 8) & 0xFF)
	b1 := int((color1 >> 16) & 0xFF)

	r2 := int(color2 & 0xFF)
	g2 := int((color2 >> 8) & 0xFF)
	b2 := int((color2 >> 16) & 0xFF)

	r := int(float64(r1) + (float64(r2)-float64(r1))*factor)
	g := int(float64(g1) + (float64(g2)-float64(g1))*factor)
	b := int(float64(b1) + (float64(b2)-float64(b1))*factor)

	return uintptr(r | (g << 8) | (b << 16))
}

// addHistorySample adds new values to history buffer
func (o *Overlay) addHistorySample(cpu, ram, gpu float64) {
	o.history.cpu[o.history.index] = cpu
	o.history.ram[o.history.index] = ram
	o.history.gpu[o.history.index] = gpu
	o.history.index = (o.history.index + 1) % HISTORY_SIZE
	if o.history.count < HISTORY_SIZE {
		o.history.count++
	}
}

// drawSparkline draws a mini line graph for the given history
func (o *Overlay) drawSparkline(hdc uintptr, data *[HISTORY_SIZE]float64, x, y, width, height int32, color uintptr) {
	if o.history.count < 2 {
		return
	}

	// Draw background
	bgBrush, _, _ := procCreateSolidBrush.Call(COLOR_BG_GRAPH)
	rect := RECT{Left: x, Top: y, Right: x + width, Bottom: y + height}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rect)), bgBrush)
	procDeleteObject.Call(bgBrush)

	// Create pen for the line
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, color)
	oldPen, _, _ := procSelectObject.Call(hdc, pen)

	// Calculate points
	count := o.history.count
	if count > int(width) {
		count = int(width)
	}

	stepX := float64(width-2) / float64(count-1)
	startIdx := (o.history.index - count + HISTORY_SIZE) % HISTORY_SIZE

	// Draw the line
	for i := 0; i < count; i++ {
		idx := (startIdx + i) % HISTORY_SIZE
		value := data[idx]
		if value > 100 {
			value = 100
		}

		px := x + 1 + int32(float64(i)*stepX)
		py := y + height - 2 - int32((value/100.0)*float64(height-4))

		if i == 0 {
			procMoveToEx.Call(hdc, uintptr(px), uintptr(py), 0)
		} else {
			procLineTo.Call(hdc, uintptr(px), uintptr(py))
		}
	}

	procSelectObject.Call(hdc, oldPen)
	procDeleteObject.Call(pen)
}

// Custom window messages for inter-thread communication
const (
	WM_APP                   = 0x8000
	WM_OVERLAY_SHOW          = WM_APP + 1
	WM_OVERLAY_HIDE          = WM_APP + 2
	WM_OVERLAY_TOGGLE        = WM_APP + 3
	WM_OVERLAY_TOGGLE_DRAG   = WM_APP + 4
	WM_OVERLAY_STOP          = WM_APP + 5
	WM_OVERLAY_SET_OPACITY   = WM_APP + 6
	WM_OVERLAY_UPDATE_POS    = WM_APP + 7
	WM_OVERLAY_UPDATE_CONFIG = WM_APP + 8
)

// Overlay represents a transparent overlay window with proper thread safety.
// All WinAPI calls happen ONLY in the UI thread via PostMessage.
type Overlay struct {
	config    *config.OverlayConfig
	collector *collector.Collector
	log       *logger.Logger

	// Window handles - only accessed from UI thread
	hwnd      uintptr
	hInstance uintptr
	fontLarge uintptr
	fontSmall uintptr

	// State - atomic for thread-safe access
	visible  atomic.Bool
	running  atomic.Bool
	dragMode atomic.Bool

	// Animation state - only accessed from UI thread
	anim animState

	// History for sparklines - only accessed from UI thread
	history historyData

	// Dimensions
	width  int32
	height int32

	// Callback for position changes
	onPositionChanged func(x, y int)

	// Channel to signal when UI thread is ready
	readyCh chan struct{}
}

type animState struct {
	cpuPercent  float64
	ramPercent  float64
	gpuPercent  float64
	pulsePhase  float64
	cpuCritical bool
	ramCritical bool
	gpuCritical bool
}

// historyData stores historical values for sparkline graphs
type historyData struct {
	cpu    [HISTORY_SIZE]float64
	ram    [HISTORY_SIZE]float64
	gpu    [HISTORY_SIZE]float64
	index  int
	count  int
	ticker int // counts animation frames to add new sample
}

// Global instance - ONLY used from UI thread in WndProc
var globalOverlay *Overlay

// NewOverlay creates a new overlay window.
func NewOverlay(cfg *config.OverlayConfig, coll *collector.Collector) *Overlay {
	return &Overlay{
		config:    cfg,
		collector: coll,
		log:       logger.Get(),
		width:     240,
		height:    195, // Back to normal size
		readyCh:   make(chan struct{}),
	}
}

// Start starts the overlay window in a dedicated UI thread.
func (o *Overlay) Start() error {
	if o.running.Load() {
		return nil
	}
	o.running.Store(true)

	// Start UI thread
	go o.uiThread()

	// Wait for UI thread to be ready
	<-o.readyCh

	o.log.Info("Overlay started")
	return nil
}

// Stop stops the overlay. Safe to call from any goroutine.
func (o *Overlay) Stop() {
	if !o.running.Load() {
		return
	}
	o.running.Store(false)

	// Send stop command to UI thread
	if o.hwnd != 0 {
		procPostMessageW.Call(o.hwnd, WM_OVERLAY_STOP, 0, 0)
	}
	o.log.Info("Overlay stop requested")
}

// Show shows the overlay. Safe to call from any goroutine.
func (o *Overlay) Show() {
	o.log.Debug("Overlay Show() called")
	if o.hwnd != 0 {
		procPostMessageW.Call(o.hwnd, WM_OVERLAY_SHOW, 0, 0)
	}
}

// Hide hides the overlay. Safe to call from any goroutine.
func (o *Overlay) Hide() {
	o.log.Debug("Overlay Hide() called")
	if o.hwnd != 0 {
		procPostMessageW.Call(o.hwnd, WM_OVERLAY_HIDE, 0, 0)
	}
}

// Toggle toggles the overlay visibility. Safe to call from any goroutine.
func (o *Overlay) Toggle() {
	o.log.Debug("Overlay Toggle() called")
	if o.hwnd != 0 {
		procPostMessageW.Call(o.hwnd, WM_OVERLAY_TOGGLE, 0, 0)
	}
}

// ToggleDragMode toggles drag mode. Safe to call from any goroutine.
func (o *Overlay) ToggleDragMode() bool {
	o.log.Debug("Overlay ToggleDragMode() called")
	if o.hwnd != 0 {
		procPostMessageW.Call(o.hwnd, WM_OVERLAY_TOGGLE_DRAG, 0, 0)
	}
	// Return current state (may be slightly stale, but that's OK for UI feedback)
	return o.dragMode.Load()
}

// IsVisible returns whether the overlay is visible.
func (o *Overlay) IsVisible() bool {
	return o.visible.Load()
}

// IsDragMode returns whether drag mode is active.
func (o *Overlay) IsDragMode() bool {
	return o.dragMode.Load()
}

// SetOnPositionChanged sets the callback for position changes.
func (o *Overlay) SetOnPositionChanged(callback func(x, y int)) {
	o.onPositionChanged = callback
}

// GetPosition returns the current overlay position.
func (o *Overlay) GetPosition() (int, int) {
	if o.hwnd == 0 {
		return 0, 0
	}
	var rect RECT
	procGetWindowRect.Call(o.hwnd, uintptr(unsafe.Pointer(&rect)))
	return int(rect.Left), int(rect.Top)
}

// SetOpacity sets the overlay opacity (0.0 to 1.0). Safe to call from any goroutine.
func (o *Overlay) SetOpacity(opacity float64) {
	if o.hwnd == 0 {
		return
	}
	// Clamp opacity to valid range
	if opacity < 0.2 {
		opacity = 0.2
	}
	if opacity > 1.0 {
		opacity = 1.0
	}
	// Convert to 0-255 range and send via PostMessage
	alpha := byte(opacity * 255)
	procPostMessageW.Call(o.hwnd, WM_OVERLAY_SET_OPACITY, uintptr(alpha), 0)
}

// UpdatePosition moves overlay to a preset position. Safe to call from any goroutine.
func (o *Overlay) UpdatePosition(position string) {
	if o.hwnd == 0 {
		return
	}
	// Encode position as wParam: 0=top-right, 1=top-left, 2=bottom-right, 3=bottom-left
	var posCode uintptr
	switch position {
	case "top-left":
		posCode = 1
	case "bottom-right":
		posCode = 2
	case "bottom-left":
		posCode = 3
	default: // top-right
		posCode = 0
	}
	procPostMessageW.Call(o.hwnd, WM_OVERLAY_UPDATE_POS, posCode, 0)
}

// UpdateConfig updates the overlay config and refreshes display. Safe to call from any goroutine.
func (o *Overlay) UpdateConfig(cfg *config.OverlayConfig) {
	o.config = cfg
	if o.hwnd != 0 {
		procPostMessageW.Call(o.hwnd, WM_OVERLAY_UPDATE_CONFIG, 0, 0)
	}
}

// uiThread is the dedicated UI thread for overlay window.
// All WinAPI calls for the overlay happen here.
func (o *Overlay) uiThread() {
	// CRITICAL: Lock this goroutine to OS thread
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	o.log.Debug("Overlay UI thread started")

	// Set global for WndProc callbacks
	globalOverlay = o

	o.hInstance, _, _ = procGetModuleHandleW.Call(0)

	className, _ := syscall.UTF16PtrFromString("EREZMonitorOverlayV3")

	wc := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		Style:         CS_HREDRAW | CS_VREDRAW,
		LpfnWndProc:   syscall.NewCallback(OverlayWndProc),
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
		o.log.Error("Failed to create overlay window")
		close(o.readyCh)
		return
	}

	o.hwnd = hwnd
	o.log.Debugf("Overlay window created: hwnd=%d", hwnd)

	// Set opacity
	alpha := byte(255 * o.config.Opacity)
	if alpha < 80 {
		alpha = 80
	}
	if alpha > 220 {
		alpha = 220
	}
	procSetLayeredWindowAttributes.Call(hwnd, 0, uintptr(alpha), LWA_ALPHA)

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

	// Show if enabled
	if o.config.Enabled {
		o.visible.Store(true)
		procShowWindow.Call(hwnd, SW_SHOW)
		procInvalidateRect.Call(hwnd, 0, 1)
	}

	// Start timer for animations (50ms = ~20 FPS)
	procSetTimer.Call(hwnd, 1, 50, 0)

	// Signal that we're ready
	close(o.readyCh)

	o.log.Debug("Overlay entering message loop")

	// Message loop - runs until WM_QUIT
	var msg MSG
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

	// Cleanup
	if o.fontLarge != 0 {
		procDeleteObject.Call(o.fontLarge)
	}
	if o.fontSmall != 0 {
		procDeleteObject.Call(o.fontSmall)
	}
	procKillTimer.Call(o.hwnd, 1)

	o.log.Debug("Overlay UI thread exiting")
}

// OverlayWndProc handles window messages for the overlay.
// This runs in the UI thread.
func OverlayWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	o := globalOverlay
	if o == nil {
		ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
		return ret
	}

	switch msg {
	case WM_PAINT:
		o.paint(hwnd)
		return 0

	case WM_TIMER:
		// Update animation and repaint
		o.updateAnimation()
		if o.visible.Load() {
			procInvalidateRect.Call(hwnd, 0, 1)
		}
		return 0

	case WM_OVERLAY_SHOW:
		o.log.Debug("WM_OVERLAY_SHOW received")
		o.visible.Store(true)
		alpha := byte(255 * o.config.Opacity)
		if alpha < 80 {
			alpha = 80
		}
		if alpha > 220 {
			alpha = 220
		}
		procSetLayeredWindowAttributes.Call(hwnd, 0, uintptr(alpha), LWA_ALPHA)
		procShowWindow.Call(hwnd, SW_SHOW)
		procInvalidateRect.Call(hwnd, 0, 1)
		return 0

	case WM_OVERLAY_HIDE:
		o.log.Debug("WM_OVERLAY_HIDE received")
		o.visible.Store(false)
		procShowWindow.Call(hwnd, SW_HIDE)
		return 0

	case WM_OVERLAY_TOGGLE:
		o.log.Debug("WM_OVERLAY_TOGGLE received")
		if o.visible.Load() {
			o.visible.Store(false)
			procShowWindow.Call(hwnd, SW_HIDE)
		} else {
			o.visible.Store(true)
			alpha := byte(255 * o.config.Opacity)
			if alpha < 80 {
				alpha = 80
			}
			if alpha > 220 {
				alpha = 220
			}
			procSetLayeredWindowAttributes.Call(hwnd, 0, uintptr(alpha), LWA_ALPHA)
			procShowWindow.Call(hwnd, SW_SHOW)
			procInvalidateRect.Call(hwnd, 0, 1)
		}
		return 0

	case WM_OVERLAY_TOGGLE_DRAG:
		o.log.Debug("WM_OVERLAY_TOGGLE_DRAG received")
		newDragMode := !o.dragMode.Load()
		o.dragMode.Store(newDragMode)

		// GWL_EXSTYLE = -20, need to convert to uintptr properly
		gwlExStyle := uintptr(0xFFFFFFEC) // -20 in two's complement for 32-bit
		style, _, _ := procGetWindowLongW.Call(hwnd, gwlExStyle)
		if newDragMode {
			style = style &^ WS_EX_TRANSPARENT
		} else {
			style = style | WS_EX_TRANSPARENT
			// Save position when exiting drag mode
			if o.onPositionChanged != nil {
				x, y := o.GetPosition()
				go o.onPositionChanged(x, y)
			}
		}
		procSetWindowLongW.Call(hwnd, gwlExStyle, style)
		procInvalidateRect.Call(hwnd, 0, 1)
		return 0

	case WM_OVERLAY_STOP:
		o.log.Debug("WM_OVERLAY_STOP received")
		procDestroyWindow.Call(hwnd)
		return 0

	case WM_OVERLAY_SET_OPACITY:
		// wParam contains alpha value (0-255)
		alpha := byte(wParam)
		if alpha < 50 {
			alpha = 50
		}
		o.log.Debugf("WM_OVERLAY_SET_OPACITY received: alpha=%d", alpha)
		procSetLayeredWindowAttributes.Call(hwnd, 0, uintptr(alpha), LWA_ALPHA)
		return 0

	case WM_OVERLAY_UPDATE_POS:
		// wParam contains position code: 0=top-right, 1=top-left, 2=bottom-right, 3=bottom-left
		o.log.Debugf("WM_OVERLAY_UPDATE_POS received: pos=%d", wParam)
		screenWidth, _, _ := procGetSystemMetrics.Call(0)
		screenHeight, _, _ := procGetSystemMetrics.Call(1)
		padding := int32(15)
		var x, y int32
		switch wParam {
		case 1: // top-left
			x, y = padding, padding
		case 2: // bottom-right
			x, y = int32(screenWidth)-o.width-padding, int32(screenHeight)-o.height-padding-50
		case 3: // bottom-left
			x, y = padding, int32(screenHeight)-o.height-padding-50
		default: // 0 = top-right
			x, y = int32(screenWidth)-o.width-padding, padding
		}
		procSetWindowPos.Call(hwnd, HWND_TOPMOST, uintptr(x), uintptr(y), 0, 0, SWP_NOSIZE|SWP_NOACTIVATE)
		return 0

	case WM_OVERLAY_UPDATE_CONFIG:
		// Config was updated externally, refresh the display
		o.log.Debug("WM_OVERLAY_UPDATE_CONFIG received")
		procInvalidateRect.Call(hwnd, 0, 1)
		return 0

	case WM_NCHITTEST:
		if o.dragMode.Load() {
			ret, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
			if ret == 1 { // HTCLIENT
				return HTCAPTION
			}
			return ret
		}
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

// updateAnimation updates animation state. Called from UI thread only.
func (o *Overlay) updateAnimation() {
	// Get metrics - this is thread-safe (atomic pointer)
	var targetCPU, targetRAM, targetGPU float64
	if o.collector != nil {
		if metrics := o.collector.GetLatest(); metrics != nil {
			targetCPU = metrics.CPU.UsagePercent
			targetRAM = metrics.Memory.UsedPercent
			if metrics.GPU.Available {
				targetGPU = metrics.GPU.UsagePercent
			}
		}
	}

	// Smooth interpolation
	const lerpSpeed = 0.15
	o.anim.cpuPercent = lerp(o.anim.cpuPercent, targetCPU, lerpSpeed)
	o.anim.ramPercent = lerp(o.anim.ramPercent, targetRAM, lerpSpeed)
	o.anim.gpuPercent = lerp(o.anim.gpuPercent, targetGPU, lerpSpeed)

	// Add to history every ~500ms (30 frames at 60fps)
	o.history.ticker++
	if o.history.ticker >= 30 {
		o.history.ticker = 0
		o.addHistorySample(targetCPU, targetRAM, targetGPU)
	}

	// Critical state
	const criticalThreshold = 85.0
	o.anim.cpuCritical = targetCPU >= criticalThreshold
	o.anim.ramCritical = targetRAM >= criticalThreshold
	o.anim.gpuCritical = targetGPU >= criticalThreshold

	// Pulse animation
	if o.anim.cpuCritical || o.anim.ramCritical || o.anim.gpuCritical {
		o.anim.pulsePhase += 0.15
		if o.anim.pulsePhase > 2*math.Pi {
			o.anim.pulsePhase -= 2 * math.Pi
		}
	}
}

// paint draws the overlay. Called from UI thread only.
func (o *Overlay) paint(hwnd uintptr) {
	var ps PAINTSTRUCT
	hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))

	if hdc == 0 {
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return
	}

	// Get metrics
	var metrics *models.Metrics
	if o.collector != nil {
		metrics = o.collector.GetLatest()
	}

	// Background
	bgBrush, _, _ := procCreateSolidBrush.Call(COLOR_BG_DARK)
	rect := RECT{Left: 0, Top: 0, Right: o.width, Bottom: o.height}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rect)), bgBrush)
	procDeleteObject.Call(bgBrush)

	// Left accent bar
	accentColor := uintptr(COLOR_ACCENT)
	if o.dragMode.Load() {
		accentColor = uintptr(COLOR_ORANGE)
	}
	accentBrush, _, _ := procCreateSolidBrush.Call(accentColor)
	accentRect := RECT{Left: 0, Top: 0, Right: 4, Bottom: o.height}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&accentRect)), accentBrush)
	procDeleteObject.Call(accentBrush)

	// Drag mode border
	if o.dragMode.Load() {
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

	y := int32(10)
	rowHeight := int32(28)
	labelX := int32(12)
	barX := int32(52)
	barWidth := int32(130)
	barHeight := int32(8)
	valueX := int32(190)

	pulseMultiplier := 0.85 + 0.15*math.Sin(o.anim.pulsePhase)

	if metrics != nil {
		// CPU
		if o.config.ShowCPU {
			o.drawMetricRowAnimated(hdc, "CPU", o.anim.cpuPercent, o.anim.cpuCritical, pulseMultiplier, y, labelX, barX, barWidth, barHeight, valueX)
			y += rowHeight
		}

		// RAM
		if o.config.ShowRAM {
			o.drawMetricRowAnimated(hdc, "RAM", o.anim.ramPercent, o.anim.ramCritical, pulseMultiplier, y, labelX, barX, barWidth, barHeight, valueX)
			procSelectObject.Call(hdc, o.fontSmall)
			procSetTextColor.Call(hdc, COLOR_TEXT_GRAY)
			memText := fmt.Sprintf("%dG / %dG", metrics.Memory.UsedMB/1024, metrics.Memory.TotalMB/1024)
			o.drawText(hdc, memText, barX, y+12)
			y += rowHeight + 4
		}

		// GPU
		if o.config.ShowGPU && metrics.GPU.Available {
			o.drawMetricRowAnimated(hdc, "GPU", o.anim.gpuPercent, o.anim.gpuCritical, pulseMultiplier, y, labelX, barX, barWidth, barHeight, valueX)
			procSelectObject.Call(hdc, o.fontSmall)
			procSetTextColor.Call(hdc, COLOR_TEXT_GRAY)
			vramGB := float64(metrics.GPU.VRAMUsedMB) / 1024.0
			totalGB := float64(metrics.GPU.VRAMTotalMB) / 1024.0
			vramText := fmt.Sprintf("%.1fG/%.0fG", vramGB, totalGB)
			o.drawText(hdc, vramText, barX, y+12)
			if metrics.GPU.TemperatureC > 0 {
				procSetTextColor.Call(hdc, getTempColor(metrics.GPU.TemperatureC))
				tempText := fmt.Sprintf("%d°C", metrics.GPU.TemperatureC)
				o.drawText(hdc, tempText, barX+75, y+12)
			}
			y += rowHeight + 4
		}

		// Separator
		if o.config.ShowNet || o.config.ShowDisk {
			y += 2
			sepBrush, _, _ := procCreateSolidBrush.Call(COLOR_BG_BAR)
			sepRect := RECT{Left: 12, Top: y, Right: o.width - 12, Bottom: y + 1}
			procFillRect.Call(hdc, uintptr(unsafe.Pointer(&sepRect)), sepBrush)
			procDeleteObject.Call(sepBrush)
			y += 8
		}

		// Network
		if o.config.ShowNet {
			procSelectObject.Call(hdc, o.fontSmall)
			procSetTextColor.Call(hdc, COLOR_TEXT_GRAY)
			o.drawText(hdc, "NET", labelX, y)

			procSetTextColor.Call(hdc, COLOR_CYAN)
			var dlText string
			if metrics.Network.DownloadKBps >= 1024 {
				dlText = fmt.Sprintf("↓%.1f MB/s", metrics.Network.DownloadKBps/1024)
			} else {
				dlText = fmt.Sprintf("↓%.0f KB/s", metrics.Network.DownloadKBps)
			}
			o.drawText(hdc, dlText, barX, y)

			var ulText string
			if metrics.Network.UploadKBps >= 1024 {
				ulText = fmt.Sprintf("↑%.1f MB/s", metrics.Network.UploadKBps/1024)
			} else {
				ulText = fmt.Sprintf("↑%.0f KB/s", metrics.Network.UploadKBps)
			}
			o.drawText(hdc, ulText, barX+85, y)
			y += 18

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

		// Disk
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
		procSetTextColor.Call(hdc, COLOR_TEXT)
		o.drawText(hdc, "Loading...", 12, 80)
	}

	procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
}
func (o *Overlay) drawMetricRowAnimated(hdc uintptr, label string, percent float64, isCritical bool, pulseMultiplier float64, y, labelX, barX, barWidth, barHeight, valueX int32) {
	procSelectObject.Call(hdc, o.fontSmall)
	if isCritical {
		pulseColor := blendColors(COLOR_TEXT_GRAY, COLOR_RED, pulseMultiplier)
		procSetTextColor.Call(hdc, pulseColor)
	} else {
		procSetTextColor.Call(hdc, COLOR_TEXT_GRAY)
	}
	o.drawText(hdc, label, labelX, y)

	barY := y + 2
	bgBrush, _, _ := procCreateSolidBrush.Call(COLOR_BG_BAR)
	bgRect := RECT{Left: barX, Top: barY, Right: barX + barWidth, Bottom: barY + barHeight}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&bgRect)), bgBrush)
	procDeleteObject.Call(bgBrush)

	if percent > 0.5 {
		fillWidth := int32(float64(barWidth) * percent / 100.0)
		if fillWidth < 4 {
			fillWidth = 4
		}
		if fillWidth > barWidth {
			fillWidth = barWidth
		}

		// Draw gradient bar - from green to yellow to red based on position
		// Draw in segments for performance (every 2 pixels)
		segmentWidth := int32(2)
		for x := int32(0); x < fillWidth; x += segmentWidth {
			// Calculate color based on position in the bar (0-100%)
			posPercent := float64(x) / float64(barWidth) * 100.0

			var r, g, b int
			if posPercent < 50 {
				// Green to Yellow (0-50%)
				factor := posPercent / 50.0
				r = int(factor * 255)
				g = 200
				b = 0
			} else if posPercent < 75 {
				// Yellow to Orange (50-75%)
				factor := (posPercent - 50) / 25.0
				r = 255
				g = int(200 - factor*80)
				b = 0
			} else {
				// Orange to Red (75-100%)
				factor := (posPercent - 75) / 25.0
				r = 255
				g = int(120 - factor*120)
				b = 0
			}

			// Apply pulse effect if critical
			if isCritical {
				brightness := 0.7 + 0.3*pulseMultiplier
				r = int(float64(r) * brightness)
				g = int(float64(g) * brightness)
				b = int(float64(b) * brightness)
			}

			segEnd := x + segmentWidth
			if segEnd > fillWidth {
				segEnd = fillWidth
			}

			color := uintptr(r | (g << 8) | (b << 16))
			brush, _, _ := procCreateSolidBrush.Call(color)
			pixelRect := RECT{Left: barX + x, Top: barY, Right: barX + segEnd, Bottom: barY + barHeight}
			procFillRect.Call(hdc, uintptr(unsafe.Pointer(&pixelRect)), brush)
			procDeleteObject.Call(brush)
		}
	}

	procSelectObject.Call(hdc, o.fontLarge)
	textColor := getValueColor(percent)
	if isCritical {
		textColor = pulseColorFn(textColor, pulseMultiplier)
	}
	procSetTextColor.Call(hdc, textColor)
	valueText := fmt.Sprintf("%.0f%%", percent)
	o.drawText(hdc, valueText, valueX, y-2)
}

func (o *Overlay) drawText(hdc uintptr, text string, x, y int32) {
	textW, _ := syscall.UTF16FromString(text)
	procTextOutW.Call(hdc, uintptr(x), uintptr(y), uintptr(unsafe.Pointer(&textW[0])), uintptr(len(textW)-1))
}

func pulseColorFn(color uintptr, multiplier float64) uintptr {
	b := byte(color & 0xFF)
	g := byte((color >> 8) & 0xFF)
	r := byte((color >> 16) & 0xFF)

	adjust := 1.0 + (multiplier-0.85)*0.5
	b = byte(math.Min(255, float64(b)*adjust))
	g = byte(math.Min(255, float64(g)*adjust))
	r = byte(math.Min(255, float64(r)*adjust))

	return uintptr(b) | (uintptr(g) << 8) | (uintptr(r) << 16)
}

// drawMetricIcon draws a colored icon/symbol for a metric
func (o *Overlay) drawMetricIcon(hdc uintptr, icon string, x, y int32, color uintptr) {
	procSelectObject.Call(hdc, o.fontSmall)
	procSetTextColor.Call(hdc, color)
	textW, _ := syscall.UTF16FromString(icon)
	procTextOutW.Call(hdc, uintptr(x), uintptr(y), uintptr(unsafe.Pointer(&textW[0])), uintptr(len(textW)-1))
}

// drawStylishSeparator draws a stylish dotted separator line
func (o *Overlay) drawStylishSeparator(hdc uintptr, startX, y, endX int32) {
	dotBrush, _, _ := procCreateSolidBrush.Call(COLOR_BORDER)
	// Draw gradient dots
	dotSpacing := int32(8)
	dotSize := int32(2)
	for x := startX; x < endX; x += dotSpacing {
		// Fade effect at edges
		distFromCenter := float64(x-startX) / float64(endX-startX)
		alpha := 1.0
		if distFromCenter < 0.1 {
			alpha = distFromCenter * 10
		} else if distFromCenter > 0.9 {
			alpha = (1.0 - distFromCenter) * 10
		}

		if alpha > 0.3 {
			dotRect := RECT{Left: x, Top: y, Right: x + dotSize, Bottom: y + dotSize}
			procFillRect.Call(hdc, uintptr(unsafe.Pointer(&dotRect)), dotBrush)
		}
	}
	procDeleteObject.Call(dotBrush)

	// Draw center accent dot
	accentBrush, _, _ := procCreateSolidBrush.Call(COLOR_ACCENT)
	centerX := (startX + endX) / 2
	accentRect := RECT{Left: centerX - 1, Top: y - 1, Right: centerX + 3, Bottom: y + 3}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&accentRect)), accentBrush)
	procDeleteObject.Call(accentBrush)
}
