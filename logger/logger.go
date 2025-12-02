// Package logger provides structured logging and CSV export functionality.
package logger

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/NaveLIL/erez-monitor/config"
	"github.com/NaveLIL/erez-monitor/models"
)

// Logger is the application logger with CSV export support.
type Logger struct {
	*logrus.Logger
	csvWriter   *csv.Writer
	csvFile     *os.File
	csvMu       sync.Mutex
	logFile     *lumberjack.Logger
	config      *config.LoggingConfig
	initialized bool
}

var (
	instance *Logger
	once     sync.Once
)

// Get returns the singleton logger instance.
func Get() *Logger {
	once.Do(func() {
		instance = &Logger{
			Logger: logrus.New(),
		}
	})
	return instance
}

// Init initializes the logger with the provided configuration.
func (l *Logger) Init(cfg *config.LoggingConfig, configDir string) error {
	l.config = cfg

	// Set log level
	level, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		level = logrus.InfoLevel
	}
	l.SetLevel(level)

	// Set formatter
	l.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		ForceColors:     true,
	})

	// Set output
	if cfg.ToFile {
		logPath := cfg.FilePath
		if !filepath.IsAbs(logPath) {
			logPath = filepath.Join(configDir, logPath)
		}

		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}

		// Parse max file size
		maxSize := 10 // Default 10 MB
		if cfg.MaxFileSize != "" {
			fmt.Sscanf(cfg.MaxFileSize, "%dMB", &maxSize)
		}

		l.logFile = &lumberjack.Logger{
			Filename:   logPath,
			MaxSize:    maxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   true,
		}

		// Write to both file and stdout
		l.SetOutput(io.MultiWriter(os.Stdout, l.logFile))
	} else {
		l.SetOutput(os.Stdout)
	}

	// Initialize CSV export if enabled
	if cfg.CSVExport {
		csvPath := cfg.CSVPath
		if !filepath.IsAbs(csvPath) {
			csvPath = filepath.Join(configDir, csvPath)
		}

		if err := l.initCSV(csvPath); err != nil {
			l.Warnf("Failed to initialize CSV export: %v", err)
		}
	}

	l.initialized = true
	l.Info("Logger initialized")
	return nil
}

// initCSV initializes the CSV writer.
func (l *Logger) initCSV(path string) error {
	l.csvMu.Lock()
	defer l.csvMu.Unlock()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	// Check if file exists
	isNewFile := false
	if _, err := os.Stat(path); os.IsNotExist(err) {
		isNewFile = true
	}

	// Open file for appending
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	l.csvFile = file
	l.csvWriter = csv.NewWriter(file)

	// Write header if new file
	if isNewFile {
		header := []string{
			"Timestamp",
			"CPU%",
			"CPU_Temp",
			"RAM_MB",
			"RAM_Total_MB",
			"RAM%",
			"Swap_MB",
			"GPU%",
			"GPU_Temp",
			"GPU_VRAM_MB",
			"GPU_VRAM_Total_MB",
			"Disk_Read_MBps",
			"Disk_Write_MBps",
			"Net_Download_KBps",
			"Net_Upload_KBps",
		}
		if err := l.csvWriter.Write(header); err != nil {
			return err
		}
		l.csvWriter.Flush()
	}

	return nil
}

// LogMetrics writes metrics to the CSV file.
func (l *Logger) LogMetrics(m *models.Metrics) {
	if l.csvWriter == nil || l.csvFile == nil {
		return
	}

	l.csvMu.Lock()
	defer l.csvMu.Unlock()

	record := []string{
		m.Timestamp.Format("2006-01-02 15:04:05"),
		fmt.Sprintf("%.1f", m.CPU.UsagePercent),
		fmt.Sprintf("%.1f", m.CPU.Temperature),
		fmt.Sprintf("%d", m.Memory.UsedMB),
		fmt.Sprintf("%d", m.Memory.TotalMB),
		fmt.Sprintf("%.1f", m.Memory.UsedPercent),
		fmt.Sprintf("%d", m.Memory.SwapUsedMB),
		fmt.Sprintf("%.1f", m.GPU.UsagePercent),
		fmt.Sprintf("%d", m.GPU.TemperatureC),
		fmt.Sprintf("%d", m.GPU.VRAMUsedMB),
		fmt.Sprintf("%d", m.GPU.VRAMTotalMB),
		fmt.Sprintf("%.2f", m.Disk.ReadMBps),
		fmt.Sprintf("%.2f", m.Disk.WriteMBps),
		fmt.Sprintf("%.2f", m.Network.DownloadKBps),
		fmt.Sprintf("%.2f", m.Network.UploadKBps),
	}

	if err := l.csvWriter.Write(record); err != nil {
		l.Errorf("Failed to write CSV record: %v", err)
		return
	}
	l.csvWriter.Flush()
}

// ExportLogs exports the log buffer to a file.
func (l *Logger) ExportLogs(path string, entries []LogEntry) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, entry := range entries {
		_, err := fmt.Fprintf(file, "[%s] %s: %s\n",
			entry.Timestamp.Format("2006-01-02 15:04:05"),
			entry.Level,
			entry.Message)
		if err != nil {
			return err
		}
	}

	return nil
}

