package collector

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
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
// Uses PDH API for reliable Windows GPU monitoring.
type GPUCollector struct {
	info        *GPUInfo
	infoOnce    sync.Once
	initialized bool
	mu          sync.Mutex
	log         *logger.Logger

	// PDH-based collector (reliable)
	pdhCollector *PDHGPUCollector

	// GPU info detected at init
	gpuName     string
	vramTotalMB uint64
}

// NewGPUCollector creates a new GPU collector.
func NewGPUCollector() *GPUCollector {
	return &GPUCollector{
		log:          logger.Get(),
		pdhCollector: NewPDHGPUCollector(),
	}
}

// Init initializes the GPU collector.
func (c *GPUCollector) Init() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	// Detect GPU via WMI first
	gpuName, vram, err := c.detectGPU()
	if err != nil {
		c.log.Warnf("GPU detection failed: %v", err)
		return err
	}

	c.gpuName = gpuName
	c.vramTotalMB = vram
	c.log.Infof("GPU detected: %s (VRAM: %d MB)", gpuName, vram)

	// Initialize PDH collector
	if err := c.pdhCollector.Init(); err != nil {
		c.log.Warnf("PDH GPU collector failed: %v", err)
	}

	// Update PDH collector with detected GPU info
	c.pdhCollector.gpuName = gpuName
	c.pdhCollector.vramTotalMB = vram

	c.initialized = true
	c.log.Info("Using PDH GPU collector")
	return nil
}

// detectGPU detects discrete GPU using WMI.
func (c *GPUCollector) detectGPU() (string, uint64, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		`Get-CimInstance Win32_VideoController | Where-Object { $_.Name -notmatch 'Intel' -and $_.Name -notmatch 'Microsoft' } | Select-Object -First 1 Name, AdapterRAM | ForEach-Object { $vram = $_.AdapterRAM; if($vram -eq 4293918720 -or $vram -lt 4294967296){ $vram = 8589934592 }; "$($_.Name)|$vram" }`)

	output, err := cmd.Output()
	if err == nil {
		parts := strings.Split(strings.TrimSpace(string(output)), "|")
		if len(parts) >= 2 && parts[0] != "" {
			name := parts[0]
			vram, _ := strconv.ParseUint(parts[1], 10, 64)
			vramMB := vram / (1024 * 1024)
			if strings.Contains(name, "6650") || strings.Contains(name, "6700") || strings.Contains(name, "6800") || strings.Contains(name, "6900") {
				if vramMB < 8192 {
					vramMB = 8192
				}
			}
			return name, vramMB, nil
		}
	}

	return "", 0, fmt.Errorf("no discrete GPU found")
}

// Shutdown cleans up GPU resources.
func (c *GPUCollector) Shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.pdhCollector != nil {
		c.pdhCollector.Shutdown()
	}

	c.initialized = false
}

// Collect gathers current GPU metrics.
func (c *GPUCollector) Collect() models.GPUMetrics {
	if !c.initialized {
		return models.GPUMetrics{Available: false}
	}

	return c.pdhCollector.Collect()
}

// GetInfo returns static GPU information.
func (c *GPUCollector) GetInfo() *GPUInfo {
	c.infoOnce.Do(func() {
		c.info = &GPUInfo{
			Name:        c.gpuName,
			VRAMTotalMB: c.vramTotalMB,
			Vendor:      "AMD",
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
