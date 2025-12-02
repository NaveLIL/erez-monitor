//go:build windows

package collector

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/NaveLIL/erez-monitor/logger"
	"github.com/NaveLIL/erez-monitor/models"
)

// AMDGPUCollector collects AMD GPU metrics via Windows Performance Counters.
type AMDGPUCollector struct {
	initialized bool
	mu          sync.Mutex
	log         *logger.Logger

	// Cached values
	lastMetrics models.GPUMetrics
	lastUpdate  time.Time
	updateMu    sync.RWMutex

	// GPU info
	gpuName     string
	vramTotalMB uint64
}

// NewAMDGPUCollector creates a new AMD GPU collector.
func NewAMDGPUCollector() *AMDGPUCollector {
	return &AMDGPUCollector{
		log: logger.Get(),
	}
}

// Init initializes the AMD GPU collector.
func (c *AMDGPUCollector) Init() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	// Check if any GPU exists via WMI
	gpuName, vram, err := c.detectGPU()
	if err != nil {
		c.log.Debugf("GPU not detected: %v", err)
		return err
	}

	c.gpuName = gpuName
	c.vramTotalMB = vram
	c.initialized = true
	c.log.Infof("GPU detected: %s (VRAM: %d MB)", gpuName, vram)

	// Start background update goroutine
	go c.backgroundUpdate()

	return nil
}

// detectGPU detects discrete GPU (AMD/NVIDIA) using WMI, skipping integrated Intel.
func (c *AMDGPUCollector) detectGPU() (string, uint64, error) {
	// First, try to find a discrete GPU (AMD or NVIDIA), skip Intel integrated
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		`Get-WmiObject Win32_VideoController | Where-Object { $_.Name -notmatch 'Intel' -and $_.Name -notmatch 'Microsoft' } | Select-Object -First 1 Name, AdapterRAM | ForEach-Object { "$($_.Name)|$($_.AdapterRAM)" }`)

	output, err := cmd.Output()
	if err == nil {
		parts := strings.Split(strings.TrimSpace(string(output)), "|")
		if len(parts) >= 2 && parts[0] != "" {
			name := parts[0]
			vram, _ := strconv.ParseUint(parts[1], 10, 64)
			vramMB := vram / (1024 * 1024)
			c.log.Debugf("Found discrete GPU: %s", name)
			return name, vramMB, nil
		}
	}

	// Fallback: try all GPUs and pick one with most VRAM (likely discrete)
	cmd = exec.Command("powershell", "-NoProfile", "-Command",
		`Get-WmiObject Win32_VideoController | Sort-Object AdapterRAM -Descending | Select-Object -First 1 Name, AdapterRAM | ForEach-Object { "$($_.Name)|$($_.AdapterRAM)" }`)

	output, err = cmd.Output()
	if err != nil {
		return "", 0, fmt.Errorf("WMI query failed: %w", err)
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(parts) < 2 || parts[0] == "" {
		return "", 0, fmt.Errorf("no GPU found")
	}

	name := parts[0]
	vram, _ := strconv.ParseUint(parts[1], 10, 64)
	vramMB := vram / (1024 * 1024)

	return name, vramMB, nil
}

// backgroundUpdate updates metrics in background every 1 second.
func (c *AMDGPUCollector) backgroundUpdate() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		if !c.initialized {
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()

		metrics := c.collectMetrics()

		c.updateMu.Lock()
		c.lastMetrics = metrics
		c.lastUpdate = time.Now()
		c.updateMu.Unlock()
	}
}

// collectMetrics gathers GPU metrics.
func (c *AMDGPUCollector) collectMetrics() models.GPUMetrics {
	metrics := models.GPUMetrics{
		Available:   true,
		Name:        c.gpuName,
		VRAMTotalMB: c.vramTotalMB,
	}

	// Get GPU usage via Windows Performance Counters (works for all GPUs)
	usage := c.getGPUUsage()
	metrics.UsagePercent = usage

	// Get VRAM usage
	vramUsed := c.getVRAMUsage()
	metrics.VRAMUsedMB = vramUsed

	// Get temperature (may not work on all systems)
	temp := c.getTemperature()
	metrics.TemperatureC = temp

	return metrics
}

// getGPUUsage gets GPU utilization using Performance Counters.
func (c *AMDGPUCollector) getGPUUsage() float64 {
	// Use PowerShell Get-Counter for reliable results
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		`$samples = (Get-Counter '\GPU Engine(*engtype_3D)\Utilization Percentage' -ErrorAction SilentlyContinue).CounterSamples; if($samples){($samples | Measure-Object -Property CookedValue -Maximum).Maximum}else{0}`)

	output, err := cmd.Output()
	if err != nil {
		return c.getGPUUsageFallback()
	}

	usageStr := strings.TrimSpace(string(output))
	usage, _ := strconv.ParseFloat(strings.Replace(usageStr, ",", ".", 1), 64)
	if usage > 100 {
		usage = 100
	}

	return usage
}

// getGPUUsageFallback uses sum of all 3D engines.
func (c *AMDGPUCollector) getGPUUsageFallback() float64 {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		`$samples = (Get-Counter '\GPU Engine(*)\Utilization Percentage' -ErrorAction SilentlyContinue).CounterSamples | Where-Object {$_.InstanceName -like '*3D*'}; if($samples){($samples | Measure-Object -Property CookedValue -Sum).Sum}else{0}`)

	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	usageStr := strings.TrimSpace(string(output))
	usage, _ := strconv.ParseFloat(strings.Replace(usageStr, ",", ".", 1), 64)
	if usage > 100 {
		usage = 100
	}

	return usage
}

// getVRAMUsage gets dedicated GPU memory usage.
func (c *AMDGPUCollector) getVRAMUsage() uint64 {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		`$samples = (Get-Counter '\GPU Process Memory(*)\Dedicated Usage' -ErrorAction SilentlyContinue).CounterSamples; if($samples){[math]::Round(($samples | Measure-Object -Property CookedValue -Sum).Sum / 1MB)}else{0}`)

	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	vramStr := strings.TrimSpace(string(output))
	vram, _ := strconv.ParseUint(vramStr, 10, 64)
	return vram
}

// getTemperature gets GPU temperature.
func (c *AMDGPUCollector) getTemperature() uint32 {
	// Try GPU temperature counter
	cmd := exec.Command("cmd", "/c", `typeperf "\GPU Local Adapter Memory(*)\Local Usage" -sc 1 -y 2>nul`)
	cmd.Output() // Just try, ignore errors

	// For AMD, try ACPI thermal zone
	cmd = exec.Command("powershell", "-NoProfile", "-Command",
		`$t = (Get-WmiObject -Namespace root\WMI -Class MSAcpi_ThermalZoneTemperature -ErrorAction SilentlyContinue | Select-Object -First 1).CurrentTemperature; if($t){[math]::Round(($t-2732)/10)}else{0}`)

	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	tempStr := strings.TrimSpace(string(output))
	temp, _ := strconv.ParseInt(tempStr, 10, 32)
	if temp < 0 || temp > 120 {
		return 0
	}

	return uint32(temp)
}

// Collect returns cached GPU metrics.
func (c *AMDGPUCollector) Collect() models.GPUMetrics {
	c.updateMu.RLock()
	defer c.updateMu.RUnlock()

	if !c.initialized {
		return models.GPUMetrics{Available: false}
	}

	// Return cached metrics
	return c.lastMetrics
}

// IsAvailable returns whether GPU monitoring is available.
func (c *AMDGPUCollector) IsAvailable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.initialized
}

// Shutdown cleans up resources.
func (c *AMDGPUCollector) Shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.initialized = false
}
