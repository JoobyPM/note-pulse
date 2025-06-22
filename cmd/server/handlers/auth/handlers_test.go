package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"note-pulse/cmd/server/ctxkeys"
	"note-pulse/cmd/server/testutil"
	"note-pulse/internal/services/auth"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	signUpEndpoint = "/api/v1/auth/sign-up"
	signInEndpoint = "/api/v1/auth/sign-in"
	meEndpoint     = "/api/v1/me"
	rateLimitIP    = "192.168.1.1"
	testEmail      = "test@example.com"
	testPassword   = "Password123"
)

// MockAuthService mocks the auth service
type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) SignUp(ctx context.Context, req auth.SignUpRequest) (*auth.Response, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.Response), args.Error(1)
}

func (m *MockAuthService) SignIn(ctx context.Context, req auth.SignInRequest) (*auth.Response, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.Response), args.Error(1)
}

func (m *MockAuthService) Refresh(ctx context.Context, rawRefreshToken string) (*auth.Response, error) {
	args := m.Called(ctx, rawRefreshToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.Response), args.Error(1)
}

func (m *MockAuthService) SignOut(ctx context.Context, userID bson.ObjectID, rawRefreshToken string) error {
	args := m.Called(ctx, userID, rawRefreshToken)
	return args.Error(0)
}

func (m *MockAuthService) SignOutAll(ctx context.Context, userID bson.ObjectID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

// AuthTestSetup contains common test setup data
type AuthTestSetup struct {
	MockService *MockAuthService
	App         *fiber.App
	TestUser    *auth.User
	TestToken   string
}

// SetupAuthTest creates a common auth test setup
func SetupAuthTest(t *testing.T) *AuthTestSetup {
	t.Helper()

	mockService := &MockAuthService{}
	app := testutil.CreateTestApp(t)
	validator := testutil.CreateTestValidator(t)

	h := NewHandlers(mockService, validator)

	v1 := app.Group("/api/v1")
	authGrp := v1.Group("/auth")

	// Add rate limiter for sign-in (for testing)
	rateLimiter := testutil.CreateRateLimiter(2, 1*time.Minute)

	authGrp.Post("/sign-up", h.SignUp)
	authGrp.Post("/sign-in", rateLimiter, h.SignIn)

	now := time.Now().UTC()
	testUser := &auth.User{
		ID:        bson.NewObjectID(),
		Email:     testEmail,
		CreatedAt: now,
		UpdatedAt: now,
	}

	return &AuthTestSetup{
		MockService: mockService,
		App:         app,
		TestUser:    testUser,
		TestToken:   "mock-jwt-token",
	}
}

// SetupAuthTestWithJWT creates auth test setup with JWT middleware
func SetupAuthTestWithJWT(t *testing.T) *AuthTestSetup {
	t.Helper()

	mockService := &MockAuthService{}
	app := testutil.CreateTestApp(t)
	validator := testutil.CreateTestValidator(t)

	h := NewHandlers(mockService, validator)

	v1 := app.Group("/api/v1")
	authGrp := v1.Group("/auth")

	authGrp.Post("/sign-up", h.SignUp)
	authGrp.Post("/sign-in", h.SignIn)

	// JWT middleware and protected route for testing
	jwtSecret := "test-secret-with-32-plus-characters"
	jwtMW := testutil.SetupJWTMiddleware(jwtSecret)

	protected := v1.Group("/me", jwtMW)
	protected.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"uid":   c.Locals(ctxkeys.UserIDKey),
			"email": c.Locals(ctxkeys.UserEmailKey),
		})
	})

	now := time.Now().UTC()
	testUser := &auth.User{
		ID:        bson.NewObjectID(),
		Email:     testEmail,
		CreatedAt: now,
		UpdatedAt: now,
	}

	return &AuthTestSetup{
		MockService: mockService,
		App:         app,
		TestUser:    testUser,
		TestToken:   "mock-jwt-token",
	}
}

