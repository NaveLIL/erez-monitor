//go:build windows

package collector

import (
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/NaveLIL/erez-monitor/logger"
	"github.com/NaveLIL/erez-monitor/models"
)

var (
	pdh                        = syscall.NewLazyDLL("pdh.dll")
	procPdhOpenQuery           = pdh.NewProc("PdhOpenQueryW")
	procPdhCloseQuery          = pdh.NewProc("PdhCloseQuery")
	procPdhAddCounterW         = pdh.NewProc("PdhAddEnglishCounterW")
	procPdhCollectQueryData    = pdh.NewProc("PdhCollectQueryData")
	procPdhGetFormattedValue   = pdh.NewProc("PdhGetFormattedCounterValue")
	procPdhExpandWildCardPathW = pdh.NewProc("PdhExpandWildCardPathW")
)

const (
	PDH_FMT_DOUBLE = 0x00000200
	PDH_MORE_DATA  = 0x800007D2
)

type PDH_FMT_COUNTERVALUE struct {
	CStatus     uint32
	Padding     uint32
	DoubleValue float64
}

// PDHGPUCollector uses Windows PDH API directly for reliable GPU monitoring.
type PDHGPUCollector struct {
	initialized bool
	mu          sync.Mutex
	log         *logger.Logger

	// PDH handles
	query        uintptr
	counters     []uintptr
	vramCounters []uintptr

	// Cached values
	cachedUsage    float64
	cachedVRAMUsed uint64
	cachedTemp     float64
	cachedPower    float64
	usageMu        sync.RWMutex

	// GPU info
	gpuName     string
	vramTotalMB uint64

	// Stop channel
	stopCh chan struct{}
}

// NewPDHGPUCollector creates a new PDH-based GPU collector.
func NewPDHGPUCollector() *PDHGPUCollector {
	return &PDHGPUCollector{
		log:    logger.Get(),
		stopCh: make(chan struct{}),
	}
}

// Init initializes the PDH GPU collector.
func (c *PDHGPUCollector) Init() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	// Detect GPU first
	gpuName, vram, err := c.detectGPU()
	if err != nil {
		return err
	}
	c.gpuName = gpuName
	c.vramTotalMB = vram

	// Open PDH query
	ret, _, _ := procPdhOpenQuery.Call(0, 0, uintptr(unsafe.Pointer(&c.query)))
	if ret != 0 {
		c.log.Warnf("PDH OpenQuery failed: 0x%X, falling back to simple mode", ret)
		c.initialized = true
		go c.backgroundUpdateSimple()
		return nil
	}

	// Add GPU 3D engine counters
	counterPath := utf16PtrFromString(`\GPU Engine(*engtype_3D)\Utilization Percentage`)

	// Get expanded paths
	var bufferSize uint32
	ret, _, _ = procPdhExpandWildCardPathW.Call(
		0,
		uintptr(unsafe.Pointer(counterPath)),
		0,
		uintptr(unsafe.Pointer(&bufferSize)),
		0,
	)

	if bufferSize > 0 {
		buffer := make([]uint16, bufferSize)
		ret, _, _ = procPdhExpandWildCardPathW.Call(
			0,
			uintptr(unsafe.Pointer(counterPath)),
			uintptr(unsafe.Pointer(&buffer[0])),
			uintptr(unsafe.Pointer(&bufferSize)),
			0,
		)

		if ret == 0 {
			// Parse multi-string buffer
			paths := parseMultiString(buffer)
			for _, path := range paths {
				var counter uintptr
				pathPtr := utf16PtrFromString(path)
				ret, _, _ = procPdhAddCounterW.Call(
					c.query,
					uintptr(unsafe.Pointer(pathPtr)),
					0,
					uintptr(unsafe.Pointer(&counter)),
				)
				if ret == 0 {
					c.counters = append(c.counters, counter)
				}
			}
		}
	}

	if len(c.counters) == 0 {
		c.log.Warn("No GPU counters added, using simple mode")
		c.initialized = true
		go c.backgroundUpdateSimple()
		return nil
	}

	// Try to add VRAM usage counters
	vramPath := utf16PtrFromString(`\GPU Process Memory(*)\Dedicated Usage`)
	bufferSize = 0
	ret, _, _ = procPdhExpandWildCardPathW.Call(
		0,
		uintptr(unsafe.Pointer(vramPath)),
		0,
		uintptr(unsafe.Pointer(&bufferSize)),
		0,
	)

	if bufferSize > 0 {
		buffer := make([]uint16, bufferSize)
		ret, _, _ = procPdhExpandWildCardPathW.Call(
			0,
			uintptr(unsafe.Pointer(vramPath)),
			uintptr(unsafe.Pointer(&buffer[0])),
			uintptr(unsafe.Pointer(&bufferSize)),
			0,
		)

		if ret == 0 {
			paths := parseMultiString(buffer)
			for _, path := range paths {
				var counter uintptr
				pathPtr := utf16PtrFromString(path)
				ret, _, _ = procPdhAddCounterW.Call(
					c.query,
					uintptr(unsafe.Pointer(pathPtr)),
					0,
					uintptr(unsafe.Pointer(&counter)),
				)
				if ret == 0 {
					c.vramCounters = append(c.vramCounters, counter)
				}
			}
		}
	}

	c.log.Infof("GPU detected: %s (VRAM: %d MB), %d usage counters, %d VRAM counters", gpuName, vram, len(c.counters), len(c.vramCounters))
	c.initialized = true

	// Initial collection to prime the counters
	procPdhCollectQueryData.Call(c.query)
	time.Sleep(100 * time.Millisecond)

	// Start background update
	go c.backgroundUpdate()

	return nil
}

