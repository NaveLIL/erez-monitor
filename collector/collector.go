// Package collector provides system metrics collection functionality.
package collector

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/NaveLIL/erez-monitor/config"
	"github.com/NaveLIL/erez-monitor/logger"
	"github.com/NaveLIL/erez-monitor/models"
	"github.com/NaveLIL/erez-monitor/storage"
)

// MetricsCollector defines the interface for collecting system metrics.
type MetricsCollector interface {
	// Start begins collecting metrics at the configured interval.
	Start(ctx context.Context) error
	// Stop stops the collector.
	Stop()
	// GetLatest returns the most recent metrics.
	GetLatest() *models.Metrics
	// GetHistory returns the metrics storage buffer.
	GetHistory() *storage.RingBuffer
	// Subscribe adds a channel to receive metrics updates.
	Subscribe(ch chan<- *models.Metrics)
	// Unsubscribe removes a channel from receiving updates.
	Unsubscribe(ch chan<- *models.Metrics)
}

// Collector collects system metrics at regular intervals.
type Collector struct {
	config      *config.MonitoringConfig
	storage     *storage.RingBuffer
	log         *logger.Logger
	subscribers []chan<- *models.Metrics
	subMu       sync.RWMutex

	// Sub-collectors
	cpuCollector     *CPUCollector
	memoryCollector  *MemoryCollector
	gpuCollector     *GPUCollector
	diskCollector    *DiskCollector
	networkCollector *NetworkCollector
	processCollector *ProcessCollector

	// State
	running bool
	mu      sync.RWMutex
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	// Latest metrics cache - atomic pointer for lock-free reads
	latestPtr unsafe.Pointer // *models.Metrics
}

// New creates a new Collector with the given configuration.
func New(cfg *config.MonitoringConfig) *Collector {
	historySeconds := int(cfg.HistoryDuration.Seconds())
	if historySeconds <= 0 {
		historySeconds = 60
	}

	c := &Collector{
		config:  cfg,
		storage: storage.NewRingBuffer(historySeconds),
		log:     logger.Get(),
	}

	// Initialize sub-collectors
	c.cpuCollector = NewCPUCollector()
	c.memoryCollector = NewMemoryCollector()
	c.diskCollector = NewDiskCollector()
	c.networkCollector = NewNetworkCollector()
	c.processCollector = NewProcessCollector(cfg.TopProcessCount)

	if cfg.EnableGPU {
		c.gpuCollector = NewGPUCollector()
	}

	return c
}

// Start begins collecting metrics at the configured interval.
func (c *Collector) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = true
	c.mu.Unlock()

	// Create a cancellable context
	ctx, c.cancel = context.WithCancel(ctx)

	// Initialize GPU if enabled
	if c.gpuCollector != nil {
		if err := c.gpuCollector.Init(); err != nil {
			c.log.Warnf("GPU monitoring unavailable: %v", err)
			c.gpuCollector = nil
		} else {
			c.log.Info("GPU monitoring initialized")
		}
	}

	// Initialize network collector with first reading
	c.networkCollector.Init()

	// Initial collection
	c.collect()

	// Start collection goroutine
	c.wg.Add(1)
	go c.collectionLoop(ctx)

	c.log.Infof("Collector started with %v interval", c.config.UpdateInterval)
	return nil
}

// Stop stops the collector.
func (c *Collector) Stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	c.running = false
	c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}

	// Wait for collection goroutine to finish
	c.wg.Wait()

	// Cleanup GPU
	if c.gpuCollector != nil {
		c.gpuCollector.Shutdown()
	}

	c.log.Info("Collector stopped")
}

// collectionLoop runs the metrics collection at regular intervals.
func (c *Collector) collectionLoop(ctx context.Context) {
	defer c.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			c.log.Errorf("Panic in collection loop: %v", r)
		}
	}()

	ticker := time.NewTicker(c.config.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.collect()
		}
	}
}

