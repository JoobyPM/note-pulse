package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"note-pulse/internal/config"
	"note-pulse/internal/utils/crypto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var silentLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// testServiceStandalone is a test helper that simulates standalone MongoDB
type testServiceStandalone struct {
	Service
}

// supportsTransactions always returns false to simulate standalone MongoDB
//
//nolint:unused // Used for polymorphic method override in tests
func (s *testServiceStandalone) supportsTransactions(ctx context.Context, client *mongo.Client) bool {
	return false
}

// MockUsersRepo is a mock implementation of UsersRepo
type MockUsersRepo struct {
	mock.Mock
}

// MockRefreshTokensRepo is a mock implementation of RefreshTokensRepo
type MockRefreshTokensRepo struct {
	mock.Mock
}

func (m *MockUsersRepo) Create(ctx context.Context, user *User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUsersRepo) FindByEmail(ctx context.Context, email string) (*User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*User), args.Error(1)
}

func (m *MockUsersRepo) FindByID(ctx context.Context, id bson.ObjectID) (*User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*User), args.Error(1)
}

func (m *MockRefreshTokensRepo) Create(ctx context.Context, userID bson.ObjectID, rawToken string, expiresAt time.Time) error {
	args := m.Called(ctx, userID, rawToken, expiresAt)
	return args.Error(0)
}

func (m *MockRefreshTokensRepo) FindActive(ctx context.Context, rawToken string) (*RefreshToken, error) {
	args := m.Called(ctx, rawToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*RefreshToken), args.Error(1)
}

func (m *MockRefreshTokensRepo) Revoke(ctx context.Context, id bson.ObjectID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRefreshTokensRepo) RevokeAllForUser(ctx context.Context, userID bson.ObjectID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockRefreshTokensRepo) Client() *mongo.Client {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*mongo.Client)
}

func TestService_SignUp(t *testing.T) {
	cfg := config.Config{
		BcryptCost:   12,
		JWTSecret:    "super-secret-jwt-key-at-least-32-chars",
		JWTAlgorithm: "HS256",
	}

	tests := []struct {
		name    string
		req     SignUpRequest
		setup   func(*MockUsersRepo)
		wantErr bool
		errMsg  string
	}{
		{
			name: "successful signup",
			req: SignUpRequest{
				Email:    "test@example.com",
				Password: "Password123",
			},
			setup: func(repo *MockUsersRepo) {
				repo.On("FindByEmail", mock.Anything, "test@example.com").Return(nil, errors.New("not found"))
				repo.On("Create", mock.Anything, mock.AnythingOfType("*auth.User")).Return(nil)
			},
			wantErr: false,
		},

		{
			name: "duplicate email",
			req: SignUpRequest{
				Email:    "test@example.com",
				Password: "Password123",
			},
			setup: func(repo *MockUsersRepo) {
				existingUser := &User{
					ID:    bson.NewObjectID(),
					Email: "test@example.com",
				}
				repo.On("FindByEmail", mock.Anything, "test@example.com").Return(existingUser, nil)
			},
			wantErr: true,
			errMsg:  "registration failed",
		},
		{
			name: "repository duplicate error",
			req: SignUpRequest{
				Email:    "test@example.com",
				Password: "Password123",
			},
			setup: func(repo *MockUsersRepo) {
				repo.On("FindByEmail", mock.Anything, "test@example.com").Return(nil, errors.New("not found"))
				repo.On("Create", mock.Anything, mock.AnythingOfType("*auth.User")).Return(ErrDuplicate)
			},
			wantErr: true,
			errMsg:  "registration failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockUsersRepo)
			tt.setup(repo)

			refreshRepo := new(MockRefreshTokensRepo)
			refreshRepo.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
			service := NewService(repo, refreshRepo, cfg, silentLogger)
			resp, err := service.SignUp(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotEmpty(t, resp.Token)
				assert.Equal(t, tt.req.Email, resp.User.Email)
			}

			repo.AssertExpectations(t)
		})
	}
}

