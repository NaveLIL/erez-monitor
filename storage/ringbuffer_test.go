package storage

import (
	"sync"
	"testing"
	"time"

	"github.com/NaveLIL/erez-monitor/models"
)

func TestNewRingBuffer(t *testing.T) {
	rb := NewRingBuffer(60)
	if rb.Capacity() != 60 {
		t.Errorf("Expected capacity 60, got %d", rb.Capacity())
	}
	if rb.Size() != 0 {
		t.Errorf("Expected size 0, got %d", rb.Size())
	}
	if !rb.IsEmpty() {
		t.Error("Expected buffer to be empty")
	}

	// Test default capacity
	rb2 := NewRingBuffer(0)
	if rb2.Capacity() != 60 {
		t.Errorf("Expected default capacity 60, got %d", rb2.Capacity())
	}
}

func TestAdd(t *testing.T) {
	rb := NewRingBuffer(5)

	// Add first element
	m1 := createTestMetrics(50.0, 60.0)
	rb.Add(m1)

	if rb.Size() != 1 {
		t.Errorf("Expected size 1, got %d", rb.Size())
	}

	// Add more elements
	for i := 0; i < 4; i++ {
		rb.Add(createTestMetrics(float64(i*10), float64(i*5)))
	}

	if rb.Size() != 5 {
		t.Errorf("Expected size 5, got %d", rb.Size())
	}

	if !rb.IsFull() {
		t.Error("Expected buffer to be full")
	}

	// Add one more (should overwrite oldest)
	rb.Add(createTestMetrics(99.0, 99.0))

	if rb.Size() != 5 {
		t.Errorf("Expected size 5 after overflow, got %d", rb.Size())
	}

	// Check that the newest is 99%
	latest := rb.GetLatest()
	if latest.CPU.UsagePercent != 99.0 {
		t.Errorf("Expected latest CPU 99, got %f", latest.CPU.UsagePercent)
	}
}

func TestGetLast(t *testing.T) {
	rb := NewRingBuffer(10)

	// Add 5 elements with CPU usage 10, 20, 30, 40, 50
	for i := 1; i <= 5; i++ {
		rb.Add(createTestMetrics(float64(i*10), float64(i*5)))
	}

	// Get last 3
	results := rb.GetLast(3)
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// Should be 30, 40, 50 (in order)
	expected := []float64{30, 40, 50}
	for i, m := range results {
		if m.CPU.UsagePercent != expected[i] {
			t.Errorf("Expected CPU %f at index %d, got %f", expected[i], i, m.CPU.UsagePercent)
		}
	}

	// Test getting more than available
	results = rb.GetLast(100)
	if len(results) != 5 {
		t.Errorf("Expected 5 results when requesting more than available, got %d", len(results))
	}

	// Test getting zero
	results = rb.GetLast(0)
	if results != nil {
		t.Errorf("Expected nil for GetLast(0), got %v", results)
	}
}

func TestGetLatest(t *testing.T) {
	rb := NewRingBuffer(5)

	// Empty buffer
	if rb.GetLatest() != nil {
		t.Error("Expected nil for empty buffer")
	}

	// Add one element
	rb.Add(createTestMetrics(25.0, 50.0))
	latest := rb.GetLatest()
	if latest == nil {
		t.Fatal("Expected non-nil latest")
	}
	if latest.CPU.UsagePercent != 25.0 {
		t.Errorf("Expected CPU 25, got %f", latest.CPU.UsagePercent)
	}
}

func TestGetAll(t *testing.T) {
	rb := NewRingBuffer(5)

	for i := 1; i <= 3; i++ {
		rb.Add(createTestMetrics(float64(i*10), 50.0))
	}

	all := rb.GetAll()
	if len(all) != 3 {
		t.Errorf("Expected 3 elements, got %d", len(all))
	}
}

func TestGetAverage(t *testing.T) {
	rb := NewRingBuffer(10)

	// Add 5 elements: CPU 10, 20, 30, 40, 50 (average = 30)
	for i := 1; i <= 5; i++ {
		rb.Add(createTestMetrics(float64(i*10), 50.0))
	}

	avg := rb.GetAverage(5)
	if avg == nil {
		t.Fatal("Expected non-nil average")
	}

	// Average of 10, 20, 30, 40, 50 = 30
	if avg.CPU.UsagePercent != 30.0 {
		t.Errorf("Expected average CPU 30, got %f", avg.CPU.UsagePercent)
	}

	// Test partial average (last 3: 30, 40, 50 = 40)
	avg = rb.GetAverage(3)
	expectedAvg := (30.0 + 40.0 + 50.0) / 3
	if avg.CPU.UsagePercent != expectedAvg {
		t.Errorf("Expected average CPU %f, got %f", expectedAvg, avg.CPU.UsagePercent)
	}
}

func TestGetAverageByDuration(t *testing.T) {
	rb := NewRingBuffer(10)

	for i := 1; i <= 5; i++ {
		rb.Add(createTestMetrics(float64(i*10), 50.0))
	}

	avg := rb.GetAverageByDuration(3 * time.Second)
	if avg == nil {
		t.Fatal("Expected non-nil average")
	}
}