// ExportMetricsCSV exports metrics to a new CSV file.
func (l *Logger) ExportMetricsCSV(path string, metrics []*models.Metrics) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"Timestamp",
		"CPU%",
		"CPU_Temp",
		"RAM_MB",
		"RAM_Total_MB",
		"RAM%",
		"Swap_MB",
		"GPU%",
		"GPU_Temp",
		"GPU_VRAM_MB",
		"GPU_VRAM_Total_MB",
		"Disk_Read_MBps",
		"Disk_Write_MBps",
		"Net_Download_KBps",
		"Net_Upload_KBps",
	}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write records
	for _, m := range metrics {
		record := []string{
			m.Timestamp.Format("2006-01-02 15:04:05"),
			fmt.Sprintf("%.1f", m.CPU.UsagePercent),
			fmt.Sprintf("%.1f", m.CPU.Temperature),
			fmt.Sprintf("%d", m.Memory.UsedMB),
			fmt.Sprintf("%d", m.Memory.TotalMB),
			fmt.Sprintf("%.1f", m.Memory.UsedPercent),
			fmt.Sprintf("%d", m.Memory.SwapUsedMB),
			fmt.Sprintf("%.1f", m.GPU.UsagePercent),
			fmt.Sprintf("%d", m.GPU.TemperatureC),
			fmt.Sprintf("%d", m.GPU.VRAMUsedMB),
			fmt.Sprintf("%d", m.GPU.VRAMTotalMB),
			fmt.Sprintf("%.2f", m.Disk.ReadMBps),
			fmt.Sprintf("%.2f", m.Disk.WriteMBps),
			fmt.Sprintf("%.2f", m.Network.DownloadKBps),
			fmt.Sprintf("%.2f", m.Network.UploadKBps),
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// Close closes the logger and associated resources.
func (l *Logger) Close() {
	l.csvMu.Lock()
	defer l.csvMu.Unlock()

	if l.csvWriter != nil {
		l.csvWriter.Flush()
	}
	if l.csvFile != nil {
		l.csvFile.Close()
	}
	if l.logFile != nil {
		l.logFile.Close()
	}

	l.Info("Logger closed")
}

// LogEntry represents a log entry for the UI buffer.
type LogEntry struct {
	Timestamp time.Time
	Level     string
	Message   string
}

// LogBuffer is a circular buffer for storing recent log entries.
type LogBuffer struct {
	entries  []LogEntry
	capacity int
	head     int
	count    int
	mu       sync.RWMutex
}

// NewLogBuffer creates a new log buffer with the specified capacity.
func NewLogBuffer(capacity int) *LogBuffer {
	return &LogBuffer{
		entries:  make([]LogEntry, capacity),
		capacity: capacity,
	}
}

// Add adds a new log entry to the buffer.
func (b *LogBuffer) Add(level, message string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.entries[b.head] = LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
	}
	b.head = (b.head + 1) % b.capacity
	if b.count < b.capacity {
		b.count++
	}
}

// GetAll returns all log entries in chronological order.
func (b *LogBuffer) GetAll() []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return nil
	}

	result := make([]LogEntry, b.count)
	start := (b.head - b.count + b.capacity) % b.capacity

	for i := 0; i < b.count; i++ {
		idx := (start + i) % b.capacity
		result[i] = b.entries[idx]
	}

	return result
}

// GetFiltered returns log entries filtered by level.
func (b *LogBuffer) GetFiltered(levels ...string) []LogEntry {
	all := b.GetAll()
	if len(levels) == 0 {
		return all
	}

	levelSet := make(map[string]bool)
	for _, l := range levels {
		levelSet[l] = true
	}

	var filtered []LogEntry
	for _, entry := range all {
		if levelSet[entry.Level] {
			filtered = append(filtered, entry)
		}
	}

	return filtered
}

// Clear removes all entries from the buffer.
func (b *LogBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.head = 0
	b.count = 0
}

// BufferedHook is a logrus hook that writes entries to a LogBuffer.
type BufferedHook struct {
	buffer *LogBuffer
}

// NewBufferedHook creates a new BufferedHook.
func NewBufferedHook(capacity int) *BufferedHook {
	return &BufferedHook{
		buffer: NewLogBuffer(capacity),
	}
}

// Levels returns the log levels this hook should be called for.
func (h *BufferedHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Fire is called when a log entry is made.
func (h *BufferedHook) Fire(entry *logrus.Entry) error {
	h.buffer.Add(entry.Level.String(), entry.Message)
	return nil
}

// GetBuffer returns the underlying log buffer.
func (h *BufferedHook) GetBuffer() *LogBuffer {
	return h.buffer
}

// WithFields is a convenience wrapper for logrus.WithFields.
func (l *Logger) WithFields(fields logrus.Fields) *logrus.Entry {
	return l.Logger.WithFields(fields)
}

// CPU logs a CPU-related message.
func (l *Logger) CPU(format string, args ...interface{}) {
	l.WithField("component", "cpu").Infof(format, args...)
}

// Memory logs a memory-related message.
func (l *Logger) Memory(format string, args ...interface{}) {
	l.WithField("component", "memory").Infof(format, args...)
}

// GPU logs a GPU-related message.
func (l *Logger) GPU(format string, args ...interface{}) {
	l.WithField("component", "gpu").Infof(format, args...)
}

// Disk logs a disk-related message.
func (l *Logger) Disk(format string, args ...interface{}) {
	l.WithField("component", "disk").Infof(format, args...)
}

// Network logs a network-related message.
func (l *Logger) Network(format string, args ...interface{}) {
	l.WithField("component", "network").Infof(format, args...)
}

// Alert logs an alert message.
func (l *Logger) Alert(alertType string, format string, args ...interface{}) {
	l.WithFields(logrus.Fields{
		"component":  "alerter",
		"alert_type": alertType,
	}).Warnf(format, args...)
}
