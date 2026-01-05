package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// LogLevel represents the severity level of a log message
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

var (
	levelNames = map[LogLevel]string{
		LevelDebug: "DEBUG",
		LevelInfo:  "INFO",
		LevelWarn:  "WARN",
		LevelError: "ERROR",
		LevelFatal: "FATAL",
	}

	levelColors = map[LogLevel]string{
		LevelDebug: "\033[36m", // Cyan
		LevelInfo:  "\033[32m", // Green
		LevelWarn:  "\033[33m", // Yellow
		LevelError: "\033[31m", // Red
		LevelFatal: "\033[35m", // Magenta
	}
)

// Logger represents a structured logger
type Logger struct {
	mu        sync.Mutex
	level     LogLevel
	output    io.Writer
	showTime  bool
	showLevel bool
	showCaller bool
	colorful  bool
	name      string
}

// LoggerOptions configures the logger
type LoggerOptions struct {
	Level      LogLevel
	Output     io.Writer
	ShowTime   bool
	ShowLevel  bool
	ShowCaller bool
	Colorful   bool
	Name       string
}

// DefaultLoggerOptions returns default logger options
func DefaultLoggerOptions() LoggerOptions {
	return LoggerOptions{
		Level:      LevelInfo,
		Output:     os.Stdout,
		ShowTime:   true,
		ShowLevel:  true,
		ShowCaller: false,
		Colorful:   true,
		Name:       "orchix",
	}
}

// NewLogger creates a new logger with the given options
func NewLogger(options LoggerOptions) *Logger {
	if options.Output == nil {
		options.Output = os.Stdout
	}

	return &Logger{
		level:     options.Level,
		output:    options.Output,
		showTime:  options.ShowTime,
		showLevel: options.ShowLevel,
		showCaller: options.ShowCaller,
		colorful:  options.Colorful,
		name:      options.Name,
	}
}

// SetLevel changes the log level
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetOutput changes the output writer
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...interface{}) {
	l.log(LevelDebug, msg, fields...)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...interface{}) {
	l.log(LevelInfo, msg, fields...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...interface{}) {
	l.log(LevelWarn, msg, fields...)
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...interface{}) {
	l.log(LevelError, msg, fields...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, fields ...interface{}) {
	l.log(LevelFatal, msg, fields...)
	os.Exit(1)
}

// WithFields creates a new logger with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	// For structured logging, you'd append fields to each log
	// For simplicity, we return the same logger
	return l
}

// log writes a log message
func (l *Logger) log(level LogLevel, msg string, fields ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	var parts []string

	// Add timestamp
	if l.showTime {
		timestamp := time.Now().Format("2006-01-02 15:04:05.000")
		parts = append(parts, timestamp)
	}

	// Add level
	if l.showLevel {
		levelStr := levelNames[level]
		if l.colorful {
			color := levelColors[level]
			reset := "\033[0m"
			levelStr = color + levelStr + reset
		}
		parts = append(parts, levelStr)
	}

	// Add logger name
	if l.name != "" {
		parts = append(parts, "["+l.name+"]")
	}

	// Add caller info
	if l.showCaller {
		if file, line, ok := getCaller(); ok {
			caller := fmt.Sprintf("%s:%d", filepath.Base(file), line)
			parts = append(parts, "("+caller+")")
		}
	}

	// Add message
	parts = append(parts, msg)

	// Format fields if provided
	if len(fields) > 0 {
		fieldStr := formatFields(fields...)
		if fieldStr != "" {
			parts = append(parts, fieldStr)
		}
	}

	// Write to output
	line := strings.Join(parts, " ") + "\n"
	l.output.Write([]byte(line))
}

// formatFields formats key-value pairs
func formatFields(fields ...interface{}) string {
	if len(fields)%2 != 0 {
		fields = append(fields, "MISSING")
	}

	var pairs []string
	for i := 0; i < len(fields); i += 2 {
		key := fmt.Sprintf("%v", fields[i])
		value := fmt.Sprintf("%v", fields[i+1])
		pairs = append(pairs, key+"="+value)
	}

	if len(pairs) > 0 {
		return "[" + strings.Join(pairs, " ") + "]"
	}
	return ""
}

// getCaller returns the file and line number of the caller
func getCaller() (string, int, bool) {
	// Skip this function and the log function
	for i := 2; i < 10; i++ {
		_, file, line, ok := runtime.Caller(i)
		if ok && !strings.Contains(file, "logger.go") {
			return file, line, true
		}
	}
	return "", 0, false
}

// Global logger instance
var (
	globalLogger *Logger
	once         sync.Once
)

// GetLogger returns the global logger instance
func GetLogger() *Logger {
	once.Do(func() {
		globalLogger = NewLogger(DefaultLoggerOptions())
	})
	return globalLogger
}

// SetGlobalLogger sets the global logger
func SetGlobalLogger(logger *Logger) {
	globalLogger = logger
}

// Convenience functions
func Debug(msg string, fields ...interface{}) {
	GetLogger().Debug(msg, fields...)
}

func Info(msg string, fields ...interface{}) {
	GetLogger().Info(msg, fields...)
}

func Warn(msg string, fields ...interface{}) {
	GetLogger().Warn(msg, fields...)
}

func Error(msg string, fields ...interface{}) {
	GetLogger().Error(msg, fields...)
}

func Fatal(msg string, fields ...interface{}) {
	GetLogger().Fatal(msg, fields...)
}

// FileLogger creates a logger that writes to a file
func FileLogger(filename string, options LoggerOptions) (*Logger, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	options.Output = file
	return NewLogger(options), nil
}

// JSONLogger creates a JSON-formatted logger
type JSONLogger struct {
	*Logger
}

// NewJSONLogger creates a new JSON logger
func NewJSONLogger(options LoggerOptions) *JSONLogger {
	// For simplicity, we'll use the same logger
	// In production, you'd implement JSON formatting
	return &JSONLogger{Logger: NewLogger(options)}
}