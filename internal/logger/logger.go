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

		opts := &slog.HandlerOptions{
			Level: level,
		}

		var handler slog.Handler
		switch cfg.LogFormat {
		case "text":
			handler = slog.NewTextHandler(os.Stdout, opts)
		case "json":
			fallthrough
		default:
			handler = slog.NewJSONHandler(os.Stdout, opts)
		}

		singleton = slog.New(handler)
	})

	return singleton, initErr
}

// L returns the singleton logger instance.
func L() *slog.Logger {
	return singleton
}

// InitTest initializes a test logger with sane defaults for testing.
// This should only be used in test files.
func InitTest() error {
	var initErr error

	once.Do(func() {
		opts := &slog.HandlerOptions{
			Level: slog.LevelError, // Minimize noise in tests
		}

		// Use text handler for easier debugging in tests
		handler := slog.NewTextHandler(os.Stdout, opts)
		singleton = slog.New(handler)
	})

	return initErr
}

// WithReq creates a logger with consistent request context fields
func WithReq(c any, handler string) *slog.Logger {
	logger := L()
	if logger == nil {
		return slog.Default()
	}

	switch ctx := c.(type) {
	case interface{ Path() string }:
		// Fiber context
		logCtx := logger.With("handler", handler, "path", ctx.Path())

		// Add user_id if available
		if userCtx, ok := ctx.(interface{ Locals(key string) any }); ok {
			if userID, exists := userCtx.Locals("userID").(string); exists {
				logCtx = logCtx.With("user_id", userID)
			}
		}

		return logCtx
	default:
		// Fallback for other context types
		return logger.With("handler", handler)
	}
}
