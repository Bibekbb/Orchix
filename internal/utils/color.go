package utils

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// Color represents an ANSI color code
type Color string

// ANSI color codes
const (
	Reset        Color = "\033[0m"
	Black        Color = "\033[30m"
	Red          Color = "\033[31m"
	Green        Color = "\033[32m"
	Yellow       Color = "\033[33m"
	Blue         Color = "\033[34m"
	Magenta      Color = "\033[35m"
	Cyan         Color = "\033[36m"
	White        Color = "\033[37m"
	BrightBlack  Color = "\033[90m"
	BrightRed    Color = "\033[91m"
	BrightGreen  Color = "\033[92m"
	BrightYellow Color = "\033[93m"
	BrightBlue   Color = "\033[94m"
	BrightMagenta Color = "\033[95m"
	BrightCyan   Color = "\033[96m"
	BrightWhite  Color = "\033[97m"
	BgBlack      Color = "\033[40m"
	BgRed        Color = "\033[41m"
	BgGreen      Color = "\033[42m"
	BgYellow     Color = "\033[43m"
	BgBlue       Color = "\033[44m"
	BgMagenta    Color = "\033[45m"
	BgCyan       Color = "\033[46m"
	BgWhite      Color = "\033[47m"
	Bold         Color = "\033[1m"
	Dim          Color = "\033[2m"
	Italic       Color = "\033[3m"
	Underline    Color = "\033[4m"
	Blink        Color = "\033[5m"
	Reverse      Color = "\033[7m"
	Hidden       Color = "\033[8m"
)

// Colorizer provides colorized output
type Colorizer struct {
	enabled bool
	colors  map[string]Color
}

// NewColorizer creates a new colorizer
func NewColorizer(enabled bool) *Colorizer {
	colorizer := &Colorizer{
		enabled: enabled && supportsColor(),
		colors:  make(map[string]Color),
	}

	// Register default colors
	colorizer.RegisterColor("success", Green)
	colorizer.RegisterColor("error", Red)
	colorizer.RegisterColor("warning", Yellow)
	colorizer.RegisterColor("info", Cyan)
	colorizer.RegisterColor("debug", Blue)
	colorizer.RegisterColor("highlight", Magenta)
	colorizer.RegisterColor("muted", BrightBlack)

	return colorizer
}

// RegisterColor registers a named color
func (c *Colorizer) RegisterColor(name string, color Color) {
	c.colors[name] = color
}

// Colorize applies color to text
func (c *Colorizer) Colorize(text string, color Color) string {
	if !c.enabled || color == "" {
		return text
	}
	return string(color) + text + string(Reset)
}

// NamedColor applies a named color to text
func (c *Colorizer) NamedColor(text string, name string) string {
	color, ok := c.colors[name]
	if !ok {
		return text
	}
	return c.Colorize(text, color)
}

// Success returns success-colored text
func (c *Colorizer) Success(text string) string {
	return c.NamedColor(text, "success")
}

// Error returns error-colored text
func (c *Colorizer) Error(text string) string {
	return c.NamedColor(text, "error")
}

// Warning returns warning-colored text
func (c *Colorizer) Warning(text string) string {
	return c.NamedColor(text, "warning")
}

// Info returns info-colored text
func (c *Colorizer) Info(text string) string {
	return c.NamedColor(text, "info")
}

// Debug returns debug-colored text
func (c *Colorizer) Debug(text string) string {
	return c.NamedColor(text, "debug")
}

// Highlight returns highlighted text
func (c *Colorizer) Highlight(text string) string {
	return c.NamedColor(text, "highlight")
}

// Muted returns muted text
func (c *Colorizer) Muted(text string) string {
	return c.NamedColor(text, "muted")
}

// Printf prints colorized text
func (c *Colorizer) Printf(format string, args ...interface{}) {
	fmt.Print(c.Sprintf(format, args...))
}

// Sprintf returns colorized formatted string
func (c *Colorizer) Sprintf(format string, args ...interface{}) string {
	text := fmt.Sprintf(format, args...)
	
	// Replace color tags
	for name, color := range c.colors {
		tag := fmt.Sprintf("{%s}", name)
		if strings.Contains(text, tag) {
			text = strings.ReplaceAll(text, tag, string(color))
		}
	}
	
	// Reset color at the end
	if c.enabled && strings.Contains(text, string(Reset)) {
		text += string(Reset)
	}
	
	return text
}

// Println prints colorized line
func (c *Colorizer) Println(args ...interface{}) {
	fmt.Println(c.Sprintln(args...))
}

// Sprintln returns colorized line
func (c *Colorizer) Sprintln(args ...interface{}) string {
	text := fmt.Sprintln(args...)
	return c.Colorize(text, Reset)
}

// Style creates a styled text
func (c *Colorizer) Style(text string, styles ...Color) string {
	if !c.enabled {
		return text
	}
	
	var result string
	for _, style := range styles {
		result += string(style)
	}
	result += text + string(Reset)
	return result
}

// ProgressBar creates a colorized progress bar
func (c *Colorizer) ProgressBar(percent float64, width int) string {
	if !c.enabled {
		return createProgressBar(percent, width, false)
	}
	return createProgressBar(percent, width, true)
}

