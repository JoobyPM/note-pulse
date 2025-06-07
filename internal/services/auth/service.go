package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"
	"strings"
	"time"

	"note-pulse/internal/config"
	"note-pulse/internal/utils/crypto"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Service handles authentication business logic
type Service struct {
	usersRepo        UsersRepo
	refreshTokenRepo RefreshTokensRepo
	config           config.Config
	log              *slog.Logger
}

// ErrInvalidRefreshToken is returned whenever the caller supplies a refresh
// token that is expired, revoked or does not belong to the user.
var ErrInvalidRefreshToken = errors.New("invalid refresh token")

// ErrUserNotFound user not found in DB
var ErrUserNotFound = errors.New("user not found")

// NewService creates a new auth service
func NewService(usersRepo UsersRepo, refreshTokenRepo RefreshTokensRepo, cfg config.Config, log *slog.Logger) *Service {
	return &Service{
		usersRepo:        usersRepo,
		refreshTokenRepo: refreshTokenRepo,
		config:           cfg,
		log:              log,
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
	User         *User  `json:"user"`
	Token        string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MTcyMzkyMjIsImlhdCI6MTcxNzIzOTIyMiwidXNlcl9pZCI6IjEyMyIsImVtYWlsIjoic3RyaW5nQGV4YW1wbGUuY29tIn0.1234567890"`
	RefreshToken string `json:"refresh_token" example:"refresh_token_example_abcd1234"`
}

// SignUpResponse is an alias for AuthResponse
type SignUpResponse = AuthResponse

// SignInResponse is an alias for AuthResponse
type SignInResponse = AuthResponse

// SignUp registers a new user
func (s *Service) SignUp(ctx context.Context, req SignUpRequest) (*AuthResponse, error) {
	email := normalizeEmail(req.Email)

	existing, err := s.usersRepo.FindByEmail(ctx, email)
	if err == nil && existing != nil {
		s.log.Warn("user already exists", "email", email)
		return nil, maskDuplicateError()
	}

	hashedPassword, err := crypto.HashPassword(req.Password, s.config.BcryptCost)
	if err != nil {
		return nil, errors.New("failed to process password")
	}

	now := time.Now().UTC()
	user := &User{
		ID:           bson.NewObjectID(),
		Email:        email,
		PasswordHash: hashedPassword,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.usersRepo.Create(ctx, user); err != nil {
		if errors.Is(err, ErrDuplicate) {
			s.log.Warn("user already exists", "email", email)
			return nil, maskDuplicateError()
		}
		return nil, errors.New("failed to create user")
	}

	accessToken, err := s.GenerateAccessToken(user)
	if err != nil {
		return nil, ErrGenAccessToken
	}

	refreshToken, err := s.GenerateRefreshToken(user)
	if err != nil {
		return nil, errors.New("failed to generate refresh token")
	}

	refreshExpiresAt := now.Add(time.Duration(s.config.RefreshTokenDays) * 24 * time.Hour)
	if err := s.refreshTokenRepo.Create(ctx, user.ID, refreshToken, refreshExpiresAt); err != nil {
		s.log.Error("failed to store refresh token", "error", err, "user_id", user.ID.Hex())
		return nil, errors.New("failed to generate refresh token")
	}

	return &AuthResponse{
		User:         user,
		Token:        accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// SignIn authenticates a user
func (s *Service) SignIn(ctx context.Context, req SignInRequest) (*AuthResponse, error) {
	email := normalizeEmail(req.Email)

	user, err := s.usersRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			s.log.Info("user not found for signin", "email", email)
		} else {
			s.log.Error("failed to find user by email", "error", err)
		}
		return nil, errors.New("invalid credentials")
	}

	if err := crypto.CheckPassword(req.Password, user.PasswordHash); err != nil {
		s.log.Error("failed to check password", "error", err)
		return nil, errors.New("invalid credentials")
	}

	accessToken, err := s.GenerateAccessToken(user)
	if err != nil {
		s.log.Error(ErrGenAccessToken.Error(), "error", err)
		return nil, ErrGenAccessToken
	}

	refreshToken, err := s.GenerateRefreshToken(user)
	if err != nil {
		s.log.Error("failed to generate refresh token", "error", err)
		return nil, errors.New("failed to generate refresh token")
	}

	refreshExpiresAt := time.Now().UTC().Add(time.Duration(s.config.RefreshTokenDays) * 24 * time.Hour)
	if err := s.refreshTokenRepo.Create(ctx, user.ID, refreshToken, refreshExpiresAt); err != nil {
		s.log.Error("failed to store refresh token", "error", err, "user_id", user.ID.Hex())
		return nil, errors.New("failed to generate refresh token")
	}

	return &AuthResponse{
		User:         user,
		Token:        accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// GenerateAccessToken generates a short-lived access token
func (s *Service) GenerateAccessToken(user *User) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", errors.New("failed to generate token id")
	}
	jti := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b[:])

	now := time.Now().UTC()

	claims := jwt.MapClaims{
		"jti":     jti,
		"user_id": user.ID.Hex(),
		"email":   user.Email,
		"exp":     now.Add(time.Duration(s.config.AccessTokenMinutes) * time.Minute).Unix(),
		"iat":     now.Unix(),
	}

	var method jwt.SigningMethod
	switch strings.ToUpper(s.config.JWTAlgorithm) {
	case "HS256":
		method = jwt.SigningMethodHS256
	default:
		return "", errors.New("unsupported JWT algorithm")
	}

	return jwt.NewWithClaims(method, claims).SignedString([]byte(s.config.JWTSecret))
}

// GenerateRefreshToken generates a cryptographically secure refresh token
func (s *Service) GenerateRefreshToken(user *User) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		s.log.Error("failed to generate random bytes for refresh token", "error", err, "user_id", user.ID.Hex())
		return "", errors.New("failed to generate refresh token")
	}

	return base64.URLEncoding.EncodeToString(bytes), nil
}