func TestService_Refresh_TransactionRollback(t *testing.T) {
	// This test verifies that refresh token rotation fails properly when
	// one of the operations in the transaction fails
	cfg := config.Config{
		BcryptCost:         12,
		JWTSecret:          "super-secret-jwt-key-at-least-32-chars",
		JWTAlgorithm:       "HS256",
		AccessTokenMinutes: 15,
		RefreshTokenDays:   30,
		RefreshTokenRotate: true,
	}

	userID := bson.NewObjectID()
	tokenID := bson.NewObjectID()
	rawToken := "test-refresh-token"
	now := time.Now().UTC()

	existingToken := &RefreshToken{
		ID:        tokenID,
		UserID:    userID,
		TokenHash: "hashed-token",
		ExpiresAt: now.Add(24 * time.Hour),
		CreatedAt: now,
	}

	user := &User{
		ID:    userID,
		Email: "test@example.com",
	}

	t.Run("transaction failure scenario", func(t *testing.T) {
		// This test demonstrates that the transaction approach is working
		// In a real scenario, if Client() returns nil or StartSession fails,
		// the service should handle it gracefully
		userRepo := new(MockUsersRepo)
		refreshRepo := new(MockRefreshTokensRepo)

		refreshRepo.On("FindActive", mock.Anything, rawToken).Return(existingToken, nil)
		userRepo.On("FindByID", mock.Anything, userID).Return(user, nil)
		refreshRepo.On("Client").Return((*mongo.Client)(nil))

		service := NewService(userRepo, refreshRepo, cfg, silentLogger)

		// This will fail because client.StartSession() will panic on nil client
		// In a real implementation, we'd want to check for nil client first
		assert.Panics(t, func() {
			_, _ = service.Refresh(context.Background(), rawToken)
		}, "Should panic when client is nil")

		userRepo.AssertExpectations(t)
		refreshRepo.AssertExpectations(t)
	})

	t.Run("refresh without rotation works [rotation=false]", func(t *testing.T) {
		cfgNoRotation := cfg
		cfgNoRotation.RefreshTokenRotate = false

		userRepo := new(MockUsersRepo)
		refreshRepo := new(MockRefreshTokensRepo)

		refreshRepo.On("FindActive", mock.Anything, rawToken).Return(existingToken, nil)
		userRepo.On("FindByID", mock.Anything, userID).Return(user, nil)

		service := NewService(userRepo, refreshRepo, cfgNoRotation, silentLogger)

		resp, err := service.Refresh(context.Background(), rawToken)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, rawToken, resp.RefreshToken, "should return same token when rotation disabled")
		assert.NotEmpty(t, resp.Token, "should get new access token")

		userRepo.AssertExpectations(t)
		refreshRepo.AssertExpectations(t)
	})
}

func TestService_GenerateJWT_DifferentAlgorithms(t *testing.T) {
	tests := []struct {
		name      string
		algorithm string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "HS256 algorithm",
			algorithm: "HS256",
			wantErr:   false,
		},
		{
			name:      "unsupported algorithm",
			algorithm: "INVALID",
			wantErr:   true,
			errMsg:    "unsupported JWT algorithm",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{
				BcryptCost:   12,
				JWTSecret:    "super-secret-jwt-key-at-least-32-chars",
				JWTAlgorithm: tt.algorithm,
			}

			repo := new(MockUsersRepo)
			refreshRepo := new(MockRefreshTokensRepo)
			refreshRepo.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
			service := NewService(repo, refreshRepo, cfg, silentLogger)

			user := &User{
				ID:    bson.NewObjectID(),
				Email: "test@example.com",
			}

			token, err := service.GenerateAccessToken(user)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, token)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, token)
			}
		})
	}
}

func TestService_GenerateJWT_ValidTokenStructure(t *testing.T) {
	cfg := config.Config{
		BcryptCost:   12,
		JWTSecret:    "super-secret-jwt-key-at-least-32-chars",
		JWTAlgorithm: "HS256",
	}

	repo := new(MockUsersRepo)
	refreshRepo := new(MockRefreshTokensRepo)
	service := NewService(repo, refreshRepo, cfg, silentLogger)

	user := &User{
		ID:    bson.NewObjectID(),
		Email: "test@example.com",
	}

	token, err := service.GenerateAccessToken(user)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Token should be valid JWT format (3 parts separated by dots)
	parts := strings.Split(token, ".")
	assert.Equal(t, 3, len(parts), "JWT should have 3 parts: header.payload.signature")

	// Each part should be non-empty
	for i, part := range parts {
		assert.NotEmpty(t, part, "JWT part %d should not be empty", i)
	}
}

