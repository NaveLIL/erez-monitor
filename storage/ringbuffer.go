// Package storage provides thread-safe storage for metrics history.
package storage

import (
	"sync"
	"time"

	"github.com/NaveLIL/erez-monitor/models"
)

// RingBuffer is a thread-safe circular buffer for storing metrics history.
type RingBuffer struct {
	mu       sync.RWMutex
	data     []*models.Metrics
	size     int
	head     int // Index where the next element will be written
	count    int // Number of elements in the buffer
	capacity int
}

// NewRingBuffer creates a new RingBuffer with the specified capacity.
// The capacity determines how many metrics snapshots can be stored.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 60 // Default: 60 seconds of history
	}
	return &RingBuffer{
		data:     make([]*models.Metrics, capacity),
		capacity: capacity,
	}
}

// Add adds a new metrics snapshot to the buffer.
// If the buffer is full, the oldest entry is overwritten.
func (rb *RingBuffer) Add(metrics *models.Metrics) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	// Clone the metrics to avoid external modifications
	rb.data[rb.head] = metrics.Clone()
	rb.head = (rb.head + 1) % rb.capacity
	if rb.count < rb.capacity {
		rb.count++
	}
	rb.size = rb.count
}

// GetLast returns the last n metrics snapshots in chronological order.
// If n is greater than the number of stored snapshots, all snapshots are returned.
func (rb *RingBuffer) GetLast(n int) []*models.Metrics {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if n <= 0 || rb.count == 0 {
		return nil
	}

	if n > rb.count {
		n = rb.count
	}

	result := make([]*models.Metrics, n)

	// Calculate the starting index for the oldest of the n elements we want
	start := (rb.head - n + rb.capacity) % rb.capacity

	for i := 0; i < n; i++ {
		idx := (start + i) % rb.capacity
		result[i] = rb.data[idx].Clone()
	}

	return result
}

// GetLatest returns the most recent metrics snapshot.
// Returns nil if the buffer is empty.
func (rb *RingBuffer) GetLatest() *models.Metrics {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.count == 0 {
		return nil
	}

	idx := (rb.head - 1 + rb.capacity) % rb.capacity
	return rb.data[idx].Clone()
}

// GetAll returns all stored metrics in chronological order.
func (rb *RingBuffer) GetAll() []*models.Metrics {
	return rb.GetLast(rb.count)
}

// GetAverage calculates average metrics over the last specified number of seconds.
// Returns nil if no data is available.
func (rb *RingBuffer) GetAverage(seconds int) *models.Metrics {
	snapshots := rb.GetLast(seconds)
	if len(snapshots) == 0 {
		return nil
	}

	avg := &models.Metrics{
		Timestamp: time.Now(),
	}

	var (
		cpuSum         float64
		memUsedSum     uint64
		memPercentSum  float64
		gpuSum         float64
		gpuTempSum     uint32
		diskReadSum    float64
		diskWriteSum   float64
		netDownloadSum float64
		netUploadSum   float64
	)

	for _, m := range snapshots {
		cpuSum += m.CPU.UsagePercent
		memUsedSum += m.Memory.UsedMB
		memPercentSum += m.Memory.UsedPercent
		gpuSum += m.GPU.UsagePercent
		gpuTempSum += m.GPU.TemperatureC
		diskReadSum += m.Disk.ReadMBps
		diskWriteSum += m.Disk.WriteMBps
		netDownloadSum += m.Network.DownloadKBps
		netUploadSum += m.Network.UploadKBps
	}

	n := float64(len(snapshots))
	avg.CPU.UsagePercent = cpuSum / n
	avg.Memory.UsedMB = memUsedSum / uint64(len(snapshots))
	avg.Memory.UsedPercent = memPercentSum / n
	avg.Memory.TotalMB = snapshots[len(snapshots)-1].Memory.TotalMB
	avg.GPU.UsagePercent = gpuSum / n
	avg.GPU.TemperatureC = gpuTempSum / uint32(len(snapshots))
	avg.GPU.Available = snapshots[len(snapshots)-1].GPU.Available
	avg.Disk.ReadMBps = diskReadSum / n
	avg.Disk.WriteMBps = diskWriteSum / n
	avg.Network.DownloadKBps = netDownloadSum / n
	avg.Network.UploadKBps = netUploadSum / n

	return avg
}

// GetAverageByDuration calculates average metrics over the specified duration.
func (rb *RingBuffer) GetAverageByDuration(duration time.Duration) *models.Metrics {
	seconds := int(duration.Seconds())
	if seconds <= 0 {
		seconds = 1
	}
	return rb.GetAverage(seconds)
}

// GetMinMax returns the minimum and maximum values for key metrics over the last n seconds.
func (rb *RingBuffer) GetMinMax(seconds int) (min, max *models.Metrics) {
	snapshots := rb.GetLast(seconds)
	if len(snapshots) == 0 {
		return nil, nil
	}

	min = &models.Metrics{
		Timestamp: snapshots[0].Timestamp,
		CPU:       models.CPUMetrics{UsagePercent: 100},
		Memory:    models.MemoryMetrics{UsedPercent: 100},
		GPU:       models.GPUMetrics{UsagePercent: 100, TemperatureC: 200},
	}

	max = &models.Metrics{
		Timestamp: snapshots[len(snapshots)-1].Timestamp,
	}

	for _, m := range snapshots {
		// CPU
		if m.CPU.UsagePercent < min.CPU.UsagePercent {
			min.CPU.UsagePercent = m.CPU.UsagePercent
		}
		if m.CPU.UsagePercent > max.CPU.UsagePercent {
			max.CPU.UsagePercent = m.CPU.UsagePercent
		}

		// Memory
		if m.Memory.UsedPercent < min.Memory.UsedPercent {
			min.Memory.UsedPercent = m.Memory.UsedPercent
		}
		if m.Memory.UsedPercent > max.Memory.UsedPercent {
			max.Memory.UsedPercent = m.Memory.UsedPercent
		}

		// GPU
		if m.GPU.UsagePercent < min.GPU.UsagePercent {
			min.GPU.UsagePercent = m.GPU.UsagePercent
		}
		if m.GPU.UsagePercent > max.GPU.UsagePercent {
			max.GPU.UsagePercent = m.GPU.UsagePercent
		}
		if m.GPU.TemperatureC < min.GPU.TemperatureC {
			min.GPU.TemperatureC = m.GPU.TemperatureC
		}
		if m.GPU.TemperatureC > max.GPU.TemperatureC {
			max.GPU.TemperatureC = m.GPU.TemperatureC
		}
	}

	return min, max
}

// Clear removes all entries from the buffer.
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	for i := range rb.data {
		rb.data[i] = nil
	}
	rb.head = 0
	rb.count = 0
	rb.size = 0
}

// Size returns the number of elements currently in the buffer.
func (rb *RingBuffer) Size() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}

// Capacity returns the maximum capacity of the buffer.
func (rb *RingBuffer) Capacity() int {
	return rb.capacity
}

// IsFull returns true if the buffer has reached its capacity.
func (rb *RingBuffer) IsFull() bool {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count == rb.capacity
}

// IsEmpty returns true if the buffer has no elements.
func (rb *RingBuffer) IsEmpty() bool {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count == 0
}
