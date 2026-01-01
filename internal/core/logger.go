package core

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// LogLevel represents logging level
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

// String returns string representation of log level
func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// LoggerConfig holds logger configuration
type LoggerConfig struct {
	Level      LogLevel
	Format     string // "text", "json"
	Output     string // "stdout", "file", "both"
	FilePath   string
	MaxSize    int64  // Max file size in MB
	MaxBackups int    // Max number of backup files
	MaxAge     int    // Max age in days
	Compress   bool   // Compress backup files
}

// Logger is a structured logger
type Logger struct {
	config   LoggerConfig
	mu       sync.Mutex
	file     *os.File
	writer   io.Writer
	fields   map[string]interface{}
	prefix   string
}

// NewLogger creates a new logger instance
func NewLogger() *Logger {
	return NewLoggerWithConfig(LoggerConfig{
		Level:    LevelInfo,
		Format:   "text",
		Output:   "stdout",
		FilePath: "logs/orchix.log",
		MaxSize:  10, // 10MB
		MaxBackups: 5,
		MaxAge:   30, // 30 days
		Compress: true,
	})
}

// NewLoggerWithConfig creates a logger with custom configuration
func NewLoggerWithConfig(config LoggerConfig) *Logger {
	logger := &Logger{
		config: config,
		fields: make(map[string]interface{}),
	}
	
	logger.setupWriter()
	return logger
}

// setupWriter sets up the log writer based on configuration
func (l *Logger) setupWriter() {
	var writers []io.Writer
	
	// Add stdout if configured
	if l.config.Output == "stdout" || l.config.Output == "both" {
		writers = append(writers, os.Stdout)
	}
	
	// Add file if configured
	if l.config.Output == "file" || l.config.Output == "both" {
		if err := l.setupFile(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to setup log file: %v\n", err)
			// Fall back to stdout
			l.writer = os.Stdout
			return
		}
		writers = append(writers, l.file)
	}
	
	if len(writers) == 0 {
		// Default to stdout
		l.writer = os.Stdout
	} else if len(writers) == 1 {
		l.writer = writers[0]
	} else {
		l.writer = io.MultiWriter(writers...)
	}
}

// setupFile sets up log file with rotation
func (l *Logger) setupFile() error {
	// Create log directory if it doesn't exist
	dir := filepath.Dir(l.config.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	
	// Open log file
	file, err := os.OpenFile(l.config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	
	l.file = file
	return nil
}

// WithField adds a field to the logger context
func (l *Logger) WithField(key string, value interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	newLogger := &Logger{
		config: l.config,
		writer: l.writer,
		fields: make(map[string]interface{}),
		prefix: l.prefix,
	}
	
	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	
	// Add new field
	newLogger.fields[key] = value
	
	return newLogger
}

// WithFields adds multiple fields to the logger context
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	newLogger := &Logger{
		config: l.config,
		writer: l.writer,
		fields: make(map[string]interface{}),
		prefix: l.prefix,
	}
	
	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	
	// Add new fields
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	
	return newLogger
}

// WithPrefix sets a prefix for the logger
func (l *Logger) WithPrefix(prefix string) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	newLogger := &Logger{
		config: l.config,
		writer: l.writer,
		fields: make(map[string]interface{}),
		prefix: prefix,
	}
	
	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	
	return newLogger
}

// log writes a log entry
func (l *Logger) log(level LogLevel, msg string, args ...interface{}) {
	if level < l.config.Level {
		return
	}
	
	// Format message with arguments
	formattedMsg := fmt.Sprintf(msg, args...)
	
	// Get caller information for debug level
	var caller string
	if level == LevelDebug {
		if pc, file, line, ok := runtime.Caller(2); ok {
			funcName := runtime.FuncForPC(pc).Name()
			caller = fmt.Sprintf("%s:%d %s", filepath.Base(file), line, funcName)
		}
	}
	
	// Create log entry
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   formattedMsg,
		Caller:    caller,
		Fields:    l.fields,
		Prefix:    l.prefix,
	}
	
	// Format and write the entry
	l.writeEntry(entry)
}

