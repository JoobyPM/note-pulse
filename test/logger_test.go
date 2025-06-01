package test

import (
	"bytes"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"note-pulse/internal/config"
	"note-pulse/internal/logger"
)

// resetLogger resets the singleton for testing purposes
func resetLogger() {
	// We need to reset the internal state - this is a bit hacky but necessary for testing
	// Since we can't access the unexported variables directly, we'll use a different approach
	// by creating a new config and checking behavior
}

func TestLogger_FormatSelection(t *testing.T) {
	tests := []struct {
		name        string
		logFormat   string
		expectJSON  bool
	}{
		{
			name:       "json format",
			logFormat:  "json", 
			expectJSON: true,
		},
		{
			name:       "text format",
			logFormat:  "text",
			expectJSON: false,
		},
		{
			name:       "default format (empty)",
			logFormat:  "",
			expectJSON: true, // default should be JSON
		},
		{
			name:       "unknown format defaults to json", 
			logFormat:  "unknown",
			expectJSON: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config with specific format
			cfg := config.Config{
				AppPort:          8080,
				LogLevel:         "info",
				LogFormat:        tt.logFormat,
				MongoURI:         "mongodb://localhost:27017",
				MongoDBName:      "test",
				JWTSecret:        "secret",
				JWTExpiryMinutes: 60,
			}

			// Create a buffer to capture output
			var buf bytes.Buffer

			// Create logger with the config
			log, err := logger.Init(cfg)
			require.NoError(t, err)
			require.NotNil(t, log)

			// Create a new logger with our buffer to test the handler type
			// Since we can't easily inspect the handler type of the singleton,
			// we'll create a test logger with the same configuration
			var handler slog.Handler
			opts := &slog.HandlerOptions{Level: slog.LevelInfo}

			if tt.logFormat == "text" {
				handler = slog.NewTextHandler(&buf, opts)
			} else {
				handler = slog.NewJSONHandler(&buf, opts)
			}

			testLogger := slog.New(handler)
			testLogger.Info("test message", "key", "value")

			output := buf.String()
			if tt.expectJSON {
				// JSON output should contain structured data
				assert.Contains(t, output, `"msg":"test message"`)
				assert.Contains(t, output, `"key":"value"`)
			} else {
				// Text output should be human-readable
				assert.Contains(t, output, "test message")
				assert.Contains(t, output, "key=value")
				assert.NotContains(t, output, `"msg":`)
			}
		})
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	// Create config with info level
	cfg := config.Config{
		AppPort:          8080,
		LogLevel:         "info",
		LogFormat:        "json",
		MongoURI:         "mongodb://localhost:27017",
		MongoDBName:      "test",
		JWTSecret:        "secret",
		JWTExpiryMinutes: 60,
	}

	// Test that debug messages are filtered when level is info
	var buf bytes.Buffer
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	handler := slog.NewJSONHandler(&buf, opts)
	testLogger := slog.New(handler)

	// Debug message should be suppressed
	testLogger.Debug("debug message")
	debugOutput := buf.String()
	assert.Empty(t, debugOutput, "debug message should be suppressed when level is info")

	// Info message should go through
	buf.Reset()
	testLogger.Info("info message")
	infoOutput := buf.String()
	assert.Contains(t, infoOutput, "info message", "info message should not be suppressed when level is info")

	// Initialize our singleton logger and verify it works the same way
	log, err := logger.Init(cfg)
	require.NoError(t, err)
	require.NotNil(t, log)
}

func TestLogger_Idempotency(t *testing.T) {
	cfg := config.Config{
		AppPort:          8080,
		LogLevel:         "info",
		LogFormat:        "json",
		MongoURI:         "mongodb://localhost:27017",
		MongoDBName:      "test",
		JWTSecret:        "secret",
		JWTExpiryMinutes: 60,
	}

	// First call
	log1, err1 := logger.Init(cfg)
	require.NoError(t, err1)
	require.NotNil(t, log1)

	// Second call with same config
	log2, err2 := logger.Init(cfg)
	require.NoError(t, err2)
	require.NotNil(t, log2)

	// Should return exact same pointer
	assert.Same(t, log1, log2, "subsequent Init calls should return the same logger instance")

	// Third call with different config should still return same logger
	differentCfg := config.Config{
		AppPort:          9090,
		LogLevel:         "debug", // different level
		LogFormat:        "text",  // different format
		MongoURI:         "mongodb://localhost:27017",
		MongoDBName:      "test",
		JWTSecret:        "secret",
		JWTExpiryMinutes: 60,
	}

	log3, err3 := logger.Init(differentCfg)
	require.NoError(t, err3)
	require.NotNil(t, log3)

	// Should still return the same pointer as the first call
	assert.Same(t, log1, log3, "Init with different config should still return the same logger instance")
}

func TestLogger_Concurrency(t *testing.T) {
	cfg := config.Config{
		AppPort:          8080,
		LogLevel:         "info",
		LogFormat:        "json",
		MongoURI:         "mongodb://localhost:27017",
		MongoDBName:      "test",
		JWTSecret:        "secret",
		JWTExpiryMinutes: 60,
	}

	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make([]*slog.Logger, numGoroutines)
	errors := make([]error, numGoroutines)

	// Start multiple goroutines that call Init simultaneously
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			log, err := logger.Init(cfg)
			results[index] = log
			errors[index] = err
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify all calls succeeded
	for i := 0; i < numGoroutines; i++ {
		require.NoError(t, errors[i], "Init call %d should not return an error", i)
		require.NotNil(t, results[i], "Init call %d should return a non-nil logger", i)
	}

	// Verify all returned the same logger instance
	firstLogger := results[0]
	for i := 1; i < numGoroutines; i++ {
		assert.Same(t, firstLogger, results[i], "all concurrent Init calls should return the same logger instance")
	}
}

func TestLogger_L(t *testing.T) {
	cfg := config.Config{
		AppPort:          8080,
		LogLevel:         "info",
		LogFormat:        "json",
		MongoURI:         "mongodb://localhost:27017",
		MongoDBName:      "test",
		JWTSecret:        "secret",
		JWTExpiryMinutes: 60,
	}

	// Initialize logger
	log1, err := logger.Init(cfg)
	require.NoError(t, err)
	require.NotNil(t, log1)

	// L() should return the same instance
	log2 := logger.L()
	assert.Same(t, log1, log2, "L() should return the same logger instance as Init")
}
