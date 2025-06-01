package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"note-pulse/internal/handlers/httperr"
	"note-pulse/internal/services/auth"
	"note-pulse/internal/utils/crypto"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
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

func setupTestApp(authService *MockAuthService) *fiber.App {
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
		Max:        2, // Allow only 2 requests for testing
		Expiration: 1 * time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return httperr.Fail(httperr.ErrTooManyRequests)
		},
	})

	authGrp.Post("/sign-up", h.SignUp)
	authGrp.Post("/sign-in", limiterMW, h.SignIn)

	return app
}

func TestSignUp_Success(t *testing.T) {
	mockService := &MockAuthService{}
	app := setupTestApp(mockService)

	user := &auth.User{
		ID:        bson.NewObjectID(),
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	expectedResponse := &auth.AuthResponse{
		User:  user,
		Token: "mock-jwt-token",
	}

	mockService.On("SignUp", mock.Anything, auth.SignUpRequest{
		Email:    "test@example.com",
		Password: "Password123",
	}).Return(expectedResponse, nil)

	reqBody := map[string]string{
		"email":    "test@example.com",
		"password": "Password123",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/auth/sign-up", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 201, resp.StatusCode)

	var responseBody auth.AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&responseBody); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, expectedResponse.User.Email, responseBody.User.Email)
	assert.Equal(t, expectedResponse.Token, responseBody.Token)

	mockService.AssertExpectations(t)
}

func TestSignUp_DuplicateEmail(t *testing.T) {
	mockService := &MockAuthService{}
	app := setupTestApp(mockService)

	// Setup mock to return duplicate error (masked as registration failed)
	mockService.On("SignUp", mock.Anything, auth.SignUpRequest{
		Email:    "existing@example.com",
		Password: "Password123",
	}).Return(nil, errors.New("registration failed"))

	// Create request
	reqBody := map[string]string{
		"email":    "existing@example.com",
		"password": "Password123",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/auth/sign-up", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode, "should return 400, not 409 - masked as 400")
	mockService.AssertExpectations(t)
}

func TestSignIn_Success(t *testing.T) {
	mockService := &MockAuthService{}
	app := setupTestApp(mockService)

	user := &auth.User{
		ID:        bson.NewObjectID(),
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	expectedResponse := &auth.AuthResponse{
		User:  user,
		Token: "mock-jwt-token",
	}

	mockService.On("SignIn", mock.Anything, auth.SignInRequest{
		Email:    "test@example.com",
		Password: "Password123",
	}).Return(expectedResponse, nil)

	reqBody := map[string]string{
		"email":    "test@example.com",
		"password": "Password123",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/auth/sign-in", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var responseBody auth.AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&responseBody); err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, expectedResponse.User.Email, responseBody.User.Email)
	assert.Equal(t, expectedResponse.Token, responseBody.Token)

	mockService.AssertExpectations(t)
}

func TestSignIn_BadCredentials(t *testing.T) {
	mockService := &MockAuthService{}
	app := setupTestApp(mockService)

	mockService.On("SignIn", mock.Anything, auth.SignInRequest{
		Email:    "test@example.com",
		Password: "wrongpassword",
	}).Return(nil, errors.New("invalid credentials"))

	reqBody := map[string]string{
		"email":    "test@example.com",
		"password": "wrongpassword",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/v1/auth/sign-in", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)

	mockService.AssertExpectations(t)
}

func TestSignIn_RateLimit(t *testing.T) {
	mockService := &MockAuthService{}
	app := setupTestApp(mockService)

	user := &auth.User{
		ID:        bson.NewObjectID(),
		Email:     "test@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	expectedResponse := &auth.AuthResponse{
		User:  user,
		Token: "mock-jwt-token",
	}

	mockService.On("SignIn", mock.Anything, auth.SignInRequest{
		Email:    "test@example.com",
		Password: "Password123",
	}).Return(expectedResponse, nil)

	reqBody := map[string]string{
		"email":    "test@example.com",
		"password": "Password123",
	}
	jsonBody, _ := json.Marshal(reqBody)

	req1 := httptest.NewRequest("POST", "/api/v1/auth/sign-in", bytes.NewReader(jsonBody))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-Forwarded-For", "192.168.1.1") // Set IP for rate limiting

	resp1, err := app.Test(req1, -1)
	assert.NoError(t, err, "first request should succeed")
	assert.Equal(t, 200, resp1.StatusCode, "first request should succeed")

	req2 := httptest.NewRequest("POST", "/api/v1/auth/sign-in", bytes.NewReader(jsonBody))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-Forwarded-For", "192.168.1.1") // Same IP

	resp2, err := app.Test(req2, -1)
	assert.NoError(t, err, "second request should succeed")
	assert.Equal(t, 200, resp2.StatusCode, "second request should succeed")

	req3 := httptest.NewRequest("POST", "/api/v1/auth/sign-in", bytes.NewReader(jsonBody))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("X-Forwarded-For", "192.168.1.1") // Same IP

	resp3, err := app.Test(req3, -1)
	assert.NoError(t, err, "third request should fail")
	assert.Equal(t, 429, resp3.StatusCode, "third request should fail")

	mockService.AssertExpectations(t)
}
