package collector

import (
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/disk"

	"github.com/NaveLIL/erez-monitor/models"
)

// DiskCollector collects disk metrics.
type DiskCollector struct {
	lastIOCounters map[string]disk.IOCountersStat
	lastTime       time.Time
	mu             sync.Mutex
}

// NewDiskCollector creates a new disk collector.
func NewDiskCollector() *DiskCollector {
	return &DiskCollector{
		lastIOCounters: make(map[string]disk.IOCountersStat),
	}
}

// Collect gathers current disk metrics.
func (c *DiskCollector) Collect() models.DiskMetrics {
	metrics := models.DiskMetrics{
		Disks: make([]models.DiskInfo, 0),
	}

	// Get disk partitions
	partitions, err := disk.Partitions(false)
	if err == nil {
		for _, partition := range partitions {
			// Skip non-fixed drives (CD-ROM, etc.)
			if partition.Fstype == "" || partition.Fstype == "cdfs" {
				continue
			}

			usage, err := disk.Usage(partition.Mountpoint)
			if err != nil {
				continue
			}

			diskInfo := models.DiskInfo{
				Path:        partition.Mountpoint,
				FileSystem:  partition.Fstype,
				TotalGB:     usage.Total / (1024 * 1024 * 1024),
				UsedGB:      usage.Used / (1024 * 1024 * 1024),
				FreeGB:      usage.Free / (1024 * 1024 * 1024),
				UsedPercent: usage.UsedPercent,
			}
			metrics.Disks = append(metrics.Disks, diskInfo)
		}
	}

	// Get disk I/O statistics
	c.mu.Lock()
	defer c.mu.Unlock()

	ioCounters, err := disk.IOCounters()
	if err == nil && len(c.lastIOCounters) > 0 {
		now := time.Now()
		elapsed := now.Sub(c.lastTime).Seconds()

		if elapsed > 0 {
			var totalReadBytes, totalWriteBytes uint64
			var totalReadOps, totalWriteOps uint64

			for name, current := range ioCounters {
				if last, ok := c.lastIOCounters[name]; ok {
					totalReadBytes += current.ReadBytes - last.ReadBytes
					totalWriteBytes += current.WriteBytes - last.WriteBytes
					totalReadOps += current.ReadCount - last.ReadCount
					totalWriteOps += current.WriteCount - last.WriteCount
				}
			}

			// Convert to MB/s
			metrics.ReadMBps = float64(totalReadBytes) / elapsed / (1024 * 1024)
			metrics.WriteMBps = float64(totalWriteBytes) / elapsed / (1024 * 1024)

			// Calculate IOPS
			metrics.ReadIOPS = uint64(float64(totalReadOps) / elapsed)
			metrics.WriteIOPS = uint64(float64(totalWriteOps) / elapsed)
		}
	}

	// Store current counters for next calculation
	c.lastIOCounters = ioCounters
	c.lastTime = time.Now()

	return metrics
}

// GetPartitions returns all disk partitions.
func (c *DiskCollector) GetPartitions() ([]disk.PartitionStat, error) {
	return disk.Partitions(false)
}

// GetUsage returns disk usage for a specific path.
func (c *DiskCollector) GetUsage(path string) (*disk.UsageStat, error) {
	return disk.Usage(path)
}

// GetIOCounters returns raw I/O counters for all disks.
func (c *DiskCollector) GetIOCounters() (map[string]disk.IOCountersStat, error) {
	return disk.IOCounters()
}

// GetDiskInfo returns information about a specific disk.
func (c *DiskCollector) GetDiskInfo(path string) (*models.DiskInfo, error) {
	usage, err := disk.Usage(path)
	if err != nil {
		return nil, err
	}

	return &models.DiskInfo{
		Path:        path,
		TotalGB:     usage.Total / (1024 * 1024 * 1024),
		UsedGB:      usage.Used / (1024 * 1024 * 1024),
		FreeGB:      usage.Free / (1024 * 1024 * 1024),
		UsedPercent: usage.UsedPercent,
	}, nil
}
