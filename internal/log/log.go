package log

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
)

var (
	logger *slog.Logger
	mu     sync.RWMutex
)

func init() {
	// Default to text handler on stderr
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// Level represents logging levels
type Level = slog.Level

const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// Options configures the logger
type Options struct {
	Level   Level
	JSON    bool
	Output  io.Writer
	Verbose bool
}

// Configure sets up the global logger
func Configure(opts Options) {
	mu.Lock()
	defer mu.Unlock()

	output := opts.Output
	if output == nil {
		output = os.Stderr
	}

	level := opts.Level
	if opts.Verbose {
		level = LevelDebug
	}

	handlerOpts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if opts.JSON {
		handler = slog.NewJSONHandler(output, handlerOpts)
	} else {
		handler = slog.NewTextHandler(output, handlerOpts)
	}

	logger = slog.New(handler)
}

// SetLevel changes the logging level
func SetLevel(level Level) {
	Configure(Options{Level: level})
}

// EnableDebug enables debug logging
func EnableDebug() {
	SetLevel(LevelDebug)
}

// Logger returns the global logger
func Logger() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return logger
}

// With returns a logger with additional attributes
func With(args ...any) *slog.Logger {
	return Logger().With(args...)
}

// Debug logs at debug level
func Debug(msg string, args ...any) {
	Logger().Debug(msg, args...)
}

// Info logs at info level
func Info(msg string, args ...any) {
	Logger().Info(msg, args...)
}

// Warn logs at warn level
func Warn(msg string, args ...any) {
	Logger().Warn(msg, args...)
}

// Error logs at error level
func Error(msg string, args ...any) {
	Logger().Error(msg, args...)
}

// DebugContext logs at debug level with context
func DebugContext(ctx context.Context, msg string, args ...any) {
	Logger().DebugContext(ctx, msg, args...)
}

// InfoContext logs at info level with context
func InfoContext(ctx context.Context, msg string, args ...any) {
	Logger().InfoContext(ctx, msg, args...)
}

// WarnContext logs at warn level with context
func WarnContext(ctx context.Context, msg string, args ...any) {
	Logger().WarnContext(ctx, msg, args...)
}

// ErrorContext logs at error level with context
func ErrorContext(ctx context.Context, msg string, args ...any) {
	Logger().ErrorContext(ctx, msg, args...)
}

// Err is a helper for logging errors
func Err(err error) slog.Attr {
	return slog.Any("error", err)
}

// TaskID is a helper for logging task IDs
func TaskID(id string) slog.Attr {
	return slog.String("task_id", id)
}

// State is a helper for logging state transitions
func State(from, to string) []slog.Attr {
	return []slog.Attr{
		slog.String("from", from),
		slog.String("to", to),
	}
}