func TestAuthHandlersTableDriven(t *testing.T) {
	testCases := []struct {
		name           string
		endpoint       string
		method         string
		body           map[string]string
		setupMock      func(*MockAuthService, *auth.User, string)
		expectedStatus int
		expectedError  error
	}{
		{
			name:     "SignUp_Success",
			endpoint: signUpEndpoint,
			method:   "POST",
			body: map[string]string{
				"email":    testEmail,
				"password": testPassword,
			},
			setupMock: func(m *MockAuthService, user *auth.User, token string) {
				expected := &auth.Response{User: user, Token: token}
				m.On("SignUp", mock.Anything, auth.SignUpRequest{
					Email:    testEmail,
					Password: testPassword,
				}).Return(expected, nil).Once()
			},
			expectedStatus: 201,
		},
		{
			name:     "SignUp_DuplicateEmail",
			endpoint: signUpEndpoint,
			method:   "POST",
			body: map[string]string{
				"email":    testEmail,
				"password": testPassword,
			},
			setupMock: func(m *MockAuthService, user *auth.User, token string) {
				m.On("SignUp", mock.Anything, auth.SignUpRequest{
					Email:    testEmail,
					Password: testPassword,
				}).Return(nil, auth.ErrRegistrationFailed).Once()
			},
			expectedStatus: 400,
		},
		{
			name:     "SignIn_Success",
			endpoint: signInEndpoint,
			method:   "POST",
			body: map[string]string{
				"email":    testEmail,
				"password": testPassword,
			},
			setupMock: func(m *MockAuthService, user *auth.User, token string) {
				expected := &auth.Response{User: user, Token: token}
				m.On("SignIn", mock.Anything, auth.SignInRequest{
					Email:    testEmail,
					Password: testPassword,
				}).Return(expected, nil).Once()
			},
			expectedStatus: 200,
		},
		{
			name:     "SignIn_BadCredentials",
			endpoint: signInEndpoint,
			method:   "POST",
			body: map[string]string{
				"email":    testEmail,
				"password": testPassword,
			},
			setupMock: func(m *MockAuthService, user *auth.User, token string) {
				m.On("SignIn", mock.Anything, auth.SignInRequest{
					Email:    testEmail,
					Password: testPassword,
				}).Return(nil, auth.ErrInvalidCredentials).Once()
			},
			expectedStatus: 401,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setup := SetupAuthTest(t)
			tc.setupMock(setup.MockService, setup.TestUser, setup.TestToken)

			req := testutil.CreateJSONRequest(tc.method, tc.endpoint, tc.body)
			resp, err := setup.App.Test(req, -1)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, resp.StatusCode)

			if tc.expectedStatus < 400 {
				var got auth.Response
				require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
				assert.Equal(t, setup.TestUser.Email, got.User.Email)
				assert.Equal(t, setup.TestToken, got.Token)
			}

			setup.MockService.AssertExpectations(t)
		})
	}
}

func TestJWTMiddlewareHappyPath(t *testing.T) {
	setup := SetupAuthTestWithJWT(t)

	jwtSecret := "test-secret-with-32-plus-characters"
	userID := "60d5ecb74b24c4f9b8c2b1a1"
	email := "test@example.com"

	token, err := testutil.CreateTestJWT(userID, email, []byte(jwtSecret), time.Hour)
	require.NoError(t, err)

	req := testutil.CreateAuthenticatedRequest("GET", meEndpoint, nil, token)
	resp, err := setup.App.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var got map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	assert.Equal(t, userID, got["uid"])
	assert.Equal(t, email, got["email"])

	setup.MockService.AssertExpectations(t)
}

func makeTestRequestForRateLimit(setup *AuthTestSetup, body map[string]string) (resp *http.Response, err error) {
	req := testutil.CreateJSONRequest("POST", signInEndpoint, body)
	req.Header.Set("X-Forwarded-For", rateLimitIP) // fixed IP for rate limiter
	resp, err = setup.App.Test(req, -1)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func TestSignInRateLimit(t *testing.T) {
	setup := SetupAuthTest(t)

	expected := &auth.Response{User: setup.TestUser, Token: setup.TestToken}
	setup.MockService.On("SignIn", mock.Anything, auth.SignInRequest{
		Email:    testEmail,
		Password: testPassword,
	}).Return(expected, nil).Times(2)

	body := map[string]string{
		"email":    testEmail,
		"password": testPassword,
	}

	// First request should succeed
	resp1, err := makeTestRequestForRateLimit(setup, body)
	require.NoError(t, err)
	assert.Equal(t, 200, resp1.StatusCode)

	// Second request should succeed
	resp2, err := makeTestRequestForRateLimit(setup, body)
	require.NoError(t, err)
	assert.Equal(t, 200, resp2.StatusCode)

	// Third request should be rate limited
	resp3, err := makeTestRequestForRateLimit(setup, body)
	require.NoError(t, err)
	assert.Equal(t, 429, resp3.StatusCode)

	setup.MockService.AssertExpectations(t)
}
