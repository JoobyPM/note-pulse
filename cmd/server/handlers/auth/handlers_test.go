package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"note-pulse/cmd/server/handlers/httperr"
	"note-pulse/internal/config"
	"note-pulse/internal/logger"
	"note-pulse/internal/services/auth"
	"note-pulse/internal/utils/crypto"

	"github.com/go-playground/validator/v10"
	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// AuthServiceInterface defines the interface for auth service
type AuthServiceInterface interface {
	SignUp(ctx context.Context, req auth.SignUpRequest) (*auth.AuthResponse, error)
	SignIn(ctx context.Context, req auth.SignInRequest) (*auth.AuthResponse, error)
}

// MockAuthService mocks the auth service
type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) SignUp(ctx context.Context, req auth.SignUpRequest) (*auth.AuthResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.AuthResponse), args.Error(1)
}

func (m *MockAuthService) SignIn(ctx context.Context, req auth.SignInRequest) (*auth.AuthResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.AuthResponse), args.Error(1)
}

func (m *MockAuthService) Refresh(ctx context.Context, rawRefreshToken string) (*auth.AuthResponse, error) {
	args := m.Called(ctx, rawRefreshToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.AuthResponse), args.Error(1)
}

func (m *MockAuthService) SignOut(ctx context.Context, userID bson.ObjectID, rawRefreshToken string) error {
	args := m.Called(ctx, userID, rawRefreshToken)
	return args.Error(0)
}

func (m *MockAuthService) SignOutAll(ctx context.Context, userID bson.ObjectID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func setupTestApp(authService *MockAuthService) *fiber.App {
	cfg := config.Config{LogLevel: "debug", LogFormat: "text"}
	if _, err := logger.Init(cfg); err != nil {
		panic(err)
	}

	v := validator.New()
	if err := crypto.RegisterPasswordValidator(v); err != nil {
		panic(err)
	}

	app := fiber.New(fiber.Config{
		ErrorHandler: httperr.Handler,
	})

	h := NewHandlers(authService, v)

	v1 := app.Group("/api/v1")
	authGrp := v1.Group("/auth")

	// Rate limiter for sign-in (for testing)
	limiterMW := limiter.New(limiter.Config{
		Max:        2, // allow only 2 requests for testing
		Expiration: 1 * time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return httperr.Fail(httperr.ErrTooManyRequests)
		},
	})

	authGrp.Post("/sign-up", h.SignUp)
	authGrp.Post("/sign-in", limiterMW, h.SignIn)

	return app
}

func setupTestAppWithJWT(authService *MockAuthService) *fiber.App {
	cfg := config.Config{LogLevel: "debug", LogFormat: "text"}
	if _, err := logger.Init(cfg); err != nil {
		panic(err)
	}

	v := validator.New()
	if err := crypto.RegisterPasswordValidator(v); err != nil {
		panic(err)
	}

	app := fiber.New(fiber.Config{
		ErrorHandler: httperr.Handler,
	})

	h := NewHandlers(authService, v)

	v1 := app.Group("/api/v1")
	authGrp := v1.Group("/auth")

	authGrp.Post("/sign-up", h.SignUp)
	authGrp.Post("/sign-in", h.SignIn)

	// JWT middleware and protected route for testing
	jwtSecret := "test-secret-with-32-plus-characters"
	jwtMW := jwtware.New(jwtware.Config{
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

	protected := v1.Group("/me", jwtMW)
	protected.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"uid":   c.Locals("userID"),
			"email": c.Locals("userEmail"),
		})
	})

	return app
}

func TestSignUp_Success(t *testing.T) {
	mockService := &MockAuthService{}
	app := setupTestApp(mockService)
	now := time.Now().UTC()

	user := &auth.User{
		ID:        bson.NewObjectID(),
		Email:     "test@example.com",
		CreatedAt: now,
		UpdatedAt: now,
	}
	expected := &auth.AuthResponse{User: user, Token: "mock-jwt-token"}

	mockService.On("SignUp", mock.Anything, auth.SignUpRequest{
		Email:    "test@example.com",
		Password: "Password123",
	}).Return(expected, nil).Once()

	body, _ := json.Marshal(map[string]string{
		"email":    "test@example.com",
		"password": "Password123",
	})

	req := httptest.NewRequest("POST", "/api/v1/auth/sign-up", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 201, resp.StatusCode)

	var got auth.AuthResponse
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.Equal(t, expected.User.Email, got.User.Email)
	assert.Equal(t, expected.Token, got.Token)

	mockService.AssertExpectations(t)
}

