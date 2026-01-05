package utils

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
	"unicode/utf8"
)

// Spinner represents a progress spinner
type Spinner struct {
	mu         sync.Mutex
	message    string
	chars      []string
	index      int
	delay      time.Duration
	active     bool
	done       chan bool
	writer     io.Writer
	showCursor bool
	color      string
	resetColor string
}

// SpinnerOptions configures the spinner
type SpinnerOptions struct {
	Message    string
	Chars      []string
	Delay      time.Duration
	Writer     io.Writer
	ShowCursor bool
	Color      string
}

// DefaultSpinnerOptions returns default spinner options
func DefaultSpinnerOptions() SpinnerOptions {
	return SpinnerOptions{
		Message:    "Loading",
		Chars:      []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		Delay:      100 * time.Millisecond,
		Writer:     os.Stdout,
		ShowCursor: false,
		Color:      "\033[36m", // Cyan
	}
}

// NewSpinner creates a new spinner
func NewSpinner(options SpinnerOptions) *Spinner {
	if options.Writer == nil {
		options.Writer = os.Stdout
	}

	if len(options.Chars) == 0 {
		options.Chars = DefaultSpinnerOptions().Chars
	}

	if options.Delay == 0 {
		options.Delay = DefaultSpinnerOptions().Delay
	}

	return &Spinner{
		message:    options.Message,
		chars:      options.Chars,
		delay:      options.Delay,
		writer:     options.Writer,
		showCursor: options.ShowCursor,
		color:      options.Color,
		resetColor: "\033[0m",
		done:       make(chan bool),
	}
}

// Start starts the spinner
func (s *Spinner) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.active {
		return
	}

	s.active = true
	s.done = make(chan bool)

	// Hide cursor if requested
	if !s.showCursor {
		fmt.Fprint(s.writer, "\033[?25l")
	}

	go s.animate()
}

// Stop stops the spinner
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return
	}

	s.active = false
	close(s.done)

	// Clear the line
	fmt.Fprint(s.writer, "\r\033[K")

	// Show cursor if hidden
	if !s.showCursor {
		fmt.Fprint(s.writer, "\033[?25h")
	}
}

// Success stops the spinner with a success message
func (s *Spinner) Success(message ...string) {
	s.stopWithMessage("✅", message...)
}

// Fail stops the spinner with a failure message
func (s *Spinner) Fail(message ...string) {
	s.stopWithMessage("❌", message...)
}

// Warn stops the spinner with a warning message
func (s *Spinner) Warn(message ...string) {
	s.stopWithMessage("⚠️", message...)
}

// UpdateMessage updates the spinner message
func (s *Spinner) UpdateMessage(message string) {
	s.mu.Lock()
	s.message = message
	s.mu.Unlock()
}

// animate runs the spinner animation
func (s *Spinner) animate() {
	ticker := time.NewTicker(s.delay)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			if s.active {
				s.render()
				s.index = (s.index + 1) % len(s.chars)
			}
			s.mu.Unlock()
		case <-s.done:
			return
		}
	}
}

// render renders the current spinner state
func (s *Spinner) render() {
	char := s.chars[s.index]
	if s.color != "" {
		char = s.color + char + s.resetColor
	}

	// Clear line and print spinner
	line := fmt.Sprintf("\r%s %s", char, s.message)
	fmt.Fprint(s.writer, line)
}

// stopWithMessage stops the spinner with a message
func (s *Spinner) stopWithMessage(icon string, messages ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return
	}

	s.active = false
	close(s.done)

	// Clear the line
	fmt.Fprint(s.writer, "\r\033[K")

	// Show cursor if hidden
	if !s.showCursor {
		fmt.Fprint(s.writer, "\033[?25h")
	}

	// Print final message
	message := s.message
	if len(messages) > 0 {
		message = messages[0]
	}

	line := fmt.Sprintf("%s %s\n", icon, message)
	fmt.Fprint(s.writer, line)
}

// Convenience functions

