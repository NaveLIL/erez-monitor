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

	WM_DESTROY = 0x0002
	WM_PAINT   = 0x000F
	WM_TIMER   = 0x0113
	WM_CLOSE   = 0x0010

	TRANSPARENT = 1
	CS_HREDRAW  = 0x0002
	CS_VREDRAW  = 0x0001
)

// Color constants (BGR format)
const (
	COLOR_BG_DARK    = 0x1A1A1A // Darker background
	COLOR_BG_ACCENT  = 0x2A2A2A // Accent background
	COLOR_TEXT_WHITE = 0xE0E0E0 // Soft white text
	COLOR_TEXT_GRAY  = 0x888888 // Gray text for labels
	COLOR_GREEN      = 0x00DD77 // Bright green (good)
	COLOR_YELLOW     = 0x00DDDD // Yellow/cyan (warning)
	COLOR_ORANGE     = 0x0088FF // Orange (high)
	COLOR_RED        = 0x4444FF // Red (critical)
	COLOR_CYAN       = 0xFFDD00 // Cyan for network
	COLOR_PURPLE     = 0xFF6699 // Pink/purple for disk
	COLOR_BLUE       = 0xFF8844 // Blue for FPS/info
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

	visible bool
	running bool
	mu      sync.RWMutex

	currentMetrics *models.Metrics
	metricsMu      sync.RWMutex

	stopCh chan struct{}

	width  int32
	height int32
}

// Global overlay instance for window proc callback
var globalOverlay *Overlay

