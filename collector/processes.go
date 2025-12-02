package collector

import (
	"sort"

	"github.com/shirou/gopsutil/v3/process"

	"github.com/NaveLIL/erez-monitor/models"
)

// ProcessCollector collects process metrics.
type ProcessCollector struct {
	topCount int
}

// NewProcessCollector creates a new process collector.
func NewProcessCollector(topCount int) *ProcessCollector {
	if topCount <= 0 {
		topCount = 10
	}
	return &ProcessCollector{
		topCount: topCount,
	}
}

// Collect gathers current process metrics.
func (c *ProcessCollector) Collect() []models.ProcessInfo {
	processes, err := process.Processes()
	if err != nil {
		return nil
	}

	// Collect info for all processes
	processInfos := make([]models.ProcessInfo, 0, len(processes))

	for _, p := range processes {
		info := c.getProcessInfo(p)
		if info != nil {
			processInfos = append(processInfos, *info)
		}
	}

	// Sort by CPU usage (descending)
	sort.Slice(processInfos, func(i, j int) bool {
		return processInfos[i].CPUPercent > processInfos[j].CPUPercent
	})

	// Return top N processes
	if len(processInfos) > c.topCount {
		processInfos = processInfos[:c.topCount]
	}

	return processInfos
}

// getProcessInfo extracts information from a process.
// Optimized: only gets essential info to reduce CPU overhead.
func (c *ProcessCollector) getProcessInfo(p *process.Process) *models.ProcessInfo {
	name, err := p.Name()
	if err != nil {
		return nil
	}

	// Skip system processes that we can't access
	if name == "" || name == "System Idle Process" {
		return nil
	}

	info := &models.ProcessInfo{
		Name: name,
		PID:  p.Pid,
	}

	// Get CPU percent - this is the main metric we need
	cpuPercent, err := p.CPUPercent()
	if err == nil {
		info.CPUPercent = cpuPercent
	}

	// Get memory info - lightweight call
	memInfo, err := p.MemoryInfo()
	if err == nil && memInfo != nil {
		info.MemoryMB = memInfo.RSS / (1024 * 1024)
	}

	// Skip expensive calls (NumThreads, Status, MemoryPercent)
	// to reduce CPU overhead significantly

	return info
}

// GetTopByCPU returns the top N processes by CPU usage.
func (c *ProcessCollector) GetTopByCPU(n int) []models.ProcessInfo {
	processes, err := process.Processes()
	if err != nil {
		return nil
	}

	processInfos := make([]models.ProcessInfo, 0, len(processes))

	for _, p := range processes {
		info := c.getProcessInfo(p)
		if info != nil {
			processInfos = append(processInfos, *info)
		}
	}

	sort.Slice(processInfos, func(i, j int) bool {
		return processInfos[i].CPUPercent > processInfos[j].CPUPercent
	})

	if len(processInfos) > n {
		processInfos = processInfos[:n]
	}

	return processInfos
}

// GetTopByMemory returns the top N processes by memory usage.
func (c *ProcessCollector) GetTopByMemory(n int) []models.ProcessInfo {
	processes, err := process.Processes()
	if err != nil {
		return nil
	}

	processInfos := make([]models.ProcessInfo, 0, len(processes))

	for _, p := range processes {
		info := c.getProcessInfo(p)
		if info != nil {
			processInfos = append(processInfos, *info)
		}
	}

	sort.Slice(processInfos, func(i, j int) bool {
		return processInfos[i].MemoryMB > processInfos[j].MemoryMB
	})

	if len(processInfos) > n {
		processInfos = processInfos[:n]
	}

	return processInfos
}

// GetProcessByPID returns information about a specific process.
func (c *ProcessCollector) GetProcessByPID(pid int32) (*models.ProcessInfo, error) {
	p, err := process.NewProcess(pid)
	if err != nil {
		return nil, err
	}

	return c.getProcessInfo(p), nil
}

// GetAllProcesses returns information about all processes.
func (c *ProcessCollector) GetAllProcesses() ([]models.ProcessInfo, error) {
	processes, err := process.Processes()
	if err != nil {
		return nil, err
	}

	processInfos := make([]models.ProcessInfo, 0, len(processes))

	for _, p := range processes {
		info := c.getProcessInfo(p)
		if info != nil {
			processInfos = append(processInfos, *info)
		}
	}

	return processInfos, nil
}

// KillProcess terminates a process by PID.
func (c *ProcessCollector) KillProcess(pid int32) error {
	p, err := process.NewProcess(pid)
	if err != nil {
		return err
	}

	return p.Kill()
}

// GetProcessCount returns the total number of running processes.
func (c *ProcessCollector) GetProcessCount() (int, error) {
	processes, err := process.Processes()
	if err != nil {
		return 0, err
	}
	return len(processes), nil
}
