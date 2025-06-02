package auth

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"note-pulse/internal/config"
	"note-pulse/internal/utils/crypto"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Service handles authentication business logic
type Service struct {
	repo   UsersRepo
	config config.Config
	log    *slog.Logger
}

// NewService creates a new auth service
func NewService(repo UsersRepo, cfg config.Config, log *slog.Logger) *Service {
	return &Service{
		repo:   repo,
		config: cfg,
		log:    log,
	}
}

// SignUpRequest represents a user registration request
type SignUpRequest struct {
	Email    string `json:"email" validate:"required,email" example:"test@example.com"`
	Password string `json:"password" validate:"required,password" example:"Password123"`
}

// SignInRequest represents a user login request
type SignInRequest struct {
	Email    string `json:"email" validate:"required,email" example:"test@example.com"`
	Password string `json:"password" validate:"required" example:"Password123"`
}

// AuthResponse represents the response for successful authentication
type AuthResponse struct {
	User  *User  `json:"user"`
	Token string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MTcyMzkyMjIsImlhdCI6MTcxNzIzOTIyMiwidXNlcl9pZCI6IjEyMyIsImVtYWlsIjoic3RyaW5nQGV4YW1wbGUuY29tIn0.1234567890"`
}

// SignUpResponse is an alias for AuthResponse
type SignUpResponse = AuthResponse

// SignInResponse is an alias for AuthResponse
type SignInResponse = AuthResponse

// SignUp registers a new user
func (s *Service) SignUp(ctx context.Context, req SignUpRequest) (*AuthResponse, error) {
	email := normalizeEmail(req.Email)

	// Password validation is now handled by the 'password' validator tag

	existing, err := s.repo.FindByEmail(ctx, email)
	if err == nil && existing != nil {
		return nil, maskDuplicateError()
	}

	hashedPassword, err := crypto.HashPassword(req.Password, s.config.BcryptCost)
	if err != nil {
		return nil, errors.New("failed to process password")
	}

	now := time.Now()
	user := &User{
		ID:           bson.NewObjectID(),
		Email:        email,
		PasswordHash: hashedPassword,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.repo.Create(ctx, user); err != nil {
		if errors.Is(err, ErrDuplicate) {
			return nil, maskDuplicateError()
		}
		return nil, errors.New("failed to create user")
	}

	token, err := s.generateJWT(user)
	if err != nil {
		return nil, errors.New("failed to generate token")
	}

	return &AuthResponse{
		User:  user,
		Token: token,
	}, nil
}

// SignIn authenticates a user
func (s *Service) SignIn(ctx context.Context, req SignInRequest) (*AuthResponse, error) {
	email := normalizeEmail(req.Email)

	user, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		s.log.Error("failed to find user by email", "error", err)
		return nil, errors.New("invalid credentials")
	}

	if err := crypto.CheckPassword(req.Password, user.PasswordHash); err != nil {
		s.log.Error("failed to check password", "error", err)
		return nil, errors.New("invalid credentials")
	}

	token, err := s.generateJWT(user)
	if err != nil {
		s.log.Error("failed to generate token", "error", err)
		return nil, errors.New("failed to generate token")
	}

	return &AuthResponse{
		User:  user,
		Token: token,
	}, nil
}

func (s *Service) generateJWT(user *User) (string, error) {
	claims := jwt.MapClaims{
		"user_id": user.ID.Hex(),
		"email":   user.Email,
		"exp":     time.Now().Add(time.Duration(s.config.JWTExpiryMinutes) * time.Minute).Unix(),
		"iat":     time.Now().Unix(),
	}

	alg := strings.ToUpper(s.config.JWTAlgorithm)
	var method jwt.SigningMethod
	switch alg {
	case "HS256":
		method = jwt.SigningMethodHS256
	case "RS256":
		method = jwt.SigningMethodRS256
	default:
		return "", errors.New("unsupported JWT algorithm")
	}

	token := jwt.NewWithClaims(method, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func maskDuplicateError() error {
	return errors.New("registration failed")
}
