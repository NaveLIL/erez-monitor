// Package utils provides utility functions for formatting and conversions.
package utils

import (
	"fmt"
	"strings"
)

// FormatBytes converts bytes to a human-readable string.
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatBytesPerSecond converts bytes/s to a human-readable string.
func FormatBytesPerSecond(bytesPerSec float64) string {
	const unit = 1024.0
	if bytesPerSec < unit {
		return fmt.Sprintf("%.1f B/s", bytesPerSec)
	}
	div, exp := unit, 0
	for n := bytesPerSec / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB/s", bytesPerSec/div, "KMGTPE"[exp])
}

// FormatKBps converts KB/s to a human-readable string.
func FormatKBps(kbps float64) string {
	if kbps < 1024 {
		return fmt.Sprintf("%.1f KB/s", kbps)
	}
	if kbps < 1024*1024 {
		return fmt.Sprintf("%.1f MB/s", kbps/1024)
	}
	return fmt.Sprintf("%.1f GB/s", kbps/(1024*1024))
}

// FormatMBps converts MB/s to a human-readable string.
func FormatMBps(mbps float64) string {
	if mbps < 1 {
		return fmt.Sprintf("%.2f KB/s", mbps*1024)
	}
	if mbps < 1024 {
		return fmt.Sprintf("%.1f MB/s", mbps)
	}
	return fmt.Sprintf("%.1f GB/s", mbps/1024)
}

// FormatMB converts megabytes to a human-readable string.
func FormatMB(mb uint64) string {
	if mb < 1024 {
		return fmt.Sprintf("%d MB", mb)
	}
	if mb < 1024*1024 {
		return fmt.Sprintf("%.1f GB", float64(mb)/1024)
	}
	return fmt.Sprintf("%.1f TB", float64(mb)/(1024*1024))
}

// FormatGB converts gigabytes to a human-readable string.
func FormatGB(gb uint64) string {
	if gb < 1024 {
		return fmt.Sprintf("%d GB", gb)
	}
	return fmt.Sprintf("%.1f TB", float64(gb)/1024)
}

// FormatPercent formats a percentage value.
func FormatPercent(percent float64) string {
	return fmt.Sprintf("%.1f%%", percent)
}

// FormatPercentInt formats a percentage value as an integer.
func FormatPercentInt(percent float64) string {
	return fmt.Sprintf("%d%%", int(percent))
}

// FormatTemperature formats a temperature value in Celsius.
func FormatTemperature(celsius float64) string {
	return fmt.Sprintf("%.0f°C", celsius)
}

// FormatTemperatureInt formats a temperature value in Celsius.
func FormatTemperatureInt(celsius uint32) string {
	return fmt.Sprintf("%d°C", celsius)
}

// FormatFrequency converts MHz to a human-readable string.
func FormatFrequency(mhz uint32) string {
	if mhz < 1000 {
		return fmt.Sprintf("%d MHz", mhz)
	}
	return fmt.Sprintf("%.2f GHz", float64(mhz)/1000)
}

// FormatDuration formats a duration in a human-readable format.
func FormatDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm %ds", seconds/60, seconds%60)
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

// FormatUptime formats an uptime duration.
func FormatUptime(seconds uint64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60

	parts := make([]string, 0)
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}

	return strings.Join(parts, " ")
}

// TruncateString truncates a string to a maximum length.
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// PadLeft pads a string with spaces on the left.
func PadLeft(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(" ", width-len(s)) + s
}

// PadRight pads a string with spaces on the right.
func PadRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// Clamp constrains a value between min and max.
func Clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ClampInt constrains an integer value between min and max.
func ClampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// PercentToColor returns a color based on percentage (green -> yellow -> red).
func PercentToColor(percent float64) (r, g, b uint8) {
	if percent < 50 {
		// Green to Yellow (0-50%)
		g = 255
		r = uint8((percent / 50) * 255)
	} else {
		// Yellow to Red (50-100%)
		r = 255
		g = uint8((1 - (percent-50)/50) * 255)
	}
	return r, g, b
}

// PercentToHexColor returns a hex color string based on percentage.
func PercentToHexColor(percent float64) string {
	r, g, b := PercentToColor(percent)
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

// GetStatusColor returns a color based on status level.
func GetStatusColor(percent float64, warningThreshold, criticalThreshold float64) string {
	if percent >= criticalThreshold {
		return "#FF0000" // Red
	}
	if percent >= warningThreshold {
		return "#FFFF00" // Yellow
	}
	return "#00FF00" // Green
}
