package main

import (
	"net/http/httptest"
	"os"
	"testing"

	"note-pulse/internal/config"

	"github.com/gofiber/fiber/v2"
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

func TestNormalizeRoutePath(t *testing.T) {
	t.Run("matched route returns template", func(t *testing.T) {
		app := fiber.New()
		app.Get("/notes/:id", func(c *fiber.Ctx) error {
			path := normalizeRoutePath(c)
			assert.Equal(t, "/notes/:id", path, "should return route template")
			return c.SendString("ok")
		})

		req := httptest.NewRequest("GET", "/notes/abc123", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err, "request should succeed")
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("unmatched route returns actual path without panic", func(t *testing.T) {
		app := fiber.New()

		// Set up a catch-all middleware to test unmatched routes
		app.Use(func(c *fiber.Ctx) error {
			path := normalizeRoutePath(c)
			// For unmatched routes, c.Route() is nil, so we should get c.Path()
			// The exact path may vary based on Fiber's internal routing behavior
			assert.NotEmpty(t, path, "should return some path value")
			return c.SendStatus(404)
		})

		req := httptest.NewRequest("GET", "/nonexistent", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err, "request should not panic")
		assert.Equal(t, 404, resp.StatusCode)
	})
}
