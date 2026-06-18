package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Format controls how log records are written.
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// Options configures the global logger.
type Options struct {
	// AppName is attached to every log record.
	AppName string
	// Level is one of debug, info, warn, or error.
	Level string
	// Format is text or json.
	Format Format
	// Debug enables source file and line on each record.
	Debug bool
	// LogFile is the path to append logs to. When empty, logs go to stderr.
	LogFile string
	// Output overrides the writer used for logs. When nil, stderr or LogFile is used.
	Output io.Writer
}

var (
	mu      sync.RWMutex
	global  *slog.Logger
	logFile *os.File
)

// Init configures the package-wide logger. Call once at startup after config.Load().
func Init(opts Options) error {
	output, err := resolveOutput(opts)
	if err != nil {
		return err
	}

	level := parseLevel(opts.Level)
	handlerOpts := &slog.HandlerOptions{
		Level:     level,
		AddSource: opts.Debug,
	}

	var handler slog.Handler
	switch strings.ToLower(string(opts.Format)) {
	case string(FormatJSON):
		handler = slog.NewJSONHandler(output, handlerOpts)
	default:
		handler = slog.NewTextHandler(output, handlerOpts)
	}

	attrs := make([]any, 0, 2)
	if opts.AppName != "" {
		attrs = append(attrs, "app", opts.AppName)
	}

	mu.Lock()
	if logFile != nil {
		if newFile, ok := output.(*os.File); ok && newFile == logFile {
			// keep the existing file handle
		} else {
			_ = logFile.Close()
			logFile = nil
		}
	}
	if f, ok := output.(*os.File); ok && opts.LogFile != "" {
		logFile = f
	}
	global = slog.New(handler).With(attrs...)
	mu.Unlock()

	return nil
}

// Close flushes and closes the log file when file logging is enabled.
func Close() error {
	mu.Lock()
	defer mu.Unlock()

	if logFile == nil {
		return nil
	}

	err := logFile.Close()
	logFile = nil
	return err
}

func resolveOutput(opts Options) (io.Writer, error) {
	if opts.Output != nil {
		return opts.Output, nil
	}

	logPath := strings.TrimSpace(opts.LogFile)
	if logPath == "" {
		return os.Stderr, nil
	}

	file, err := openLogFile(logPath)
	if err != nil {
		return nil, err
	}

	return file, nil
}

func openLogFile(path string) (*os.File, error) {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create log directory %q: %w", dir, err)
		}
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file %q: %w", path, err)
	}

	return file, nil
}

// L returns the global logger. Before Init, a default text logger is used.
func L() *slog.Logger {
	mu.RLock()
	l := global
	mu.RUnlock()

	if l != nil {
		return l
	}

	return slog.Default()
}

// With returns a child logger with fixed key-value attributes.
// Use it to scope logs to a package or component:
//
//	var log = logger.With("component", "http")
//	log.Info("request handled", "method", "GET", "path", "/health")
func With(args ...any) *slog.Logger {
	return L().With(args...)
}

// Component returns a logger scoped to a named component.
func Component(name string) *slog.Logger {
	return With("component", name)
}

// Debug logs at debug level.
func Debug(msg string, args ...any) {
	L().Debug(msg, args...)
}

// Info logs at info level.
func Info(msg string, args ...any) {
	L().Info(msg, args...)
}

// Warn logs at warn level.
func Warn(msg string, args ...any) {
	L().Warn(msg, args...)
}

// Error logs at error level.
func Error(msg string, args ...any) {
	L().Error(msg, args...)
}

// Fatal logs at error level and exits with status 1.
func Fatal(msg string, args ...any) {
	L().Error(msg, args...)
	os.Exit(1)
}

// DebugContext logs at debug level with context.
func DebugContext(ctx context.Context, msg string, args ...any) {
	L().DebugContext(ctx, msg, args...)
}

// InfoContext logs at info level with context.
func InfoContext(ctx context.Context, msg string, args ...any) {
	L().InfoContext(ctx, msg, args...)
}

// WarnContext logs at warn level with context.
func WarnContext(ctx context.Context, msg string, args ...any) {
	L().WarnContext(ctx, msg, args...)
}

// ErrorContext logs at error level with context.
func ErrorContext(ctx context.Context, msg string, args ...any) {
	L().ErrorContext(ctx, msg, args...)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
