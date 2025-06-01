package logger

import (
	"log/slog"
	"os"
	"sync"

	"note-pulse/internal/config"
)

var (
	singleton *slog.Logger
	once      sync.Once
)

// Init initializes the singleton logger from the provided config.
// It is thread-safe and idempotent - the first successful call wins,
// and subsequent calls return the same logger instance.
func Init(cfg config.Config) (*slog.Logger, error) {
	var initErr error
	
	once.Do(func() {
		// Parse log level
		var level slog.Level
		switch cfg.LogLevel {
		case "debug":
			level = slog.LevelDebug
		case "info":
			level = slog.LevelInfo
		case "warn":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		default:
			level = slog.LevelInfo
		}

		// Create handler options
		opts := &slog.HandlerOptions{
			Level: level,
		}

		// Create handler based on format
		var handler slog.Handler
		switch cfg.LogFormat {
		case "text":
			handler = slog.NewTextHandler(os.Stdout, opts)
		case "json":
			fallthrough
		default:
			handler = slog.NewJSONHandler(os.Stdout, opts)
		}

		// Create the logger
		singleton = slog.New(handler)
	})

	return singleton, initErr
}

// L returns the singleton logger instance.
// Init must be called first, otherwise this will return nil.
func L() *slog.Logger {
	return singleton
}
