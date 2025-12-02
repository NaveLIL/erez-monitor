//go:build windows

package collector

import (
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	dwmapi                          = syscall.NewLazyDLL("dwmapi.dll")
	procDwmGetCompositionTimingInfo = dwmapi.NewProc("DwmGetCompositionTimingInfo")
)

// DWM_TIMING_INFO structure (partial)
type DWM_TIMING_INFO struct {
	CbSize                uint32
	RateRefresh           UNSIGNED_RATIO
	QpcRefreshPeriod      uint64
	RateCompose           UNSIGNED_RATIO
	QpcVBlank             uint64
	CRefresh              uint64
	CRefreshNextPresent   uint64
	CFramesSubmitted      uint64
	CFrameConfirmed       uint64
	CFramesPending        uint64
	CFramesAvailable      uint64
	CFramesDropped        uint64
	CFramesMissed         uint64
	CRefreshNextDisplayed uint64
	CRefreshStarted       uint64
	CPixelsReceived       uint64
	CPixelsDrawn          uint64
	CBuffersEmpty         uint64
}

// UNSIGNED_RATIO structure
type UNSIGNED_RATIO struct {
	Numerator   uint32
	Denominator uint32
}

// FPSCollector monitors frame rate.
type FPSCollector struct {
	mu          sync.RWMutex
	currentFPS  float64
	lastUpdate  time.Time
	lastFrames  uint64
	initialized bool
}

// NewFPSCollector creates a new FPS collector.
func NewFPSCollector() *FPSCollector {
	return &FPSCollector{}
}

// Init initializes the FPS collector.
func (c *FPSCollector) Init() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if DWM is available
	if procDwmGetCompositionTimingInfo.Find() != nil {
		return nil
	}

	c.initialized = true
	c.lastUpdate = time.Now()
	return nil
}

// GetFPS returns the current estimated FPS.
func (c *FPSCollector) GetFPS() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return 0
	}

	// Get DWM timing info
	var info DWM_TIMING_INFO
	info.CbSize = uint32(unsafe.Sizeof(info))

	ret, _, _ := procDwmGetCompositionTimingInfo.Call(0, uintptr(unsafe.Pointer(&info)))
	if ret != 0 {
		return c.currentFPS
	}

	// Calculate FPS from frame count delta
	now := time.Now()
	elapsed := now.Sub(c.lastUpdate).Seconds()

	if elapsed >= 0.5 && c.lastFrames > 0 {
		frameDelta := info.CFramesSubmitted - c.lastFrames
		c.currentFPS = float64(frameDelta) / elapsed
		c.lastFrames = info.CFramesSubmitted
		c.lastUpdate = now
	} else if c.lastFrames == 0 {
		c.lastFrames = info.CFramesSubmitted
		c.lastUpdate = now
	}

	// Use refresh rate as fallback/validation
	if info.RateRefresh.Denominator > 0 {
		refreshRate := float64(info.RateRefresh.Numerator) / float64(info.RateRefresh.Denominator)
		if c.currentFPS <= 0 || c.currentFPS > refreshRate*2 {
			c.currentFPS = refreshRate
		}
	}

	return c.currentFPS
}

// IsAvailable returns whether FPS monitoring is available.
func (c *FPSCollector) IsAvailable() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.initialized
}
