package auth

import (
	"context"
	"errors"
	"testing"

	"note-pulse/internal/config"
	"note-pulse/internal/utils/crypto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// MockUsersRepo is a mock implementation of UsersRepo
type MockUsersRepo struct {
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

func TestService_SignUp(t *testing.T) {
	cfg := config.Config{
		BcryptCost:       12,
		JWTSecret:        "super-secret-jwt-key-at-least-32-chars",
		JWTAlgorithm:     "HS256",
		JWTExpiryMinutes: 60,
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
			name: "weak password",
			req: SignUpRequest{
				Email:    "test@example.com",
				Password: "weak",
			},
			setup:   func(repo *MockUsersRepo) {},
			wantErr: true,
			errMsg:  "password must be at least 8 characters and contain uppercase, lowercase, and digit",
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

			service := NewService(repo, cfg)
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

func TestService_SignIn(t *testing.T) {
	cfg := config.Config{
		BcryptCost:       12,
		JWTSecret:        "super-secret-jwt-key-at-least-32-chars",
		JWTAlgorithm:     "HS256",
		JWTExpiryMinutes: 60,
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
			name: "user not found",
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

			service := NewService(repo, cfg)
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
