// Package utils provides Windows-specific utility functions.
//go:build windows

package utils

import (
	"syscall"
	"unsafe"
)

var (
	user32                         = syscall.NewLazyDLL("user32.dll")
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procMessageBeep                = user32.NewProc("MessageBeep")
	procSetWindowPos               = user32.NewProc("SetWindowPos")
	procGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	procGetWindowTextW             = user32.NewProc("GetWindowTextW")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procSetWindowLongPtrW          = user32.NewProc("SetWindowLongPtrW")
	procGetWindowLongPtrW          = user32.NewProc("GetWindowLongPtrW")
	procRegisterHotKey             = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey           = user32.NewProc("UnregisterHotKey")
	procGetMessageW                = user32.NewProc("GetMessageW")
)

// gwlExStyle returns GWL_EXSTYLE (-20) as uintptr safely.
func gwlExStyle() uintptr {
	return uintptr(0xFFFFFFEC) // -20 as unsigned 32-bit
}

// Window style constants
const (
	WS_EX_LAYERED     uintptr = 0x00080000
	WS_EX_TRANSPARENT uintptr = 0x00000020
	WS_EX_TOPMOST     uintptr = 0x00000008
	WS_EX_TOOLWINDOW  uintptr = 0x00000080
	WS_EX_NOACTIVATE  uintptr = 0x08000000

	WS_POPUP    uintptr = 0x80000000
	WS_VISIBLE  uintptr = 0x10000000
	WS_DISABLED uintptr = 0x08000000

	HWND_TOPMOST   uintptr = ^uintptr(0) // -1 as uintptr
	HWND_NOTOPMOST uintptr = ^uintptr(1) // -2 as uintptr

	SWP_NOMOVE     = 0x0002
	SWP_NOSIZE     = 0x0001
	SWP_NOACTIVATE = 0x0010
	SWP_SHOWWINDOW = 0x0040

	LWA_ALPHA    = 0x00000002
	LWA_COLORKEY = 0x00000001

	// MessageBeep types
	MB_OK              = 0x00000000
	MB_ICONHAND        = 0x00000010
	MB_ICONQUESTION    = 0x00000020
	MB_ICONEXCLAMATION = 0x00000030
	MB_ICONASTERISK    = 0x00000040

	// Virtual key codes
	MOD_ALT     = 0x0001
	MOD_CONTROL = 0x0002
	MOD_SHIFT   = 0x0004
	MOD_WIN     = 0x0008

	// Window messages
	WM_HOTKEY = 0x0312
)

// MSG represents a Windows message.
type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

// MessageBeep plays a system sound.
func MessageBeep(beepType uint32) error {
	ret, _, err := procMessageBeep.Call(uintptr(beepType))
	if ret == 0 {
		return err
	}
	return nil
}

// PlayAlertSound plays the system exclamation sound.
func PlayAlertSound() error {
	return MessageBeep(MB_ICONEXCLAMATION)
}

// SetWindowTopmost sets a window to be always on top.
func SetWindowTopmost(hwnd uintptr, topmost bool) error {
	var insertAfter uintptr
	if topmost {
		insertAfter = HWND_TOPMOST
	} else {
		insertAfter = HWND_NOTOPMOST
	}

	ret, _, err := procSetWindowPos.Call(
		hwnd,
		insertAfter,
		0, 0, 0, 0,
		SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE,
	)
	if ret == 0 {
		return err
	}
	return nil
}

// GetForegroundWindow returns the handle of the foreground window.
func GetForegroundWindow() uintptr {
	ret, _, _ := procGetForegroundWindow.Call()
	return ret
}

// GetWindowText gets the text of a window.
func GetWindowText(hwnd uintptr) string {
	buf := make([]uint16, 256)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 256)
	return syscall.UTF16ToString(buf)
}

// SetWindowOpacity sets the opacity of a layered window.
func SetWindowOpacity(hwnd uintptr, alpha byte) error {
	// First, add WS_EX_LAYERED style if not present
	style, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle())
	if style&WS_EX_LAYERED == 0 {
		procSetWindowLongPtrW.Call(hwnd, gwlExStyle(), style|WS_EX_LAYERED)
	}

	ret, _, err := procSetLayeredWindowAttributes.Call(
		hwnd,
		0,
		uintptr(alpha),
		LWA_ALPHA,
	)
	if ret == 0 {
		return err
	}
	return nil
}