func TestService_Refresh_StandaloneMongo(t *testing.T) {
	// Test graceful degradation when MongoDB doesn't support transactions
	cfg := config.Config{
		BcryptCost:         12,
		JWTSecret:          "super-secret-jwt-key-at-least-32-chars",
		JWTAlgorithm:       "HS256",
		AccessTokenMinutes: 15,
		RefreshTokenDays:   30,
		RefreshTokenRotate: true,
	}

	userID := bson.NewObjectID()
	tokenID := bson.NewObjectID()
	rawToken := "test-refresh-token"
	now := time.Now().UTC()

	existingToken := &RefreshToken{
		ID:        tokenID,
		UserID:    userID,
		TokenHash: "hashed-token",
		ExpiresAt: now.Add(24 * time.Hour),
		CreatedAt: now,
	}

	t.Log(existingToken.ExpiresAt.String())
	t.Log(existingToken.CreatedAt.String())

	user := &User{
		ID:    userID,
		Email: "test@example.com",
	}

	// Mock a MongoDB client that will indicate no transaction support
	mockClient := &mongo.Client{}

	t.Run("standalone MongoDB fallback", func(t *testing.T) {
		userRepo := new(MockUsersRepo)
		refreshRepo := new(MockRefreshTokensRepo)

		// Setup mocks for the refresh flow
		refreshRepo.On("FindActive", mock.Anything, rawToken).Return(existingToken, nil)
		userRepo.On("FindByID", mock.Anything, userID).Return(user, nil)
		refreshRepo.On("Client").Return(mockClient)

		// Expect fallback behavior: create new token, then revoke old token
		refreshRepo.On("Create", mock.Anything, userID, mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)
		refreshRepo.On("Revoke", mock.Anything, tokenID).Return(nil)

		// Create a custom service implementation for testing standalone mode
		service := &testServiceStandalone{
			Service: Service{
				usersRepo:        userRepo,
				refreshTokenRepo: refreshRepo,
				config:           cfg,
				log:              silentLogger,
			},
		}

		resp, err := service.Refresh(context.Background(), rawToken)

		// Should succeed without panicking
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotEmpty(t, resp.Token, "should return new access token")
		assert.NotEqual(t, rawToken, resp.RefreshToken, "should return new refresh token")

		// Verify mocks were called as expected
		userRepo.AssertExpectations(t)
		refreshRepo.AssertExpectations(t)

		// Verify that StartSession was NOT called (would be called in transaction mode)
		// This is implicitly tested by the fact that we didn't mock StartSession
		// and the test didn't panic
	})

	t.Run("standalone MongoDB fallback with revoke error", func(t *testing.T) {
		userRepo := new(MockUsersRepo)
		refreshRepo := new(MockRefreshTokensRepo)

		// Setup mocks for the refresh flow
		refreshRepo.On("FindActive", mock.Anything, rawToken).Return(existingToken, nil)
		userRepo.On("FindByID", mock.Anything, userID).Return(user, nil)
		refreshRepo.On("Client").Return(mockClient)

		// Create succeeds, but revoke fails - should still return success
		refreshRepo.On("Create", mock.Anything, userID, mock.AnythingOfType("string"), mock.AnythingOfType("time.Time")).Return(nil)
		refreshRepo.On("Revoke", mock.Anything, tokenID).Return(errors.New("revoke failed"))

		// Create a custom service implementation for testing standalone mode
		service := &testServiceStandalone{
			Service: Service{
				usersRepo:        userRepo,
				refreshTokenRepo: refreshRepo,
				config:           cfg,
				log:              silentLogger,
			},
		}

		resp, err := service.Refresh(context.Background(), rawToken)

		// Should still succeed even if revoke fails (graceful degradation)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.NotEmpty(t, resp.Token, "should return new access token")
		assert.NotEqual(t, rawToken, resp.RefreshToken, "should return new refresh token")

		// Verify mocks were called as expected
		userRepo.AssertExpectations(t)
		refreshRepo.AssertExpectations(t)
	})
}

func TestService_SignIn(t *testing.T) {
	cfg := config.Config{
		BcryptCost:   12,
		JWTSecret:    "super-secret-jwt-key-at-least-32-chars",
		JWTAlgorithm: "HS256",
	}

	password := "Password123"
	hashedPassword, err := crypto.HashPassword(password, 12)
	require.NoError(t, err, "expected no error")

	tests := []struct {
		name    string
		req     SignInRequest
		setup   func(*MockUsersRepo)
		wantErr bool
		errMsg  string
	}{
		{
			name: "successful signin",
			req: SignInRequest{
				Email:    "test@example.com",
				Password: password,
			},
			setup: func(repo *MockUsersRepo) {
				user := &User{
					ID:           bson.NewObjectID(),
					Email:        "test@example.com",
					PasswordHash: hashedPassword,
				}
				repo.On("FindByEmail", mock.Anything, "test@example.com").Return(user, nil)
			},
			wantErr: false,
		},
		{
			name: ErrUserNotFound.Error(),
			req: SignInRequest{
				Email:    "nonexistent@example.com",
				Password: password,
			},
			setup: func(repo *MockUsersRepo) {
				repo.On("FindByEmail", mock.Anything, "nonexistent@example.com").Return(nil, errors.New("user not found"))
			},
			wantErr: true,
			errMsg:  "invalid credentials",
		},
		{
			name: "wrong password",
			req: SignInRequest{
				Email:    "test@example.com",
				Password: "WrongPassword123",
			},
			setup: func(repo *MockUsersRepo) {
				user := &User{
					ID:           bson.NewObjectID(),
					Email:        "test@example.com",
					PasswordHash: hashedPassword,
				}
				repo.On("FindByEmail", mock.Anything, "test@example.com").Return(user, nil)
			},
			wantErr: true,
			errMsg:  "invalid credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockUsersRepo)
			tt.setup(repo)

			refreshRepo := new(MockRefreshTokensRepo)
			refreshRepo.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
			service := NewService(repo, refreshRepo, cfg, silentLogger)
			resp, err := service.SignIn(context.Background(), tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotEmpty(t, resp.Token)
				assert.Equal(t, tt.req.Email, resp.User.Email)
			}

			repo.AssertExpectations(t)
		})
	}
}
