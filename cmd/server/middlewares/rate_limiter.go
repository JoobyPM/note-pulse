package middlewares

import (
	"strings"
	"time"

	"note-pulse/cmd/server/handlers/httperr"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// BuildRateLimiter returns a Fiber handler that does *nothing* when max <= 0
// so callers don't need to wrap it in an if-statement.
//
//	max           — requests per Expiration window
//	expiration    — bucket window
//	skipPrefixes  — requests whose path *starts* with any of these prefixes
//	                bypass the limiter (handy for auth-only or health-checks).
func BuildRateLimiter(max int, expiration time.Duration, skipPrefixes ...string) fiber.Handler {
	if max <= 0 {
		// disabled -> just fall through
		return func(c *fiber.Ctx) error { return c.Next() }
	}

	cfg := limiter.Config{
		Max:        max,
		Expiration: expiration,
		LimitReached: func(c *fiber.Ctx) error {
			return httperr.Fail(httperr.ErrTooManyRequests)
		},
	}

	if len(skipPrefixes) > 0 {
		cfg.Next = func(c *fiber.Ctx) bool {
			for _, p := range skipPrefixes {
				if strings.HasPrefix(c.Path(), p) {
					return true // skip limiter
				}
			}
			return false
		}
	}

	return limiter.New(cfg)
}