// Table creates a colorized table
func (c *Colorizer) Table(headers []string, rows [][]string) string {
	if !c.enabled {
		return createTable(headers, rows, false)
	}
	return createTable(headers, rows, true)
}

// Helper functions

// supportsColor checks if the terminal supports colors
func supportsColor() bool {
	// Check for NO_COLOR environment variable
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// Check for CLICOLOR_FORCE
	if os.Getenv("CLICOLOR_FORCE") == "1" {
		return true
	}

	// Check if output is a terminal
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		return false
	}

	// Check for CLICOLOR
	if os.Getenv("CLICOLOR") == "0" {
		return false
	}

	// Windows has limited color support
	if runtime.GOOS == "windows" {
		return os.Getenv("ANSICON") != "" || os.Getenv("ConEmuANSI") == "ON"
	}

	return true
}

// createProgressBar creates a progress bar
func createProgressBar(percent float64, width int, colorized bool) string {
	if width <= 0 {
		width = 40
	}

	completed := int(percent * float64(width) / 100.0)
	if completed > width {
		completed = width
	}

	remaining := width - completed

	bar := strings.Repeat("█", completed) + strings.Repeat("░", remaining)
	percentage := fmt.Sprintf("%6.2f%%", percent)

	if colorized {
		var color Color
		switch {
		case percent >= 80:
			color = Green
		case percent >= 50:
			color = Yellow
		default:
			color = Red
		}
		return fmt.Sprintf("[%s%s%s] %s", color, bar, Reset, percentage)
	}

	return fmt.Sprintf("[%s] %s", bar, percentage)
}

// createTable creates a table
func createTable(headers []string, rows [][]string, colorized bool) string {
	if len(headers) == 0 || len(rows) == 0 {
		return ""
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}

	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	var builder strings.Builder

	// Create separator
	separator := "+"
	for _, width := range widths {
		separator += strings.Repeat("-", width+2) + "+"
	}
	separator += "\n"

	// Write header
	builder.WriteString(separator)
	builder.WriteString("|")
	for i, header := range headers {
		padded := fmt.Sprintf(" %-*s ", widths[i], header)
		if colorized {
			padded = string(Bold) + padded + string(Reset)
		}
		builder.WriteString(padded + "|")
	}
	builder.WriteString("\n")
	builder.WriteString(separator)

	// Write rows
	for _, row := range rows {
		builder.WriteString("|")
		for i := 0; i < len(headers); i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			padded := fmt.Sprintf(" %-*s ", widths[i], cell)
			
			// Colorize based on cell content
			if colorized && i == 0 {
				padded = string(Cyan) + padded + string(Reset)
			}
			
			builder.WriteString(padded + "|")
		}
		builder.WriteString("\n")
	}

	builder.WriteString(separator)
	return builder.String()
}

// Global colorizer instance
var (
	globalColorizer *Colorizer
	colorizerOnce   sync.Once
)

// GetColorizer returns the global colorizer
func GetColorizer() *Colorizer {
	colorizerOnce.Do(func() {
		globalColorizer = NewColorizer(true)
	})
	return globalColorizer
}

// Color shortcuts using global colorizer
func Colorize(text string, color Color) string {
	return GetColorizer().Colorize(text, color)
}

func Success(text string) string {
	return GetColorizer().Success(text)
}

func Error(text string) string {
	return GetColorizer().Error(text)
}

func Warning(text string) string {
	return GetColorizer().Warning(text)
}

func Info(text string) string {
	return GetColorizer().Info(text)
}

func Debug(text string) string {
	return GetColorizer().Debug(text)
}

func Highlight(text string) string {
	return GetColorizer().Highlight(text)
}

func Muted(text string) string {
	return GetColorizer().Muted(text)
}

// Print functions using global colorizer
func PrintSuccess(format string, args ...interface{}) {
	fmt.Print(Success(fmt.Sprintf(format, args...)))
}

func PrintError(format string, args ...interface{}) {
	fmt.Print(Error(fmt.Sprintf(format, args...)))
}

func PrintWarning(format string, args ...interface{}) {
	fmt.Print(Warning(fmt.Sprintf(format, args...)))
}

func PrintInfo(format string, args ...interface{}) {
	fmt.Print(Info(fmt.Sprintf(format, args...)))
}

// Rainbow text effect
func Rainbow(text string) string {
	if !supportsColor() {
		return text
	}

	colors := []Color{Red, Yellow, Green, Cyan, Blue, Magenta}
	var result strings.Builder

	for i, char := range text {
		color := colors[i%len(colors)]
		result.WriteString(string(color) + string(char))
	}
	result.WriteString(string(Reset))

	return result.String()
}

// Gradient text effect
func Gradient(text string, start, end Color) string {
	if !supportsColor() || len(text) < 2 {
		return Colorize(text, start)
	}

	var result strings.Builder
	length := len(text)

	for i, char := range text {
		// Linear interpolation between start and end
		ratio := float64(i) / float64(length-1)
		// Simple gradient - in reality you'd need to interpolate RGB values
		if ratio < 0.5 {
			result.WriteString(string(start) + string(char))
		} else {
			result.WriteString(string(end) + string(char))
		}
	}
	result.WriteString(string(Reset))

	return result.String()
}