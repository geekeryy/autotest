// Package logx provides leveled structured logging for the API server.
// Configure via LOG_LEVEL (debug|info|warn|error) and LOG_FORMAT (text|json).
package logx

import (
	"log/slog"
	"os"
	"strings"
)

var logger = slog.Default()

// Init configures the process-wide slog logger from environment variables.
// LOG_LEVEL defaults to info; LOG_FORMAT defaults to text.
func Init() {
	level := ParseLevel(os.Getenv("LOG_LEVEL"))
	format := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT")))
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level <= slog.LevelDebug,
	}
	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// Default returns the configured logger.
func Default() *slog.Logger {
	return logger
}

// ParseLevel maps a level name to slog.Level; unknown values fall back to info.
func ParseLevel(name string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(name)) {
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

// Debug logs at debug level.
func Debug(msg string, args ...any) {
	logger.Debug(msg, args...)
}

// Info logs at info level.
func Info(msg string, args ...any) {
	logger.Info(msg, args...)
}

// Warn logs at warn level.
func Warn(msg string, args ...any) {
	logger.Warn(msg, args...)
}

// Error logs at error level.
func Error(msg string, args ...any) {
	logger.Error(msg, args...)
}