// collect gathers all metrics and stores them.
func (c *Collector) collect() {
	metrics := models.NewMetrics()

	// Use timeout for all collection - never block more than 800ms
	done := make(chan struct{})

	go func() {
		var wg sync.WaitGroup

		// Collect CPU metrics
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer recoverPanic("CPU")
			metrics.CPU = c.cpuCollector.Collect()
		}()

		// Collect memory metrics
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer recoverPanic("Memory")
			metrics.Memory = c.memoryCollector.Collect()
		}()

		// Collect GPU metrics - already non-blocking (returns cached)
		if c.gpuCollector != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer recoverPanic("GPU")
				metrics.GPU = c.gpuCollector.Collect()
			}()
		}

		// Collect disk metrics
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer recoverPanic("Disk")
			metrics.Disk = c.diskCollector.Collect()
		}()

		// Collect network metrics
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer recoverPanic("Network")
			metrics.Network = c.networkCollector.Collect()
		}()

		// Collect process metrics
		if c.config.EnableProcesses {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer recoverPanic("Processes")
				metrics.TopProcesses = c.processCollector.Collect()
			}()
		}

		wg.Wait()
		close(done)
	}()

	// Wait with timeout
	select {
	case <-done:
		// All collectors finished
	case <-time.After(800 * time.Millisecond):
		// Timeout - use partial metrics
		c.log.Debug("Collection timeout, using partial metrics")
	}

	// Store metrics
	c.storage.Add(metrics)

	// Update latest cache atomically - no locks!
	atomic.StorePointer(&c.latestPtr, unsafe.Pointer(metrics))

	// Notify subscribers
	c.notifySubscribers(metrics)
}

// recoverPanic recovers from panics in collection goroutines.
func recoverPanic(component string) {
	if r := recover(); r != nil {
		logger.Get().Errorf("Panic in %s collector: %v", component, r)
	}
}

// notifySubscribers sends the metrics to all subscribed channels.
func (c *Collector) notifySubscribers(metrics *models.Metrics) {
	c.subMu.RLock()
	defer c.subMu.RUnlock()

	for _, ch := range c.subscribers {
		select {
		case ch <- metrics:
		default:
			// Channel full, skip
		}
	}
}

// GetLatest returns the most recent metrics.
// Returns pointer to internal struct - do not modify!
// Uses atomic load - completely lock-free for overlay!
func (c *Collector) GetLatest() *models.Metrics {
	ptr := atomic.LoadPointer(&c.latestPtr)
	if ptr == nil {
		return nil
	}
	return (*models.Metrics)(ptr)
}

// GetHistory returns the metrics storage buffer.
func (c *Collector) GetHistory() *storage.RingBuffer {
	return c.storage
}

// Subscribe adds a channel to receive metrics updates.
func (c *Collector) Subscribe(ch chan<- *models.Metrics) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	c.subscribers = append(c.subscribers, ch)
}

// Unsubscribe removes a channel from receiving updates.
func (c *Collector) Unsubscribe(ch chan<- *models.Metrics) {
	c.subMu.Lock()
	defer c.subMu.Unlock()

	for i, sub := range c.subscribers {
		if sub == ch {
			c.subscribers = append(c.subscribers[:i], c.subscribers[i+1:]...)
			return
		}
	}
}

// IsRunning returns whether the collector is running.
func (c *Collector) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

// GetSystemInfo returns static system information.
func (c *Collector) GetSystemInfo() *models.SystemInfo {
	info := &models.SystemInfo{}

	// Get CPU info
	if cpuInfo := c.cpuCollector.GetInfo(); cpuInfo != nil {
		info.CPUModel = cpuInfo.Model
		info.CPUCores = cpuInfo.Cores
		info.CPUThreads = cpuInfo.Threads
	}

	// Get memory info
	if memInfo := c.memoryCollector.GetInfo(); memInfo != nil {
		info.TotalRAM = memInfo.TotalMB
	}

	// Get GPU info
	if c.gpuCollector != nil {
		if gpuInfo := c.gpuCollector.GetInfo(); gpuInfo != nil {
			info.GPUName = gpuInfo.Name
		}
	}

	return info
}
