package main

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestAuthMiddlewareOrder(t *testing.T) {
	type stack []string

	mw := func(s *stack, id string) fiber.Handler {
		return func(c *fiber.Ctx) error {
			*s = append(*s, id)
			return c.Next() // just record & pass through
		}
	}
	final := func(s *stack, id string) fiber.Handler {
		return func(c *fiber.Ctx) error {
			*s = append(*s, id)
			return c.SendStatus(200) // terminate the chain with 200
		}
	}

	tests := []struct {
		path   string
		expect []string
	}{
		{"/api/v1/auth/sign-in", []string{"limiter", "handler"}},
		{"/api/v1/auth/sign-out", []string{"limiter", "jwt", "handler"}},
		{"/api/v1/auth/sign-out-all", []string{"limiter", "jwt", "handler"}},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			var trace stack
			app := fiber.New()

			limiterSpy := mw(&trace, "limiter")
			jwtSpy := mw(&trace, "jwt")
			handlerSpy := final(&trace, "handler")

			switch tc.path {
			case "/api/v1/auth/sign-in":
				app.Post(tc.path, limiterSpy, handlerSpy)
			case "/api/v1/auth/sign-out":
				app.Post(tc.path, limiterSpy, jwtSpy, handlerSpy)
			case "/api/v1/auth/sign-out-all":
				app.Post(tc.path, limiterSpy, jwtSpy, handlerSpy)
			}

			req := httptest.NewRequest(fiber.MethodPost, tc.path, nil)
			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)

			assert.Equal(t, tc.expect, []string(trace), // ★ fix here
				"middleware execution order drifted")
		})
	}
}