// NewOverlay creates a new overlay window.
func NewOverlay(cfg *config.OverlayConfig, coll *collector.Collector) *Overlay {
	return &Overlay{
		config:    cfg,
		collector: coll,
		stopCh:    make(chan struct{}),
		width:     220,
		height:    155,
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
	defer o.mu.Unlock()

	o.visible = true
	if o.hwnd != 0 {
		procShowWindow.Call(o.hwnd, SW_SHOW)
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

	metricsCh := make(chan *models.Metrics, 1)
	o.collector.Subscribe(metricsCh)
	defer o.collector.Unsubscribe(metricsCh)

	go func() {
		for {
			select {
			case <-o.stopCh:
				return
			case m := <-metricsCh:
				o.metricsMu.Lock()
				o.currentMetrics = m
				o.metricsMu.Unlock()
				if o.hwnd != 0 {
					procInvalidateRect.Call(o.hwnd, 0, 1)
				}
			}
		}
	}()

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
		if globalOverlay != nil && globalOverlay.hwnd != 0 {
			procInvalidateRect.Call(hwnd, 0, 1)
		}
		return 0

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

func (o *Overlay) paint(hwnd uintptr) {
	var ps PAINTSTRUCT
	hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))

	o.metricsMu.RLock()
	metrics := o.currentMetrics
	o.metricsMu.RUnlock()

	// Main background
	bgBrush, _, _ := procCreateSolidBrush.Call(COLOR_BG_DARK)
	rect := RECT{Left: 0, Top: 0, Right: o.width, Bottom: o.height}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rect)), bgBrush)
	procDeleteObject.Call(bgBrush)

	// Left accent bar
	accentBrush, _, _ := procCreateSolidBrush.Call(COLOR_GREEN)
	accentRect := RECT{Left: 0, Top: 0, Right: 3, Bottom: o.height}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&accentRect)), accentBrush)
	procDeleteObject.Call(accentBrush)

	procSetBkMode.Call(hdc, TRANSPARENT)

	y := int32(10)
	lineHeight := int32(22)
	labelX := int32(12)
	valueX := int32(55)
	infoX := int32(115)

	if metrics != nil {
		// === CPU ===
		procSelectObject.Call(hdc, o.fontSmall)
		procSetTextColor.Call(hdc, COLOR_TEXT_GRAY)
		o.drawText(hdc, "CPU", labelX, y+2)

		procSelectObject.Call(hdc, o.fontLarge)
		procSetTextColor.Call(hdc, getValueColor(metrics.CPU.UsagePercent))
		o.drawText(hdc, fmt.Sprintf("%.0f%%", metrics.CPU.UsagePercent), valueX, y)

		if metrics.CPU.Temperature > 0 {
			procSelectObject.Call(hdc, o.fontSmall)
			procSetTextColor.Call(hdc, getTempColor(uint32(metrics.CPU.Temperature)))
			o.drawText(hdc, fmt.Sprintf("%.0f°C", metrics.CPU.Temperature), infoX+20, y+2)
		}
		y += lineHeight

		// === RAM ===
		procSelectObject.Call(hdc, o.fontSmall)
		procSetTextColor.Call(hdc, COLOR_TEXT_GRAY)
		o.drawText(hdc, "RAM", labelX, y+2)

		procSelectObject.Call(hdc, o.fontLarge)
		procSetTextColor.Call(hdc, getValueColor(metrics.Memory.UsedPercent))
		o.drawText(hdc, fmt.Sprintf("%.0f%%", metrics.Memory.UsedPercent), valueX, y)

		procSelectObject.Call(hdc, o.fontSmall)
		procSetTextColor.Call(hdc, COLOR_TEXT_GRAY)
		o.drawText(hdc, fmt.Sprintf("%d/%dG", metrics.Memory.UsedMB/1024, metrics.Memory.TotalMB/1024), infoX, y+2)
		y += lineHeight

		// === GPU ===
		if metrics.GPU.Available {
			procSelectObject.Call(hdc, o.fontSmall)
			procSetTextColor.Call(hdc, COLOR_TEXT_GRAY)
			o.drawText(hdc, "GPU", labelX, y+2)

			procSelectObject.Call(hdc, o.fontLarge)
			procSetTextColor.Call(hdc, getValueColor(metrics.GPU.UsagePercent))
			o.drawText(hdc, fmt.Sprintf("%.0f%%", metrics.GPU.UsagePercent), valueX, y)

			if metrics.GPU.TemperatureC > 0 {
				procSelectObject.Call(hdc, o.fontSmall)
				procSetTextColor.Call(hdc, getTempColor(metrics.GPU.TemperatureC))
				o.drawText(hdc, fmt.Sprintf("%d°C", metrics.GPU.TemperatureC), infoX+20, y+2)
			}
			y += lineHeight

			// VRAM on same style
			if metrics.GPU.VRAMTotalMB > 0 {
				procSelectObject.Call(hdc, o.fontSmall)
				procSetTextColor.Call(hdc, COLOR_TEXT_GRAY)
				o.drawText(hdc, "VRAM", labelX, y+2)

				vramPercent := float64(metrics.GPU.VRAMUsedMB) / float64(metrics.GPU.VRAMTotalMB) * 100
				procSetTextColor.Call(hdc, getValueColor(vramPercent))
				o.drawText(hdc, fmt.Sprintf("%.0f%%", vramPercent), valueX+5, y+2)

				procSetTextColor.Call(hdc, COLOR_TEXT_GRAY)
				o.drawText(hdc, fmt.Sprintf("%dM", metrics.GPU.VRAMUsedMB), infoX, y+2)
				y += lineHeight
			}
		}

		// Separator line
		sepBrush, _, _ := procCreateSolidBrush.Call(0x333333)
		sepRect := RECT{Left: 10, Top: y, Right: o.width - 10, Bottom: y + 1}
		procFillRect.Call(hdc, uintptr(unsafe.Pointer(&sepRect)), sepBrush)
		procDeleteObject.Call(sepBrush)
		y += 6

		// === Network ===
		procSelectObject.Call(hdc, o.fontSmall)
		procSetTextColor.Call(hdc, COLOR_TEXT_GRAY)
		o.drawText(hdc, "NET", labelX, y)

		procSetTextColor.Call(hdc, COLOR_CYAN)
		var dlText, ulText string
		if metrics.Network.DownloadKBps >= 1024 {
			dlText = fmt.Sprintf("↓%.1fM", metrics.Network.DownloadKBps/1024)
		} else {
			dlText = fmt.Sprintf("↓%.0fK", metrics.Network.DownloadKBps)
		}
		if metrics.Network.UploadKBps >= 1024 {
			ulText = fmt.Sprintf("↑%.1fM", metrics.Network.UploadKBps/1024)
		} else {
			ulText = fmt.Sprintf("↑%.0fK", metrics.Network.UploadKBps)
		}
		o.drawText(hdc, dlText, valueX-5, y)
		o.drawText(hdc, ulText, infoX+15, y)
		y += lineHeight - 4

		// === Disk I/O ===
		if metrics.Disk.ReadMBps > 0.01 || metrics.Disk.WriteMBps > 0.01 {
			procSetTextColor.Call(hdc, COLOR_TEXT_GRAY)
			o.drawText(hdc, "DISK", labelX, y)

			procSetTextColor.Call(hdc, COLOR_PURPLE)
			o.drawText(hdc, fmt.Sprintf("R:%.1f W:%.1f", metrics.Disk.ReadMBps, metrics.Disk.WriteMBps), valueX-5, y)
		}

	} else {
		procSelectObject.Call(hdc, o.fontLarge)
		procSetTextColor.Call(hdc, COLOR_TEXT_WHITE)
		o.drawText(hdc, "Loading...", 12, 60)
	}

	procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
}

func (o *Overlay) drawText(hdc uintptr, text string, x, y int32) {
	textW, _ := syscall.UTF16FromString(text)
	procTextOutW.Call(hdc, uintptr(x), uintptr(y), uintptr(unsafe.Pointer(&textW[0])), uintptr(len(textW)-1))
}
