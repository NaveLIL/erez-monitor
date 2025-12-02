//go:build windows

package collector

import (
	"net"
	"sync"
	"time"

	"github.com/NaveLIL/erez-monitor/logger"
)

// PingTarget represents a server to ping.
type PingTarget struct {
	Name    string // Display name (e.g., "Cloudflare", "Google")
	Host    string // Host to ping (IP or domain)
	Port    int    // TCP port to connect to
	Enabled bool   // Whether this target is enabled
}

// PingResult represents the result of a ping.
type PingResult struct {
	Name      string        // Target name
	Host      string        // Target host
	Latency   time.Duration // Round-trip latency
	Available bool          // Whether the host is reachable
	LastCheck time.Time     // When was the last check
}

// PingCollector measures network latency to various servers.
type PingCollector struct {
	mu          sync.RWMutex
	log         *logger.Logger
	initialized bool
	stopCh      chan struct{}

	// Ping targets
	targets []PingTarget

	// Cached results
	results map[string]*PingResult

	// Best (lowest) latency result
	bestLatency time.Duration
	bestTarget  string
}

// DefaultPingTargets returns common gaming and general servers to ping.
func DefaultPingTargets() []PingTarget {
	return []PingTarget{
		// DNS servers (always available, fast response)
		{Name: "Cloudflare", Host: "1.1.1.1", Port: 443, Enabled: true},
		{Name: "Google", Host: "8.8.8.8", Port: 443, Enabled: true},
		// Gaming platforms - EU servers
		{Name: "Steam EU", Host: "155.133.248.34", Port: 443, Enabled: true},
		{Name: "Riot EU", Host: "185.40.64.65", Port: 443, Enabled: true},
	}
}

// NewPingCollector creates a new ping collector.
func NewPingCollector() *PingCollector {
	return &PingCollector{
		log:     logger.Get(),
		targets: DefaultPingTargets(),
		results: make(map[string]*PingResult),
		stopCh:  make(chan struct{}),
	}
}

// Init initializes the ping collector.
func (c *PingCollector) Init() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	c.initialized = true

	// Start background ping loop
	go c.backgroundPing()

	c.log.Info("Ping collector initialized")
	return nil
}

// backgroundPing periodically pings all targets.
func (c *PingCollector) backgroundPing() {
	// Initial ping
	c.pingAll()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.pingAll()
		}
	}
}

// pingAll pings all enabled targets.
func (c *PingCollector) pingAll() {
	c.mu.RLock()
	targets := make([]PingTarget, len(c.targets))
	copy(targets, c.targets)
	c.mu.RUnlock()

	var wg sync.WaitGroup
	resultsCh := make(chan *PingResult, len(targets))

	for _, target := range targets {
		if !target.Enabled {
			continue
		}

		wg.Add(1)
		go func(t PingTarget) {
			defer wg.Done()
			result := c.pingTarget(t)
			resultsCh <- result
		}(target)
	}

	// Wait for all pings to complete
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	// Collect results
	newResults := make(map[string]*PingResult)
	var bestLatency time.Duration = time.Hour
	var bestTarget string

	for result := range resultsCh {
		newResults[result.Name] = result
		if result.Available && result.Latency < bestLatency {
			bestLatency = result.Latency
			bestTarget = result.Name
		}
	}

	// Update cached results
	c.mu.Lock()
	c.results = newResults
	c.bestLatency = bestLatency
	c.bestTarget = bestTarget
	c.mu.Unlock()
}

// pingTarget pings a single target using TCP connection.
func (c *PingCollector) pingTarget(target PingTarget) *PingResult {
	result := &PingResult{
		Name:      target.Name,
		Host:      target.Host,
		LastCheck: time.Now(),
	}

	// Use TCP connection to measure latency (works without admin rights)
	addr := net.JoinHostPort(target.Host, itoa(target.Port))

	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	latency := time.Since(start)

	if err != nil {
		result.Available = false
		result.Latency = 0
		return result
	}
	conn.Close()

	result.Available = true
	result.Latency = latency

	return result
}

// GetBestLatency returns the best (lowest) latency and target name.
func (c *PingCollector) GetBestLatency() (time.Duration, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bestLatency, c.bestTarget
}

// GetAllResults returns all ping results.
func (c *PingCollector) GetAllResults() map[string]*PingResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy
	results := make(map[string]*PingResult)
	for k, v := range c.results {
		copied := *v
		results[k] = &copied
	}
	return results
}

// GetResult returns the ping result for a specific target.
func (c *PingCollector) GetResult(name string) *PingResult {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if r, ok := c.results[name]; ok {
		copied := *r
		return &copied
	}
	return nil
}

// SetTargets updates the ping targets.
func (c *PingCollector) SetTargets(targets []PingTarget) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.targets = targets
}

// AddTarget adds a new ping target.
func (c *PingCollector) AddTarget(target PingTarget) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.targets = append(c.targets, target)
}

// IsInitialized returns whether the collector is initialized.
func (c *PingCollector) IsInitialized() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.initialized
}

// Shutdown stops the ping collector.
func (c *PingCollector) Shutdown() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		select {
		case <-c.stopCh:
			// Already closed
		default:
			close(c.stopCh)
		}
		c.initialized = false
	}
}

// itoa converts int to string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	var b [20]byte
	pos := len(b)
	neg := i < 0
	if neg {
		i = -i
	}

	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}

	if neg {
		pos--
		b[pos] = '-'
	}

	return string(b[pos:])
}
