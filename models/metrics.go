// Package models defines data structures for system metrics.
package models

import "time"

// Metrics represents a complete snapshot of system metrics at a given point in time.
type Metrics struct {
	Timestamp    time.Time      `json:"timestamp"`
	CPU          CPUMetrics     `json:"cpu"`
	Memory       MemoryMetrics  `json:"memory"`
	GPU          GPUMetrics     `json:"gpu"`
	Disk         DiskMetrics    `json:"disk"`
	Network      NetworkMetrics `json:"network"`
	TopProcesses []ProcessInfo  `json:"top_processes"`
}

// CPUMetrics contains CPU-related metrics.
type CPUMetrics struct {
	// UsagePercent is the overall CPU usage percentage (0-100).
	UsagePercent float64 `json:"usage_percent"`
	// PerCorePercent is the usage percentage for each CPU core.
	PerCorePercent []float64 `json:"per_core_percent"`
	// Temperature is the CPU temperature in Celsius (if available).
	Temperature float64 `json:"temperature"`
	// FrequencyMHz is the current CPU frequency in MHz.
	FrequencyMHz uint32 `json:"frequency_mhz"`
}

// MemoryMetrics contains RAM-related metrics.
type MemoryMetrics struct {
	// UsedMB is the amount of RAM used in megabytes.
	UsedMB uint64 `json:"used_mb"`
	// TotalMB is the total amount of RAM in megabytes.
	TotalMB uint64 `json:"total_mb"`
	// UsedPercent is the percentage of RAM used (0-100).
	UsedPercent float64 `json:"used_percent"`
	// SwapUsedMB is the amount of swap space used in megabytes.
	SwapUsedMB uint64 `json:"swap_used_mb"`
	// SwapTotalMB is the total swap space in megabytes.
	SwapTotalMB uint64 `json:"swap_total_mb"`
}

// GPUMetrics contains GPU-related metrics (NVIDIA GPUs via NVML).
type GPUMetrics struct {
	// Available indicates if GPU monitoring is available.
	Available bool `json:"available"`
	// Name is the GPU model name.
	Name string `json:"name"`
	// UsagePercent is the GPU utilization percentage (0-100).
	UsagePercent float64 `json:"usage_percent"`
	// TemperatureC is the GPU temperature in Celsius.
	TemperatureC uint32 `json:"temperature_c"`
	// VRAMUsedMB is the VRAM used in megabytes.
	VRAMUsedMB uint64 `json:"vram_used_mb"`
	// VRAMTotalMB is the total VRAM in megabytes.
	VRAMTotalMB uint64 `json:"vram_total_mb"`
	// ClockMHz is the current GPU clock frequency in MHz.
	ClockMHz uint32 `json:"clock_mhz"`
	// MemoryClockMHz is the current memory clock frequency in MHz.
	MemoryClockMHz uint32 `json:"memory_clock_mhz"`
	// PowerWatts is the current power draw in watts.
	PowerWatts float64 `json:"power_watts"`
	// FanSpeedPercent is the fan speed percentage.
	FanSpeedPercent uint32 `json:"fan_speed_percent"`
}

// DiskMetrics contains disk I/O metrics.
type DiskMetrics struct {
	// ReadMBps is the disk read speed in MB/s.
	ReadMBps float64 `json:"read_mbps"`
	// WriteMBps is the disk write speed in MB/s.
	WriteMBps float64 `json:"write_mbps"`
	// ReadIOPS is the number of read operations per second.
	ReadIOPS uint64 `json:"read_iops"`
	// WriteIOPS is the number of write operations per second.
	WriteIOPS uint64 `json:"write_iops"`
	// Disks contains information about each disk partition.
	Disks []DiskInfo `json:"disks"`
}

// DiskInfo contains information about a single disk partition.
type DiskInfo struct {
	// Path is the mount point (e.g., "C:\").
	Path string `json:"path"`
	// FileSystem is the file system type (e.g., "NTFS").
	FileSystem string `json:"file_system"`
	// UsedGB is the used space in gigabytes.
	UsedGB uint64 `json:"used_gb"`
	// TotalGB is the total space in gigabytes.
	TotalGB uint64 `json:"total_gb"`
	// FreeGB is the free space in gigabytes.
	FreeGB uint64 `json:"free_gb"`
	// UsedPercent is the percentage of disk space used.
	UsedPercent float64 `json:"used_percent"`
}

