package collector

import (
	"sync"

	"github.com/shirou/gopsutil/v3/mem"

	"github.com/NaveLIL/erez-monitor/models"
)

// MemoryInfo contains static memory information.
type MemoryInfo struct {
	TotalMB uint64
	SwapMB  uint64
}

// MemoryCollector collects memory metrics.
type MemoryCollector struct {
	info     *MemoryInfo
	infoOnce sync.Once
}

// NewMemoryCollector creates a new memory collector.
func NewMemoryCollector() *MemoryCollector {
	return &MemoryCollector{}
}

// Collect gathers current memory metrics.
func (c *MemoryCollector) Collect() models.MemoryMetrics {
	metrics := models.MemoryMetrics{}

	// Get virtual memory stats
	vmStat, err := mem.VirtualMemory()
	if err == nil {
		metrics.TotalMB = vmStat.Total / (1024 * 1024)
		metrics.UsedMB = vmStat.Used / (1024 * 1024)
		metrics.UsedPercent = vmStat.UsedPercent
	}

	// Get swap memory stats
	swapStat, err := mem.SwapMemory()
	if err == nil {
		metrics.SwapUsedMB = swapStat.Used / (1024 * 1024)
		metrics.SwapTotalMB = swapStat.Total / (1024 * 1024)
	}

	return metrics
}

// GetInfo returns static memory information.
func (c *MemoryCollector) GetInfo() *MemoryInfo {
	c.infoOnce.Do(func() {
		c.info = &MemoryInfo{}

		vmStat, err := mem.VirtualMemory()
		if err == nil {
			c.info.TotalMB = vmStat.Total / (1024 * 1024)
		}

		swapStat, err := mem.SwapMemory()
		if err == nil {
			c.info.SwapMB = swapStat.Total / (1024 * 1024)
		}
	})

	return c.info
}

// GetVirtualMemory returns detailed virtual memory statistics.
func (c *MemoryCollector) GetVirtualMemory() (*mem.VirtualMemoryStat, error) {
	return mem.VirtualMemory()
}

// GetSwapMemory returns detailed swap memory statistics.
func (c *MemoryCollector) GetSwapMemory() (*mem.SwapMemoryStat, error) {
	return mem.SwapMemory()
}

// GetAvailableMB returns the available memory in MB.
func (c *MemoryCollector) GetAvailableMB() (uint64, error) {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	return vmStat.Available / (1024 * 1024), nil
}

// GetFreeMB returns the free memory in MB.
func (c *MemoryCollector) GetFreeMB() (uint64, error) {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	return vmStat.Free / (1024 * 1024), nil
}
