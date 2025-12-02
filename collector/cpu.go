package collector

import (
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"

	"github.com/NaveLIL/erez-monitor/models"
)

// CPUInfo contains static CPU information.
type CPUInfo struct {
	Model   string
	Cores   int
	Threads int
}

// CPUCollector collects CPU metrics.
type CPUCollector struct {
	info     *CPUInfo
	infoOnce sync.Once
}

// NewCPUCollector creates a new CPU collector.
func NewCPUCollector() *CPUCollector {
	return &CPUCollector{}
}

// Collect gathers current CPU metrics.
func (c *CPUCollector) Collect() models.CPUMetrics {
	metrics := models.CPUMetrics{}

	// Get overall CPU usage (with 0 interval for immediate reading)
	percentages, err := cpu.Percent(0, false)
	if err == nil && len(percentages) > 0 {
		metrics.UsagePercent = percentages[0]
	}

	// Get per-core CPU usage
	perCore, err := cpu.Percent(0, true)
	if err == nil {
		metrics.PerCorePercent = perCore
	}

	// Get CPU frequency
	freqs, err := cpu.Info()
	if err == nil && len(freqs) > 0 {
		metrics.FrequencyMHz = uint32(freqs[0].Mhz)
	}

	// Get CPU temperature via WMI (Windows-specific)
	metrics.Temperature = c.getTemperature()

	return metrics
}

// getTemperature gets CPU temperature via WMI on Windows.
func (c *CPUCollector) getTemperature() float64 {
	// Try to get temperature via WMI
	// This is a simplified implementation - full WMI query would be:
	// SELECT * FROM MSAcpi_ThermalZoneTemperature
	// Note: This requires admin privileges on most systems

	// For now, we return 0 as temperature reading requires
	// platform-specific implementation with WMI or third-party tools
	return 0
}

// GetInfo returns static CPU information.
func (c *CPUCollector) GetInfo() *CPUInfo {
	c.infoOnce.Do(func() {
		c.info = &CPUInfo{}

		// Get CPU info
		infos, err := cpu.Info()
		if err == nil && len(infos) > 0 {
			c.info.Model = infos[0].ModelName
		}

		// Get core counts
		physical, err := cpu.Counts(false)
		if err == nil {
			c.info.Cores = physical
		}

		logical, err := cpu.Counts(true)
		if err == nil {
			c.info.Threads = logical
		}
	})

	return c.info
}

// GetUsageOverTime gets CPU usage over a specified duration.
// This is a blocking call that waits for the duration.
func (c *CPUCollector) GetUsageOverTime(duration time.Duration) (float64, error) {
	percentages, err := cpu.Percent(duration, false)
	if err != nil {
		return 0, err
	}
	if len(percentages) > 0 {
		return percentages[0], nil
	}
	return 0, nil
}

// GetPerCoreUsageOverTime gets per-core CPU usage over a specified duration.
func (c *CPUCollector) GetPerCoreUsageOverTime(duration time.Duration) ([]float64, error) {
	return cpu.Percent(duration, true)
}

// GetTimes returns CPU times for all cores.
func (c *CPUCollector) GetTimes() ([]cpu.TimesStat, error) {
	return cpu.Times(true)
}
