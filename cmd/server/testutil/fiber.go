package testutil

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"note-pulse/cmd/server/handlers/httperr"
	"note-pulse/internal/config"
	"note-pulse/internal/logger"
	"note-pulse/internal/utils/crypto"

	"github.com/go-playground/validator/v10"
	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// CreateTestApp creates a basic Fiber app for testing with common configuration
func CreateTestApp(t *testing.T) *fiber.App {
	cfg := config.Config{LogLevel: "debug", LogFormat: "text"}
	_, err := logger.Init(cfg)
	require.NoError(t, err)

	app := fiber.New(fiber.Config{
		ErrorHandler: httperr.Handler,
	})

	return app
}

// CreateTestValidator creates a validator with crypto password validation registered
func CreateTestValidator(t *testing.T) *validator.Validate {
	v := validator.New()
	err := crypto.RegisterPasswordValidator(v)
	require.NoError(t, err)
	return v
}

// CreateTestJWT creates a JWT token for testing purposes
func CreateTestJWT(userID string, email string, secret []byte, expiry time.Duration) (string, error) {
	now := time.Now().UTC()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"exp":     now.Add(expiry).Unix(),
		"iat":     now.Unix(),
	})

	return token.SignedString(secret)
}

// SetupJWTMiddleware sets up JWT middleware for testing with the given secret
func SetupJWTMiddleware(jwtSecret string) fiber.Handler {
	return jwtware.New(jwtware.Config{
		SigningKey: jwtware.SigningKey{Key: []byte(jwtSecret)},
		SuccessHandler: func(c *fiber.Ctx) error {
			// Extract claims
			token := c.Locals("user").(*jwt.Token)
			claims := token.Claims.(jwt.MapClaims)

			userID, ok := claims["user_id"].(string)
			if !ok {
				return httperr.Fail(httperr.E{Status: 401, Message: "Invalid token: missing user_id"})
			}
			userEmail, ok := claims["email"].(string)
			if !ok {
				return httperr.Fail(httperr.E{Status: 401, Message: "Invalid token: missing email"})
			}

			c.Locals("userID", userID)
			c.Locals("userEmail", userEmail)
			return c.Next()
		},
	})
}

// CreateRateLimiter creates a rate limiter for testing
func CreateRateLimiter(maxRequests int, duration time.Duration) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        maxRequests,
		Expiration: duration,
		LimitReached: func(c *fiber.Ctx) error {
			return httperr.Fail(httperr.ErrTooManyRequests)
		},
	})
}

// CreateJSONRequest creates an HTTP request with JSON body
func CreateJSONRequest(method, url string, body any) *http.Request {
	var reqBody []byte
	if body != nil {
		reqBody, _ = json.Marshal(body)
	}

	req := httptest.NewRequest(method, url, bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// CreateAuthenticatedRequest creates an HTTP request with Authorization header
func CreateAuthenticatedRequest(method, url string, body any, token string) *http.Request {
	req := CreateJSONRequest(method, url, body)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

// CreateWebSocketRequest creates an HTTP request with WebSocket upgrade headers
func CreateWebSocketRequest(url string, token *string) *http.Request {
	requestURL := url
	if token != nil {
		requestURL += "?token=" + *token
	}

	req := httptest.NewRequest("GET", requestURL, nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "test-key")
	return req
}