func TestGetMinMax(t *testing.T) {
	rb := NewRingBuffer(10)

	// Add elements with CPU: 20, 50, 30, 80, 10
	cpuValues := []float64{20, 50, 30, 80, 10}
	for _, cpu := range cpuValues {
		rb.Add(createTestMetrics(cpu, 50.0))
	}

	min, max := rb.GetMinMax(5)
	if min == nil || max == nil {
		t.Fatal("Expected non-nil min and max")
	}

	if min.CPU.UsagePercent != 10.0 {
		t.Errorf("Expected min CPU 10, got %f", min.CPU.UsagePercent)
	}
	if max.CPU.UsagePercent != 80.0 {
		t.Errorf("Expected max CPU 80, got %f", max.CPU.UsagePercent)
	}
}

func TestClear(t *testing.T) {
	rb := NewRingBuffer(5)

	for i := 0; i < 3; i++ {
		rb.Add(createTestMetrics(float64(i*10), 50.0))
	}

	if rb.Size() != 3 {
		t.Errorf("Expected size 3, got %d", rb.Size())
	}

	rb.Clear()

	if rb.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", rb.Size())
	}
	if !rb.IsEmpty() {
		t.Error("Expected buffer to be empty after clear")
	}
	if rb.GetLatest() != nil {
		t.Error("Expected nil after clear")
	}
}

func TestConcurrentAccess(t *testing.T) {
	rb := NewRingBuffer(100)
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				rb.Add(createTestMetrics(float64(id*10+j), 50.0))
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = rb.GetLatest()
				_ = rb.GetLast(10)
				_ = rb.Size()
			}
		}()
	}

	wg.Wait()

	// Should not panic, buffer should be full
	if rb.Size() != 100 {
		t.Errorf("Expected size 100, got %d", rb.Size())
	}
}

func TestOverflow(t *testing.T) {
	rb := NewRingBuffer(3)

	// Add 5 elements (overflow by 2)
	for i := 1; i <= 5; i++ {
		rb.Add(createTestMetrics(float64(i*10), 50.0))
	}

	// Should have 3 elements: 30, 40, 50
	if rb.Size() != 3 {
		t.Errorf("Expected size 3, got %d", rb.Size())
	}

	all := rb.GetAll()
	expected := []float64{30, 40, 50}
	for i, m := range all {
		if m.CPU.UsagePercent != expected[i] {
			t.Errorf("Expected CPU %f at index %d, got %f", expected[i], i, m.CPU.UsagePercent)
		}
	}
}

func TestClone(t *testing.T) {
	rb := NewRingBuffer(5)

	original := createTestMetrics(50.0, 60.0)
	rb.Add(original)

	// Modify original after adding
	original.CPU.UsagePercent = 99.0

	// Retrieved value should still be 50
	retrieved := rb.GetLatest()
	if retrieved.CPU.UsagePercent != 50.0 {
		t.Errorf("Clone not working, expected 50, got %f", retrieved.CPU.UsagePercent)
	}

	// Modify retrieved value
	retrieved.CPU.UsagePercent = 1.0

	// Buffer value should still be 50
	retrieved2 := rb.GetLatest()
	if retrieved2.CPU.UsagePercent != 50.0 {
		t.Errorf("Clone on read not working, expected 50, got %f", retrieved2.CPU.UsagePercent)
	}
}

func BenchmarkAdd(b *testing.B) {
	rb := NewRingBuffer(60)
	m := createTestMetrics(50.0, 60.0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.Add(m)
	}
}

func BenchmarkGetLatest(b *testing.B) {
	rb := NewRingBuffer(60)
	for i := 0; i < 60; i++ {
		rb.Add(createTestMetrics(float64(i), 50.0))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rb.GetLatest()
	}
}

func BenchmarkGetLast(b *testing.B) {
	rb := NewRingBuffer(60)
	for i := 0; i < 60; i++ {
		rb.Add(createTestMetrics(float64(i), 50.0))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rb.GetLast(30)
	}
}

func BenchmarkConcurrentReadWrite(b *testing.B) {
	rb := NewRingBuffer(60)
	for i := 0; i < 60; i++ {
		rb.Add(createTestMetrics(float64(i), 50.0))
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rb.Add(createTestMetrics(50.0, 60.0))
			_ = rb.GetLatest()
		}
	})
}

// Helper function to create test metrics
func createTestMetrics(cpuPercent, memPercent float64) *models.Metrics {
	return &models.Metrics{
		Timestamp: time.Now(),
		CPU: models.CPUMetrics{
			UsagePercent:   cpuPercent,
			PerCorePercent: []float64{cpuPercent, cpuPercent},
			Temperature:    50.0,
		},
		Memory: models.MemoryMetrics{
			UsedMB:      8192,
			TotalMB:     16384,
			UsedPercent: memPercent,
		},
		GPU: models.GPUMetrics{
			Available:    true,
			UsagePercent: 50.0,
			TemperatureC: 65,
		},
		Disk: models.DiskMetrics{
			ReadMBps:  100.0,
			WriteMBps: 50.0,
		},
		Network: models.NetworkMetrics{
			DownloadKBps: 1000.0,
			UploadKBps:   500.0,
		},
		TopProcesses: []models.ProcessInfo{
			{Name: "test.exe", PID: 1234, CPUPercent: 10.0, MemoryMB: 256},
		},
	}
}