// NetworkMetrics contains network I/O metrics.
type NetworkMetrics struct {
	// DownloadKBps is the download speed in KB/s.
	DownloadKBps float64 `json:"download_kbps"`
	// UploadKBps is the upload speed in KB/s.
	UploadKBps float64 `json:"upload_kbps"`
	// DownloadBytes is the total bytes received since last measurement.
	DownloadBytes uint64 `json:"download_bytes"`
	// UploadBytes is the total bytes sent since last measurement.
	UploadBytes uint64 `json:"upload_bytes"`
	// PacketsRecv is the number of packets received per second.
	PacketsRecv uint64 `json:"packets_recv"`
	// PacketsSent is the number of packets sent per second.
	PacketsSent uint64 `json:"packets_sent"`
	// Interfaces contains per-interface metrics.
	Interfaces []InterfaceInfo `json:"interfaces"`
}

// InterfaceInfo contains information about a single network interface.
type InterfaceInfo struct {
	// Name is the interface name.
	Name string `json:"name"`
	// DownloadKBps is the download speed for this interface in KB/s.
	DownloadKBps float64 `json:"download_kbps"`
	// UploadKBps is the upload speed for this interface in KB/s.
	UploadKBps float64 `json:"upload_kbps"`
	// IsUp indicates if the interface is active.
	IsUp bool `json:"is_up"`
}

// ProcessInfo contains information about a running process.
type ProcessInfo struct {
	// Name is the process name.
	Name string `json:"name"`
	// PID is the process ID.
	PID int32 `json:"pid"`
	// CPUPercent is the CPU usage percentage of this process.
	CPUPercent float64 `json:"cpu_percent"`
	// MemoryMB is the memory usage in megabytes.
	MemoryMB uint64 `json:"memory_mb"`
	// MemoryPercent is the memory usage percentage.
	MemoryPercent float64 `json:"memory_percent"`
	// Threads is the number of threads.
	Threads int32 `json:"threads"`
	// Status is the process status (running, sleeping, etc.).
	Status string `json:"status"`
}

// AlertType represents the type of alert.
type AlertType string

const (
	AlertTypeCPU     AlertType = "cpu"
	AlertTypeRAM     AlertType = "ram"
	AlertTypeGPU     AlertType = "gpu"
	AlertTypeDisk    AlertType = "disk"
	AlertTypeNetwork AlertType = "network"
)

// Alert represents a system alert when a threshold is exceeded.
type Alert struct {
	// Type is the alert type (cpu, ram, gpu, disk, network).
	Type AlertType `json:"type"`
	// Timestamp is when the alert was triggered.
	Timestamp time.Time `json:"timestamp"`
	// Message is the alert message.
	Message string `json:"message"`
	// Value is the current value that triggered the alert.
	Value float64 `json:"value"`
	// Threshold is the threshold that was exceeded.
	Threshold float64 `json:"threshold"`
}

// SystemInfo contains static system information.
type SystemInfo struct {
	// Hostname is the computer name.
	Hostname string `json:"hostname"`
	// OS is the operating system name and version.
	OS string `json:"os"`
	// Platform is the platform (windows, linux, darwin).
	Platform string `json:"platform"`
	// CPUModel is the CPU model name.
	CPUModel string `json:"cpu_model"`
	// CPUCores is the number of physical CPU cores.
	CPUCores int `json:"cpu_cores"`
	// CPUThreads is the number of logical CPU threads.
	CPUThreads int `json:"cpu_threads"`
	// TotalRAM is the total RAM in MB.
	TotalRAM uint64 `json:"total_ram"`
	// GPUName is the GPU model name (if available).
	GPUName string `json:"gpu_name"`
}

// NewMetrics creates a new Metrics instance with the current timestamp.
func NewMetrics() *Metrics {
	return &Metrics{
		Timestamp:    time.Now(),
		TopProcesses: make([]ProcessInfo, 0, 10),
	}
}

// Clone creates a deep copy of the Metrics.
func (m *Metrics) Clone() *Metrics {
	clone := &Metrics{
		Timestamp: m.Timestamp,
		CPU:       m.CPU,
		Memory:    m.Memory,
		GPU:       m.GPU,
		Disk:      m.Disk,
		Network:   m.Network,
	}

	// Deep copy slices
	if m.CPU.PerCorePercent != nil {
		clone.CPU.PerCorePercent = make([]float64, len(m.CPU.PerCorePercent))
		copy(clone.CPU.PerCorePercent, m.CPU.PerCorePercent)
	}

	if m.Disk.Disks != nil {
		clone.Disk.Disks = make([]DiskInfo, len(m.Disk.Disks))
		copy(clone.Disk.Disks, m.Disk.Disks)
	}

	if m.Network.Interfaces != nil {
		clone.Network.Interfaces = make([]InterfaceInfo, len(m.Network.Interfaces))
		copy(clone.Network.Interfaces, m.Network.Interfaces)
	}

	if m.TopProcesses != nil {
		clone.TopProcesses = make([]ProcessInfo, len(m.TopProcesses))
		copy(clone.TopProcesses, m.TopProcesses)
	}

	return clone
}
