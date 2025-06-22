package auth

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// ErrDuplicate is returned when trying to create a user with an email that already exists
var ErrDuplicate = errors.New("user with this email already exists")

// UsersRepo defines the interface for user repository operations
type UsersRepo interface {
	Create(ctx context.Context, user *User) error
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByID(ctx context.Context, id bson.ObjectID) (*User, error)
}

// RefreshTokensRepo defines the interface for refresh token data access operations
type RefreshTokensRepo interface {
	// Create creates a new refresh token record
	Create(ctx context.Context, userID bson.ObjectID, rawToken string, expiresAt time.Time) error

	// FindActive finds an active (non-revoked, non-expired) refresh token by raw token
	FindActive(ctx context.Context, rawToken string) (*RefreshToken, error)

	// Revoke revokes a specific refresh token by setting revoked_at
	Revoke(ctx context.Context, id bson.ObjectID) error

	// RevokeAllForUser revokes all active refresh tokens for a specific user
	RevokeAllForUser(ctx context.Context, userID bson.ObjectID) error

	// Client returns the MongoDB client for transaction support
	Client() *mongo.Client

	// SupportsTransactions returns whether the MongoDB instance supports transactions
	SupportsTransactions() bool
}

// RefreshToken represents a refresh token document
type RefreshToken struct {
	ID         bson.ObjectID `bson:"_id,omitempty"`
	UserID     bson.ObjectID `bson:"user_id"`
	TokenHash  string        `bson:"token_hash"`
	LookupHash string        `bson:"lookup_hash"`
	ExpiresAt  time.Time     `bson:"expires_at"`
	CreatedAt  time.Time     `bson:"created_at"`
	RevokedAt  *time.Time    `bson:"revoked_at,omitempty"`
}
