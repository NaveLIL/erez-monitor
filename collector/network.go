package collector

import (
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/net"

	"github.com/NaveLIL/erez-monitor/models"
)

// NetworkCollector collects network metrics.
type NetworkCollector struct {
	lastCounters []net.IOCountersStat
	lastTime     time.Time
	mu           sync.Mutex
	initialized  bool
}

// NewNetworkCollector creates a new network collector.
func NewNetworkCollector() *NetworkCollector {
	return &NetworkCollector{}
}

// Init initializes the network collector with the first reading.
func (c *NetworkCollector) Init() {
	c.mu.Lock()
	defer c.mu.Unlock()

	counters, err := net.IOCounters(true)
	if err == nil {
		c.lastCounters = counters
		c.lastTime = time.Now()
		c.initialized = true
	}
}

// Collect gathers current network metrics.
func (c *NetworkCollector) Collect() models.NetworkMetrics {
	metrics := models.NetworkMetrics{
		Interfaces: make([]models.InterfaceInfo, 0),
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Get current network I/O counters
	counters, err := net.IOCounters(true)
	if err != nil {
		return metrics
	}

	now := time.Now()
	elapsed := now.Sub(c.lastTime).Seconds()

	if c.initialized && elapsed > 0 {
		// Create map of previous counters for quick lookup
		lastMap := make(map[string]net.IOCountersStat)
		for _, counter := range c.lastCounters {
			lastMap[counter.Name] = counter
		}

		var totalBytesRecv, totalBytesSent uint64
		var totalPacketsRecv, totalPacketsSent uint64

		for _, current := range counters {
			// Skip loopback and virtual interfaces
			if isVirtualInterface(current.Name) {
				continue
			}

			if last, ok := lastMap[current.Name]; ok {
				bytesRecv := current.BytesRecv - last.BytesRecv
				bytesSent := current.BytesSent - last.BytesSent
				packetsRecv := current.PacketsRecv - last.PacketsRecv
				packetsSent := current.PacketsSent - last.PacketsSent

				totalBytesRecv += bytesRecv
				totalBytesSent += bytesSent
				totalPacketsRecv += packetsRecv
				totalPacketsSent += packetsSent

				// Per-interface metrics
				if bytesRecv > 0 || bytesSent > 0 {
					iface := models.InterfaceInfo{
						Name:         current.Name,
						DownloadKBps: float64(bytesRecv) / elapsed / 1024,
						UploadKBps:   float64(bytesSent) / elapsed / 1024,
						IsUp:         true,
					}
					metrics.Interfaces = append(metrics.Interfaces, iface)
				}
			}
		}

		// Calculate total rates
		metrics.DownloadKBps = float64(totalBytesRecv) / elapsed / 1024
		metrics.UploadKBps = float64(totalBytesSent) / elapsed / 1024
		metrics.DownloadBytes = totalBytesRecv
		metrics.UploadBytes = totalBytesSent
		metrics.PacketsRecv = uint64(float64(totalPacketsRecv) / elapsed)
		metrics.PacketsSent = uint64(float64(totalPacketsSent) / elapsed)
	}

	// Store current counters for next calculation
	c.lastCounters = counters
	c.lastTime = now
	c.initialized = true

	return metrics
}

// isVirtualInterface checks if an interface is virtual/loopback.
func isVirtualInterface(name string) bool {
	// Common virtual interface patterns on Windows
	virtualPrefixes := []string{
		"Loopback",
		"isatap",
		"Teredo",
		"6to4",
	}

	for _, prefix := range virtualPrefixes {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			return true
		}
	}

	return false
}

// GetIOCounters returns raw network I/O counters.
func (c *NetworkCollector) GetIOCounters(perNic bool) ([]net.IOCountersStat, error) {
	return net.IOCounters(perNic)
}

// GetInterfaces returns all network interfaces.
func (c *NetworkCollector) GetInterfaces() ([]net.InterfaceStat, error) {
	return net.Interfaces()
}

// GetConnections returns active network connections.
func (c *NetworkCollector) GetConnections(kind string) ([]net.ConnectionStat, error) {
	return net.Connections(kind)
}

// GetConnectionsByPID returns connections for a specific process.
func (c *NetworkCollector) GetConnectionsByPID(kind string, pid int32) ([]net.ConnectionStat, error) {
	conns, err := net.Connections(kind)
	if err != nil {
		return nil, err
	}

	var result []net.ConnectionStat
	for _, conn := range conns {
		if conn.Pid == pid {
			result = append(result, conn)
		}
	}

	return result, nil
}

// GetActiveInterfaceCount returns the number of active network interfaces.
func (c *NetworkCollector) GetActiveInterfaceCount() (int, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, iface := range interfaces {
		// Check if interface is up
		for _, flag := range iface.Flags {
			if flag == "up" {
				count++
				break
			}
		}
	}

	return count, nil
}
