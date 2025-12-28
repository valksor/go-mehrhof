// Package display provides user-friendly formatting for CLI output.
package display

import (
	"fmt"
	"os"
	"sync"
)

// ANSI color codes
const (
	reset   = "\033[0m"
	bold    = "\033[1m"
	dim     = "\033[2m"
	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
	gray    = "\033[90m"
)

var (
	colorEnabled     = true
	colorInitialized = false
	colorMu          sync.RWMutex
)

// InitColors initializes the color system based on flags and environment.
// Should be called once during startup with the --no-color flag value.
func InitColors(noColor bool) {
	colorMu.Lock()
	defer colorMu.Unlock()

	colorInitialized = true

	// Disable colors if --no-color flag is set
	if noColor {
		colorEnabled = false
		return
	}

	// Respect NO_COLOR environment variable (https://no-color.org/)
	if _, exists := os.LookupEnv("NO_COLOR"); exists {
		colorEnabled = false
		return
	}

	// Could add terminal detection here if needed
	colorEnabled = true
}

// ColorsEnabled returns whether colors are currently enabled.
func ColorsEnabled() bool {
	colorMu.RLock()
	defer colorMu.RUnlock()

	// Auto-initialize if not done yet
	if !colorInitialized {
		colorMu.RUnlock()
		InitColors(false)
		colorMu.RLock()
	}

	return colorEnabled
}

// SetColorsEnabled allows manual control of color output (useful for testing).
func SetColorsEnabled(enabled bool) {
	colorMu.Lock()
	defer colorMu.Unlock()
	colorEnabled = enabled
	colorInitialized = true
}

// colorize wraps text in ANSI color codes if colors are enabled.
func colorize(text, color string) string {
	if !ColorsEnabled() {
		return text
	}
	return color + text + reset
}

// Semantic color functions

// Success formats text as successful (green).
func Success(text string) string {
	return colorize(text, green)
}

// Error formats text as an error (red).
func Error(text string) string {
	return colorize(text, red)
}

// Warning formats text as a warning (yellow).
func Warning(text string) string {
	return colorize(text, yellow)
}

// Info formats text as informational (blue).
func Info(text string) string {
	return colorize(text, blue)
}

// Muted formats text as muted/secondary (gray).
func Muted(text string) string {
	return colorize(text, gray)
}

// Bold formats text as bold.
func Bold(text string) string {
	return colorize(text, bold)
}

// Dim formats text as dim/faded.
func Dim(text string) string {
	return colorize(text, dim)
}

// Cyan formats text in cyan (used for commands/code).
func Cyan(text string) string {
	return colorize(text, cyan)
}

// Prefixed message helpers

// SuccessPrefix returns a success checkmark prefix.
func SuccessPrefix() string {
	return Success("✓")
}

// ErrorPrefix returns an error X prefix.
func ErrorPrefix() string {
	return Error("✗")
}

// WarningPrefix returns a warning icon prefix.
func WarningPrefix() string {
	return Warning("⚠")
}

// InfoPrefix returns an info arrow prefix.
func InfoPrefix() string {
	return Info("→")
}

// Formatted messages

// SuccessMsg formats a success message with prefix.
func SuccessMsg(format string, args ...any) string {
	msg := fmt.Sprintf(format, args...)
	return fmt.Sprintf("%s %s", SuccessPrefix(), msg)
}

// ErrorMsg formats an error message with prefix.
func ErrorMsg(format string, args ...any) string {
	msg := fmt.Sprintf(format, args...)
	return fmt.Sprintf("%s %s", ErrorPrefix(), Error(msg))
}

// WarningMsg formats a warning message with prefix.
func WarningMsg(format string, args ...any) string {
	msg := fmt.Sprintf(format, args...)
	return fmt.Sprintf("%s %s", WarningPrefix(), Warning(msg))
}

// InfoMsg formats an info message with prefix.
func InfoMsg(format string, args ...any) string {
	msg := fmt.Sprintf(format, args...)
	return fmt.Sprintf("%s %s", InfoPrefix(), msg)
}

// State color mapping

// ColorState returns a colored state string based on the state value.
func ColorState(state, displayName string) string {
	switch state {
	case "idle":
		return Muted(displayName)
	case "planning", "implementing", "reviewing", "checkpointing":
		return Info(displayName)
	case "done":
		return Success(displayName)
	case "failed":
		return Error(displayName)
	case "waiting", "dialogue":
		return Warning(displayName)
	default:
		return displayName
	}
}

// ColorSpecStatus returns a colored specification status.
func ColorSpecStatus(status, displayName string) string {
	switch status {
	case "draft":
		return Muted(displayName)
	case "ready":
		return Warning(displayName)
	case "implementing":
		return Info(displayName)
	case "done":
		return Success(displayName)
	default:
		return displayName
	}
}
