package main

import (
	"os"
	"testing"

	"note-pulse/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestLoggingConfig(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{
			name:     "request logging disabled",
			envValue: "false",
			expected: false,
		},
		{
			name:     "request logging enabled",
			envValue: "true",
			expected: true,
		},
		{
			name:     "default value (no env var)",
			envValue: "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				_ = os.Unsetenv("REQUEST_LOGGING_ENABLED")
				_ = os.Unsetenv("DEV_MODE")
				config.ResetCache()
			}()

			if tt.envValue != "" {
				err := os.Setenv("REQUEST_LOGGING_ENABLED", tt.envValue)
				require.NoError(t, err)
			}

			// Set DEV_MODE=true to bypass JWT_SECRET requirement for tests
			err := os.Setenv("DEV_MODE", "true")
			require.NoError(t, err)

			config.ResetCache()

			cfg, err := config.Load()
			require.NoError(t, err)

			assert.Equal(t, tt.expected, cfg.RequestLoggingEnabled,
				"RequestLoggingEnabled should be %v when REQUEST_LOGGING_ENABLED=%s",
				tt.expected, tt.envValue)
		})
	}
}
