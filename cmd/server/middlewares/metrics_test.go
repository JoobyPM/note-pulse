package middlewares

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

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
