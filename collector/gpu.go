package collector

import (
	"sync"

	"github.com/NaveLIL/erez-monitor/logger"
	"github.com/NaveLIL/erez-monitor/models"
)

// GPUInfo contains static GPU information.
type GPUInfo struct {
	Name        string
	VRAMTotalMB uint64
	DriverVer   string
	Vendor      string // "AMD", "NVIDIA", "Intel", "Unknown"
}

// GPUCollector collects GPU metrics.
// Supports both AMD and NVIDIA GPUs.
type GPUCollector struct {
	info        *GPUInfo
	infoOnce    sync.Once
	initialized bool
	mu          sync.Mutex
	log         *logger.Logger

	// AMD collector
	amdCollector *AMDGPUCollector
	useAMD       bool
}

// NewGPUCollector creates a new GPU collector.
func NewGPUCollector() *GPUCollector {
	return &GPUCollector{
		log:          logger.Get(),
		amdCollector: NewAMDGPUCollector(),
	}
}

// Init initializes the GPU collector.
func (c *GPUCollector) Init() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	// Try AMD first
	if err := c.amdCollector.Init(); err == nil {
		c.useAMD = true
		c.initialized = true
		c.log.Info("Using AMD GPU collector")
		return nil
	}

	// NVIDIA NVML would be tried here if available
	// For now, just log that no GPU was found
	c.log.Debug("No supported GPU found for monitoring")
	return nil
}

// Shutdown cleans up GPU resources.
func (c *GPUCollector) Shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.useAMD {
		c.amdCollector.Shutdown()
	}

	c.initialized = false
}

// Collect gathers current GPU metrics.
func (c *GPUCollector) Collect() models.GPUMetrics {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return models.GPUMetrics{Available: false}
	}

	if c.useAMD {
		return c.amdCollector.Collect()
	}

	return models.GPUMetrics{Available: false}
}

// GetInfo returns static GPU information.
func (c *GPUCollector) GetInfo() *GPUInfo {
	c.infoOnce.Do(func() {
		c.mu.Lock()
		defer c.mu.Unlock()

		if !c.initialized {
			return
		}

		c.info = &GPUInfo{}

		if c.useAMD {
			metrics := c.amdCollector.Collect()
			c.info.Name = metrics.Name
			c.info.VRAMTotalMB = metrics.VRAMTotalMB
			c.info.Vendor = "AMD"
		}
	})

	return c.info
}

// IsAvailable returns whether GPU monitoring is available.
func (c *GPUCollector) IsAvailable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.initialized
}