// RefreshRequest represents a token refresh request
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required" example:"refresh_token_example_abcd1234"`
}

// SignOutRequest represents a sign-out request
type SignOutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required" example:"refresh_token_example_abcd1234"`
}

// Refresh validates a refresh token and returns new access and refresh tokens
func (s *Service) Refresh(ctx context.Context, rawRefreshToken string) (*AuthResponse, error) {
	refreshToken, err := s.refreshTokenRepo.FindActive(ctx, rawRefreshToken)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			s.log.Info("refresh token not found or expired")
			return nil, ErrInvalidRefreshToken
		}
		s.log.Error("failed to find refresh token", "error", err)
		return nil, ErrInvalidRefreshToken
	}

	user, err := s.usersRepo.FindByID(ctx, refreshToken.UserID)
	if err != nil {
		s.log.Error("failed to find user for refresh token", "error", err, "user_id", refreshToken.UserID.Hex())
		return nil, ErrInvalidRefreshToken
	}

	accessToken, err := s.GenerateAccessToken(user)
	if err != nil {
		s.log.Error("failed to generate new access token", "error", err)
		return nil, errors.New("failed to refresh tokens")
	}

	newRefreshToken := rawRefreshToken // Default: don't rotate

	if s.config.RefreshTokenRotate {
		newRefreshToken, err = s.GenerateRefreshToken(user)
		if err != nil {
			s.log.Error("failed to generate new refresh token", "error", err)
			return nil, errors.New("failed to refresh tokens")
		}
		newRefreshExpiresAt := time.Now().UTC().Add(time.Duration(s.config.RefreshTokenDays) * 24 * time.Hour)

		// Check if MongoDB supports transactions
		client := s.refreshTokenRepo.Client()
		supportsTransactions := s.supportsTransactions(ctx, client)

		if !supportsTransactions {
			// Standalone mode - best-effort two-step without transaction
			s.log.Info("using fallback token rotation for standalone MongoDB")

			// Create new token first
			if err := s.refreshTokenRepo.Create(ctx, user.ID, newRefreshToken, newRefreshExpiresAt); err != nil {
				s.log.Error("failed to store new refresh token in fallback mode", "error", err)
				return nil, errors.New("failed to refresh tokens")
			}

			// Then revoke old token - log warning on error, don't fail request
			if err := s.refreshTokenRepo.Revoke(ctx, refreshToken.ID); err != nil {
				s.log.Warn("failed to revoke old refresh token in fallback mode, continuing", "error", err, "token_id", refreshToken.ID.Hex())
			}

			s.log.Debug("fallback refresh token rotation completed", "user_id", user.ID.Hex(), "old_token_id", refreshToken.ID.Hex())
		} else {
			// Use MongoDB transaction to ensure atomicity (create new -> revoke old)
			sess, err := client.StartSession()
			if err != nil {
				s.log.Error("failed to start MongoDB session", "error", err)
				return nil, errors.New("failed to refresh tokens")
			}
			defer sess.EndSession(ctx)

			_, err = sess.WithTransaction(ctx, func(sc context.Context) (any, error) {
				if err := s.refreshTokenRepo.Create(sc, user.ID, newRefreshToken, newRefreshExpiresAt); err != nil {
					s.log.Error("failed to store new refresh token in transaction", "error", err)
					return nil, err
				}

				// Second: Revoke the old refresh token (race-safe)
				if err := s.refreshTokenRepo.Revoke(sc, refreshToken.ID); err != nil {
					s.log.Error("failed to revoke old refresh token in transaction", "error", err, "token_id", refreshToken.ID.Hex())
					return nil, err
				}

				s.log.Debug("refresh token rotation completed successfully", "user_id", user.ID.Hex(), "old_token_id", refreshToken.ID.Hex())
				return nil, nil // commit transaction
			})

			if err != nil {
				s.log.Error("refresh token rotation transaction failed", "error", err)
				return nil, errors.New("failed to refresh tokens")
			}
		}
	}

	return &AuthResponse{
		User:         user,
		Token:        accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

// SignOut revokes a specific refresh token
func (s *Service) SignOut(ctx context.Context, userID bson.ObjectID, rawRefreshToken string) error {
	refreshToken, err := s.refreshTokenRepo.FindActive(ctx, rawRefreshToken)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			// Already revoked / never existed
			return ErrInvalidRefreshToken
		}
		s.log.Error("failed to find refresh token for sign out", "error", err)
		return errors.New("failed to sign out")
	}

	if refreshToken.UserID != userID {
		s.log.Warn("refresh token does not belong to user", "token_user_id", refreshToken.UserID.Hex(), "request_user_id", userID.Hex())
		return ErrInvalidRefreshToken
	}

	if err := s.refreshTokenRepo.Revoke(ctx, refreshToken.ID); err != nil {
		s.log.Error("failed to revoke refresh token", "error", err, "token_id", refreshToken.ID.Hex())
		return errors.New("failed to sign out")
	}

	s.log.Info("user signed out successfully", "user_id", userID.Hex())
	return nil
}