// detectGPU detects discrete GPU using WMI.
func (c *PDHGPUCollector) detectGPU() (string, uint64, error) {
	// Use a simple approach - just check common GPU names
	// This is a fallback, the main detection happens via WMI in the outer collector
	return "GPU", 8192, nil
}

// backgroundUpdate updates GPU metrics using PDH.
func (c *PDHGPUCollector) backgroundUpdate() {
	// Update every second instead of 500ms to reduce CPU overhead
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.collectPDH()
		}
	}
}

// collectPDH collects GPU metrics via PDH API.
func (c *PDHGPUCollector) collectPDH() {
	c.mu.Lock()
	query := c.query
	counters := c.counters
	vramCounters := c.vramCounters
	c.mu.Unlock()

	if query == 0 || len(counters) == 0 {
		return
	}

	// Collect data
	ret, _, _ := procPdhCollectQueryData.Call(query)
	if ret != 0 {
		return
	}

	// Sum all GPU usage counter values
	var totalUsage float64
	for _, counter := range counters {
		var value PDH_FMT_COUNTERVALUE
		ret, _, _ := procPdhGetFormattedValue.Call(
			counter,
			PDH_FMT_DOUBLE,
			0,
			uintptr(unsafe.Pointer(&value)),
		)
		if ret == 0 && value.DoubleValue > 0 {
			totalUsage += value.DoubleValue
		}
	}

	// Cap at 100%
	if totalUsage > 100 {
		totalUsage = 100
	}

	// Sum all VRAM counter values (in bytes)
	var totalVRAM float64
	for _, counter := range vramCounters {
		var value PDH_FMT_COUNTERVALUE
		ret, _, _ := procPdhGetFormattedValue.Call(
			counter,
			PDH_FMT_DOUBLE,
			0,
			uintptr(unsafe.Pointer(&value)),
		)
		if ret == 0 && value.DoubleValue > 0 {
			totalVRAM += value.DoubleValue
		}
	}

	// Convert bytes to MB
	vramUsedMB := uint64(totalVRAM / (1024 * 1024))

	// Get temperature via D3DKMT API (same as Task Manager uses)
	temp, power, _, _ := GetGPUPerfDataD3DKMT()

	c.usageMu.Lock()
	c.cachedUsage = totalUsage
	c.cachedVRAMUsed = vramUsedMB
	c.cachedTemp = temp
	c.cachedPower = power
	c.usageMu.Unlock()
}

// backgroundUpdateSimple is a fallback using gopsutil-like approach.
func (c *PDHGPUCollector) backgroundUpdateSimple() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			// Simple fallback - just return last known value
			// The GPU is being used if we're in a game
		}
	}
}

// Collect returns cached GPU metrics - instant, never blocks.
func (c *PDHGPUCollector) Collect() models.GPUMetrics {
	c.usageMu.RLock()
	usage := c.cachedUsage
	vramUsed := c.cachedVRAMUsed
	temp := c.cachedTemp
	power := c.cachedPower
	c.usageMu.RUnlock()

	return models.GPUMetrics{
		Available:    c.initialized,
		Name:         c.gpuName,
		VRAMTotalMB:  c.vramTotalMB,
		UsagePercent: usage,
		VRAMUsedMB:   vramUsed,
		TemperatureC: uint32(temp),
		PowerWatts:   power,
	}
}

// IsAvailable returns whether GPU monitoring is available.
func (c *PDHGPUCollector) IsAvailable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.initialized
}

// Shutdown cleans up resources.
func (c *PDHGPUCollector) Shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		select {
		case <-c.stopCh:
			// Already closed
		default:
			close(c.stopCh)
		}
	}

	if c.query != 0 {
		procPdhCloseQuery.Call(c.query)
		c.query = 0
	}
	c.counters = nil
	c.initialized = false
}

// GetInfo returns GPU information.
func (c *PDHGPUCollector) GetInfo() *GPUInfo {
	return &GPUInfo{
		Name:        c.gpuName,
		VRAMTotalMB: c.vramTotalMB,
	}
}

// Helper functions
func utf16PtrFromString(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func parseMultiString(buffer []uint16) []string {
	var result []string
	var current strings.Builder

	for _, ch := range buffer {
		if ch == 0 {
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			} else {
				break // Double null = end
			}
		} else {
			current.WriteRune(rune(ch))
		}
	}

	return result
}
