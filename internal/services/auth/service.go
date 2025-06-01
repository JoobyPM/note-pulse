package auth

import (
	"context"
	"errors"
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
}

// NewService creates a new auth service
func NewService(repo UsersRepo, cfg config.Config) *Service {
	return &Service{
		repo:   repo,
		config: cfg,
	}
}

// SignUpRequest represents a user registration request
type SignUpRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// SignInRequest represents a user login request
type SignInRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// AuthResponse represents the response for successful authentication
type AuthResponse struct {
	User  *User  `json:"user"`
	Token string `json:"token"`
}

// SignUp registers a new user
func (s *Service) SignUp(ctx context.Context, req SignUpRequest) (*AuthResponse, error) {
	email := normalizeEmail(req.Email)

	if !crypto.IsStrong(req.Password) {
		return nil, errors.New("password must be at least 8 characters and contain uppercase, lowercase, and digit")
	}

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
		return nil, errors.New("invalid credentials")
	}

	if err := crypto.CheckPassword(req.Password, user.PasswordHash); err != nil {
		return nil, errors.New("invalid credentials")
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

func (s *Service) generateJWT(user *User) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.ID.Hex(),
		"email": user.Email,
		"exp":   time.Now().Add(time.Duration(s.config.JWTExpiryMinutes) * time.Minute).Unix(),
		"iat":   time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func maskDuplicateError() error {
	return errors.New("registration failed")
}