// MakeWindowClickThrough makes a window click-through.
func MakeWindowClickThrough(hwnd uintptr) error {
	style, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle())
	newStyle := style | WS_EX_LAYERED | WS_EX_TRANSPARENT

	ret, _, err := procSetWindowLongPtrW.Call(hwnd, gwlExStyle(), newStyle)
	if ret == 0 {
		return err
	}
	return nil
}

// RegisterHotKey registers a global hotkey.
// id should be unique for each hotkey.
// modifiers can be MOD_ALT, MOD_CONTROL, MOD_SHIFT, MOD_WIN combined with |
// vk is the virtual key code.
func RegisterHotKey(hwnd uintptr, id int, modifiers uint32, vk uint32) error {
	ret, _, err := procRegisterHotKey.Call(
		hwnd,
		uintptr(id),
		uintptr(modifiers),
		uintptr(vk),
	)
	if ret == 0 {
		return err
	}
	return nil
}

// UnregisterHotKey unregisters a global hotkey.
func UnregisterHotKey(hwnd uintptr, id int) error {
	ret, _, err := procUnregisterHotKey.Call(hwnd, uintptr(id))
	if ret == 0 {
		return err
	}
	return nil
}

// GetMessage retrieves a message from the message queue.
func GetMessage(msg *MSG, hwnd uintptr, msgFilterMin, msgFilterMax uint32) (bool, error) {
	ret, _, err := procGetMessageW.Call(
		uintptr(unsafe.Pointer(msg)),
		hwnd,
		uintptr(msgFilterMin),
		uintptr(msgFilterMax),
	)
	if int32(ret) == -1 {
		return false, err
	}
	return ret != 0, nil
}

// ParseHotkey parses a hotkey string (e.g., "Ctrl+Shift+M") into modifiers and key.
func ParseHotkey(hotkey string) (modifiers uint32, vk uint32, ok bool) {
	// Map of modifier names to constants
	modMap := map[string]uint32{
		"ctrl":    MOD_CONTROL,
		"control": MOD_CONTROL,
		"alt":     MOD_ALT,
		"shift":   MOD_SHIFT,
		"win":     MOD_WIN,
	}

	// Map of virtual key codes for common keys
	vkMap := map[string]uint32{
		"a": 0x41, "b": 0x42, "c": 0x43, "d": 0x44, "e": 0x45,
		"f": 0x46, "g": 0x47, "h": 0x48, "i": 0x49, "j": 0x4A,
		"k": 0x4B, "l": 0x4C, "m": 0x4D, "n": 0x4E, "o": 0x4F,
		"p": 0x50, "q": 0x51, "r": 0x52, "s": 0x53, "t": 0x54,
		"u": 0x55, "v": 0x56, "w": 0x57, "x": 0x58, "y": 0x59,
		"z": 0x5A,
		"0": 0x30, "1": 0x31, "2": 0x32, "3": 0x33, "4": 0x34,
		"5": 0x35, "6": 0x36, "7": 0x37, "8": 0x38, "9": 0x39,
		"f1": 0x70, "f2": 0x71, "f3": 0x72, "f4": 0x73, "f5": 0x74,
		"f6": 0x75, "f7": 0x76, "f8": 0x77, "f9": 0x78, "f10": 0x79,
		"f11": 0x7A, "f12": 0x7B,
		"space": 0x20, "enter": 0x0D, "tab": 0x09, "escape": 0x1B, "esc": 0x1B,
	}

	// Parse the hotkey string
	parts := splitHotkey(hotkey)
	if len(parts) == 0 {
		return 0, 0, false
	}

	for i, part := range parts {
		lower := toLower(part)
		if i == len(parts)-1 {
			// Last part is the key
			if v, ok := vkMap[lower]; ok {
				vk = v
			} else {
				return 0, 0, false
			}
		} else {
			// Other parts are modifiers
			if m, ok := modMap[lower]; ok {
				modifiers |= m
			} else {
				return 0, 0, false
			}
		}
	}

	return modifiers, vk, true
}

// splitHotkey splits a hotkey string by + separator.
func splitHotkey(s string) []string {
	var result []string
	var current string

	for _, c := range s {
		if c == '+' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else if c != ' ' {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}

	return result
}

// toLower converts a string to lowercase without importing strings.
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}
