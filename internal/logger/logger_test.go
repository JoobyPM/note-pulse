package logger

import (
	"bytes"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"note-pulse/internal/config"
)

func TestLogger_FormatSelection(t *testing.T) {
	tests := []struct {
		name       string
		logFormat  string
		expectJSON bool
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
			expectJSON: true,
		},
		{
			name:       "unknown format defaults to json",
			logFormat:  "unknown",
			expectJSON: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{
				AppPort:     8080,
				LogLevel:    "info",
				LogFormat:   tt.logFormat,
				MongoURI:    "mongodb://localhost:27017",
				MongoDBName: "test",
				JWTSecret:   "secret",
			}

			var buf bytes.Buffer

			log, err := Init(cfg)
			require.NoError(t, err)
			require.NotNil(t, log)

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
				assert.Contains(t, output, `"msg":"test message"`)
				assert.Contains(t, output, `"key":"value"`)
			} else {
				assert.Contains(t, output, "test message")
				assert.Contains(t, output, "key=value")
				assert.NotContains(t, output, `"msg":`)
			}
		})
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	cfg := config.Config{
		AppPort:     8080,
		LogLevel:    "info",
		LogFormat:   "json",
		MongoURI:    "mongodb://localhost:27017",
		MongoDBName: "test",
		JWTSecret:   "secret",
	}

	var buf bytes.Buffer
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	handler := slog.NewJSONHandler(&buf, opts)
	testLogger := slog.New(handler)

	testLogger.Debug("debug message")
	debugOutput := buf.String()
	assert.Empty(t, debugOutput, "debug message should be suppressed when level is info")

	buf.Reset()
	testLogger.Info("info message")
	infoOutput := buf.String()
	assert.Contains(t, infoOutput, "info message", "info message should not be suppressed when level is info")

	log, err := Init(cfg)
	require.NoError(t, err)
	require.NotNil(t, log)
}

func TestLogger_Idempotency(t *testing.T) {
	cfg := config.Config{
		AppPort:     8080,
		LogLevel:    "info",
		LogFormat:   "json",
		MongoURI:    "mongodb://localhost:27017",
		MongoDBName: "test",
		JWTSecret:   "secret",
	}

	log1, err1 := Init(cfg)
	require.NoError(t, err1)
	require.NotNil(t, log1)

	log2, err2 := Init(cfg)
	require.NoError(t, err2)
	require.NotNil(t, log2)

	assert.Same(t, log1, log2, "subsequent Init calls should return the same logger instance")

	differentCfg := config.Config{
		AppPort:     9090,
		LogLevel:    "debug",
		LogFormat:   "text",
		MongoURI:    "mongodb://localhost:27017",
		MongoDBName: "test",
		JWTSecret:   "secret",
	}

	log3, err3 := Init(differentCfg)
	require.NoError(t, err3)
	require.NotNil(t, log3)

	assert.Same(t, log1, log3, "Init with different config should still return the same logger instance")
}

func TestLogger_Concurrency(t *testing.T) {
	cfg := config.Config{
		AppPort:     8080,
		LogLevel:    "info",
		LogFormat:   "json",
		MongoURI:    "mongodb://localhost:27017",
		MongoDBName: "test",
		JWTSecret:   "secret",
	}

	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make([]*slog.Logger, numGoroutines)
	errors := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			log, err := Init(cfg)
			results[index] = log
			errors[index] = err
		}(i)
	}

	wg.Wait()

	for i := 0; i < numGoroutines; i++ {
		require.NoError(t, errors[i], "Init call %d should not return an error", i)
		require.NotNil(t, results[i], "Init call %d should return a non-nil logger", i)
	}

	firstLogger := results[0]
	for i := 1; i < numGoroutines; i++ {
		assert.Same(t, firstLogger, results[i], "all concurrent Init calls should return the same logger instance")
	}
}

func TestLogger_L(t *testing.T) {
	cfg := config.Config{
		AppPort:     8080,
		LogLevel:    "info",
		LogFormat:   "json",
		MongoURI:    "mongodb://localhost:27017",
		MongoDBName: "test",
		JWTSecret:   "secret",
	}

	log1, err := Init(cfg)
	require.NoError(t, err)
	require.NotNil(t, log1)

	log2 := L()
	assert.Same(t, log1, log2, "L() should return the same logger instance as Init")
}
