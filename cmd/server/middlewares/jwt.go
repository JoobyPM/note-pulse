package middlewares

import (
	"note-pulse/internal/config"
	"note-pulse/internal/services/auth"

	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
)

// JWT returns a configured Fiber middleware that:
//
//   - validates the Bearer token signature using cfg.JWTSecret
//   - makes sure the token carries "user_id" and "email" claims
//   - stores those values in ctx.Locals("userID") / ctx.Locals("userEmail") so
//     downstream handlers can trust them.
//
// On any problem it bubbles up a 401 via the global httperr handler.
func JWT(cfg config.Config) fiber.Handler {
	return jwtware.New(jwtware.Config{
		SigningKey: jwtware.SigningKey{Key: []byte(cfg.JWTSecret)},
		SuccessHandler: func(c *fiber.Ctx) error {
			// Token already verified at this point.
			token := c.Locals("user").(*jwt.Token)
			claims, _ := token.Claims.(jwt.MapClaims)

			userID, ok := claims["user_id"].(string)
			if !ok || userID == "" {
				return auth.ErrInvalidTokenMissingUserID
			}

			userEmail, ok := claims["email"].(string)
			if !ok || userEmail == "" {
				return auth.ErrInvalidTokenMissingEmail
			}

			c.Locals("userID", userID)
			c.Locals("userEmail", userEmail)
			return c.Next()
		},

		// Override the default "unauthorized" JSON to match the project style
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return auth.ErrUnauthorized(err)
		},
	})
}