// writeEntry writes a log entry to the output
func (l *Logger) writeEntry(entry LogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	var output string
	
	switch l.config.Format {
	case "json":
		output = entry.ToJSON()
	default:
		output = entry.ToText()
	}
	
	fmt.Fprintln(l.writer, output)
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...interface{}) {
	l.log(LevelDebug, msg, args...)
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...interface{}) {
	l.log(LevelInfo, msg, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...interface{}) {
	l.log(LevelWarn, msg, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...interface{}) {
	l.log(LevelError, msg, args...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, args ...interface{}) {
	l.log(LevelFatal, msg, args...)
	os.Exit(1)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.Debug(format, args...)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	l.Info(format, args...)
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.Warn(format, args...)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Error(format, args...)
}

// Fatalf logs a formatted fatal message and exits
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.Fatal(format, args...)
}

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     LogLevel               `json:"level"`
	Message   string                 `json:"message"`
	Caller    string                 `json:"caller,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Prefix    string                 `json:"prefix,omitempty"`
}

// ToText formats log entry as text
func (e *LogEntry) ToText() string {
	timestamp := e.Timestamp.Format("2006-01-02 15:04:05.000")
	levelStr := e.Level.String()
	
	var builder strings.Builder
	
	// Add prefix if present
	if e.Prefix != "" {
		builder.WriteString(e.Prefix)
		builder.WriteString(" ")
	}
	
	// Add timestamp and level
	builder.WriteString(fmt.Sprintf("[%s] %-5s", timestamp, levelStr))
	
	// Add caller for debug messages
	if e.Level == LevelDebug && e.Caller != "" {
		builder.WriteString(fmt.Sprintf(" [%s]", e.Caller))
	}
	
	// Add message
	builder.WriteString(fmt.Sprintf(" %s", e.Message))
	
	// Add fields if present
	if len(e.Fields) > 0 {
		builder.WriteString(" |")
		for k, v := range e.Fields {
			builder.WriteString(fmt.Sprintf(" %s=%v", k, v))
		}
	}
	
	return builder.String()
}

// ToJSON formats log entry as JSON
func (e *LogEntry) ToJSON() string {
	// Simple JSON representation
	// In a real implementation, use encoding/json
	return e.ToText() // Simplified for now
}

// ComponentLogger creates a logger for a specific component
func (l *Logger) ComponentLogger(componentID, componentName string) *Logger {
	return l.WithFields(map[string]interface{}{
		"component_id":   componentID,
		"component_name": componentName,
	}).WithPrefix(fmt.Sprintf("[%s]", componentID))
}

// StageLogger creates a logger for a deployment stage
func (l *Logger) StageLogger(stageNumber int, stageName string) *Logger {
	return l.WithFields(map[string]interface{}{
		"stage_number": stageNumber,
		"stage_name":   stageName,
	}).WithPrefix(fmt.Sprintf("[Stage %d]", stageNumber))
}

// DeploymentLogger creates a logger for a deployment
func (l *Logger) DeploymentLogger(deploymentID, appName string) *Logger {
	return l.WithFields(map[string]interface{}{
		"deployment_id": deploymentID,
		"app_name":      appName,
	}).WithPrefix(fmt.Sprintf("[%s]", deploymentID[:8]))
}

// Close closes the logger and any open files
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.file != nil {
		return l.file.Close()
	}
	
	return nil
}

// SetLevel changes the log level
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.config.Level = level
}

// GetLevel returns the current log level
func (l *Logger) GetLevel() LogLevel {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.config.Level
}

// Global logger instance
var (
	globalLogger     *Logger
	globalLoggerOnce sync.Once
)

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	globalLoggerOnce.Do(func() {
		globalLogger = NewLogger()
	})
	return globalLogger
}

// SetGlobalLogger sets the global logger
func SetGlobalLogger(logger *Logger) {
	globalLogger = logger
}

// Convenience functions using global logger

// Debug logs debug message to global logger
func Debug(msg string, args ...interface{}) {
	GetLogger().Debug(msg, args...)
}

// Info logs info message to global logger
func Info(msg string, args ...interface{}) {
	GetLogger().Info(msg, args...)
}

// Warn logs warning message to global logger
func Warn(msg string, args ...interface{}) {
	GetLogger().Warn(msg, args...)
}

// Error logs error message to global logger
func Error(msg string, args ...interface{}) {
	GetLogger().Error(msg, args...)
}

// Fatal logs fatal message to global logger
func Fatal(msg string, args ...interface{}) {
	GetLogger().Fatal(msg, args...)
}

// Debugf logs formatted debug message to global logger
func Debugf(format string, args ...interface{}) {
	GetLogger().Debugf(format, args...)
}

// Infof logs formatted info message to global logger
func Infof(format string, args ...interface{}) {
	GetLogger().Infof(format, args...)
}

// Warnf logs formatted warning message to global logger
func Warnf(format string, args ...interface{}) {
	GetLogger().Warnf(format, args...)
}

// Errorf logs formatted error message to global logger
func Errorf(format string, args ...interface{}) {
	GetLogger().Errorf(format, args...)
}

// Fatalf logs formatted fatal message to global logger
func Fatalf(format string, args ...interface{}) {
	GetLogger().Fatalf(format, args...)
}

// Simple logger for quick use
type SimpleLogger struct{}

func (s *SimpleLogger) Printf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

func (s *SimpleLogger) Println(args ...interface{}) {
	log.Println(args...)
}

// NewSimpleLogger creates a simple stdlib logger wrapper
func NewSimpleLogger() *SimpleLogger {
	return &SimpleLogger{}
}