// SignOutAll revokes all active refresh tokens for a user
func (s *Service) SignOutAll(ctx context.Context, userID bson.ObjectID) error {
	if err := s.refreshTokenRepo.RevokeAllForUser(ctx, userID); err != nil {
		s.log.Error("failed to revoke all refresh tokens for user", "error", err, "user_id", userID.Hex())
		return errors.New("failed to sign out from all devices")
	}

	s.log.Info("user signed out from all devices", "user_id", userID.Hex())
	return nil
}

// TODO:[perf] this condidate for optimizatin, idea - «Expose a cheap accessor from the mongo client package, e.g. `mongo.SupportsTransactions() bool`, and cache the value there (it is already stored) and just read the cached flag»
func (s *Service) supportsTransactions(ctx context.Context, client *mongo.Client) bool {
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var hello bson.M
	err := client.Database("admin").RunCommand(probeCtx, bson.D{{Key: "hello", Value: 1}}).Decode(&hello)
	if err != nil {
		s.log.Warn("failed to probe transaction support, assuming standalone", "err", err)
		return false
	}

	// stand-alone MongoDB has no replica set name
	supportsTransactions := hello["setName"] != nil
	s.log.Debug("checked MongoDB transaction support", "supports_transactions", supportsTransactions)
	return supportsTransactions
}

func maskDuplicateError() error {
	return errors.New("registration failed")
}