func TestJWTMiddleware_HappyPath(t *testing.T) {
	mockService := &MockAuthService{}
	app := setupTestAppWithJWT(mockService)
	now := time.Now().UTC()

	jwtSecret := "test-secret-with-32-plus-characters"
	claims := jwt.MapClaims{
		"user_id": "60d5ecb74b24c4f9b8c2b1a1",
		"email":   "test@example.com",
		"exp":     now.Add(time.Hour).Unix(),
		"iat":     now.Unix(),
	}

	tkn := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tknStr, err := tkn.SignedString([]byte(jwtSecret))
	assert.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/me/", nil)
	req.Header.Set("Authorization", "Bearer "+tknStr)

	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var got map[string]any
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.Equal(t, "60d5ecb74b24c4f9b8c2b1a1", got["uid"])
	assert.Equal(t, "test@example.com", got["email"])

	mockService.AssertExpectations(t)
}

func TestSignUp_DuplicateEmail(t *testing.T) {
	mockService := &MockAuthService{}
	app := setupTestApp(mockService)

	mockService.On("SignUp", mock.Anything, auth.SignUpRequest{
		Email:    "existing@example.com",
		Password: "Password123",
	}).Return(nil, auth.ErrRegistrationFailed).Once()

	body, _ := json.Marshal(map[string]string{
		"email":    "existing@example.com",
		"password": "Password123",
	})

	req := httptest.NewRequest("POST", "/api/v1/auth/sign-up", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)

	mockService.AssertExpectations(t)
}

func TestSignIn_Success(t *testing.T) {
	mockService := &MockAuthService{}
	app := setupTestApp(mockService)
	now := time.Now().UTC()

	user := &auth.User{
		ID:        bson.NewObjectID(),
		Email:     "test@example.com",
		CreatedAt: now,
		UpdatedAt: now,
	}
	expected := &auth.AuthResponse{User: user, Token: "mock-jwt-token"}

	mockService.On("SignIn", mock.Anything, auth.SignInRequest{
		Email:    "test@example.com",
		Password: "Password123",
	}).Return(expected, nil).Once()

	body, _ := json.Marshal(map[string]string{
		"email":    "test@example.com",
		"password": "Password123",
	})

	req := httptest.NewRequest("POST", "/api/v1/auth/sign-in", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var got auth.AuthResponse
	assert.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.Equal(t, expected.User.Email, got.User.Email)
	assert.Equal(t, expected.Token, got.Token)

	mockService.AssertExpectations(t)
}

func TestSignIn_BadCredentials(t *testing.T) {
	mockService := &MockAuthService{}
	app := setupTestApp(mockService)

	mockService.On("SignIn", mock.Anything, auth.SignInRequest{
		Email:    "test@example.com",
		Password: "wrongpassword",
	}).Return(nil, auth.ErrInvalidCredentials).Once()

	body, _ := json.Marshal(map[string]string{
		"email":    "test@example.com",
		"password": "wrongpassword",
	})

	req := httptest.NewRequest("POST", "/api/v1/auth/sign-in", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)

	mockService.AssertExpectations(t)
}

func TestSignIn_RateLimit(t *testing.T) {
	mockService := &MockAuthService{}
	app := setupTestApp(mockService)
	now := time.Now().UTC()

	user := &auth.User{
		ID:        bson.NewObjectID(),
		Email:     "test@example.com",
		CreatedAt: now,
		UpdatedAt: now,
	}
	expected := &auth.AuthResponse{User: user, Token: "mock-jwt-token"}

	mockService.On("SignIn", mock.Anything, auth.SignInRequest{
		Email:    "test@example.com",
		Password: "Password123",
	}).Return(expected, nil).Times(2)

	body, _ := json.Marshal(map[string]string{
		"email":    "test@example.com",
		"password": "Password123",
	})

	req1 := httptest.NewRequest("POST", "/api/v1/auth/sign-in", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Forwarded-For", "192.168.1.1") // fixed IP for rate limiter

	resp1, err := app.Test(req1, -1)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp1.StatusCode)

	req2 := httptest.NewRequest("POST", "/api/v1/auth/sign-in", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Forwarded-For", "192.168.1.1")

	resp2, err := app.Test(req2, -1)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp2.StatusCode)

	req3 := httptest.NewRequest("POST", "/api/v1/auth/sign-in", bytes.NewReader(body))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("X-Forwarded-For", "192.168.1.1")

	resp3, err := app.Test(req3, -1)
	assert.NoError(t, err)
	assert.Equal(t, 429, resp3.StatusCode)

	mockService.AssertExpectations(t)
}