// WithSpinner runs a function with a spinner
func WithSpinner(message string, fn func() error) error {
	spinner := NewSpinner(SpinnerOptions{
		Message: message,
		Color:   "\033[36m",
	})
	spinner.Start()

	err := fn()

	if err != nil {
		spinner.Fail(message + " failed")
		return err
	}

	spinner.Success(message + " completed")
	return nil
}

// Simple spinner for quick tasks
func SimpleSpinner(message string) *Spinner {
	return NewSpinner(SpinnerOptions{
		Message: message,
		Chars:   []string{".", "..", "..."},
		Delay:   500 * time.Millisecond,
	})
}

// Colorful spinner with different styles
type SpinnerStyle string

const (
	StyleDots      SpinnerStyle = "dots"
	StyleLine      SpinnerStyle = "line"
	StylePulse     SpinnerStyle = "pulse"
	StyleBouncing  SpinnerStyle = "bouncing"
	StyleArrow     SpinnerStyle = "arrow"
	StyleClock     SpinnerStyle = "clock"
	StyleEarth     SpinnerStyle = "earth"
	StyleMoon      SpinnerStyle = "moon"
	StyleRunner    SpinnerStyle = "runner"
	StyleShark     SpinnerStyle = "shark"
	StyleTriangle  SpinnerStyle = "triangle"
	StyleFlip      SpinnerStyle = "flip"
	StyleHamburger SpinnerStyle = "hamburger"
)

var spinnerStyles = map[SpinnerStyle][]string{
	StyleDots:      {"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	StyleLine:      {"-", "\\", "|", "/"},
	StylePulse:     {"▉", "▊", "▋", "▌", "▍", "▎", "▏", "▎", "▍", "▌", "▋", "▊", "▉"},
	StyleBouncing:  {"[    ]", "[=   ]", "[==  ]", "[=== ]", "[ ===]", "[  ==]", "[   =]", "[    ]", "[   =]", "[  ==]", "[ ===]", "[====]", "[=== ]", "[==  ]", "[=   ]"},
	StyleArrow:     {"←", "↖", "↑", "↗", "→", "↘", "↓", "↙"},
	StyleClock:     {"🕐", "🕑", "🕒", "🕓", "🕔", "🕕", "🕖", "🕗", "🕘", "🕙", "🕚", "🕛"},
	StyleEarth:     {"🌍", "🌎", "🌏"},
	StyleMoon:      {"🌑", "🌒", "🌓", "🌔", "🌕", "🌖", "🌗", "🌘"},
	StyleRunner:    {"🚶", "🏃"},
	StyleShark:     {"▐|\\____________▌", "▐_|\\___________▌", "▐__|\\__________▌", "▐___|\\_________▌", "▐____|\\________▌", "▐_____|\\_______▌", "▐______|\\______▌", "▐_______|\\_____▌", "▐________|\\____▌", "▐_________|\\___▌", "▐__________|\\__▌", "▐___________|\\_▌", "▐____________|\\▌", "▐____________/|▌", "▐___________/|_▌", "▐__________/|__▌", "▐_________/|___▌", "▐________/|____▌", "▐_______/|_____▌", "▐______/|______▌", "▐_____/|_______▌", "▐____/|________▌", "▐___/|_________▌", "▐__/|__________▌", "▐_/|___________▌", "▐/|____________▌"},
	StyleTriangle:  {"◢", "◣", "◤", "◥"},
	StyleFlip:      {"___", "__-", "_--", "---", "--_", "-__"},
	StyleHamburger: {"☱", "☲", "☴"},
}

// NewStyledSpinner creates a spinner with a specific style
func NewStyledSpinner(style SpinnerStyle, message string, color string) *Spinner {
	chars, ok := spinnerStyles[style]
	if !ok {
		chars = spinnerStyles[StyleDots]
	}

	return NewSpinner(SpinnerOptions{
		Message: message,
		Chars:   chars,
		Delay:   100 * time.Millisecond,
		Color:   color,
	})
}

// GetSpinnerWidth returns the width of the spinner characters
func GetSpinnerWidth(chars []string) int {
	if len(chars) == 0 {
		return 0
	}
	return utf8.RuneCountInString(chars[0])
}