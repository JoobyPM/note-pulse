package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/crypto/bcrypt"

	"note-pulse/internal/logger"
	"note-pulse/internal/services/auth"
)

// RefreshTokensRepo manages refresh token operations in MongoDB
type RefreshTokensRepo struct {
	collection *mongo.Collection
}

// NewRefreshTokensRepo creates a new RefreshTokensRepo instance
func NewRefreshTokensRepo(db *mongo.Database) *RefreshTokensRepo {
	return &RefreshTokensRepo{
		collection: db.Collection("refresh_tokens"),
	}
}

// Create creates a new refresh token record
func (r *RefreshTokensRepo) Create(ctx context.Context, userID bson.ObjectID, rawToken string, expiresAt time.Time) error {
	tokenHash, err := bcrypt.GenerateFromPassword([]byte(rawToken), bcrypt.DefaultCost)
	if err != nil {
		logger.L().Error("failed to hash refresh token", "error", err, "user_id", userID.Hex())
		return err
	}

	refreshToken := auth.RefreshToken{
		UserID:    userID,
		TokenHash: string(tokenHash),
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	}

	_, err = r.collection.InsertOne(ctx, refreshToken)
	if err != nil {
		logger.L().Error("failed to create refresh token", "error", err, "user_id", userID.Hex())
		return err
	}

	logger.L().Debug("refresh token created successfully", "user_id", userID.Hex(), "expires_at", expiresAt)

	return nil
}

// FindActive finds an active (non-revoked, non-expired) refresh token by raw token
func (r *RefreshTokensRepo) FindActive(ctx context.Context, rawToken string) (*auth.RefreshToken, error) {
	filter := bson.M{
		"revoked_at": bson.M{"$exists": false},
		"expires_at": bson.M{"$gt": time.Now().UTC()},
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		logger.L().Error("failed to query refresh tokens", "error", err)
		return nil, err
	}
	defer cursor.Close(ctx)

	// Check each token hash against the provided raw token
	for cursor.Next(ctx) {
		var token auth.RefreshToken
		if err := cursor.Decode(&token); err != nil {
			logger.L().Error("failed to decode refresh token", "error", err)
			continue
		}

		if err := bcrypt.CompareHashAndPassword([]byte(token.TokenHash), []byte(rawToken)); err == nil {
			logger.L().Debug("active refresh token found", "token_id", token.ID.Hex(), "user_id", token.UserID.Hex())
			return &token, nil
		}
	}

	if err := cursor.Err(); err != nil {
		logger.L().Error("cursor error while finding refresh token", "error", err)
		return nil, err
	}

	logger.L().Debug("no active refresh token found")
	return nil, mongo.ErrNoDocuments
}

// Revoke revokes a specific refresh token by setting revoked_at
func (r *RefreshTokensRepo) Revoke(ctx context.Context, id bson.ObjectID) error {
	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"revoked_at": time.Now().UTC(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		logger.L().Error("failed to revoke refresh token", "error", err, "token_id", id.Hex())
		return err
	}

	if result.MatchedCount == 0 {
		logger.L().Warn("refresh token not found for revocation", "token_id", id.Hex())
		return mongo.ErrNoDocuments
	}

	logger.L().Debug("refresh token revoked successfully", "token_id", id.Hex())

	return nil
}

// RevokeAllForUser revokes all active refresh tokens for a specific user
func (r *RefreshTokensRepo) RevokeAllForUser(ctx context.Context, userID bson.ObjectID) error {
	filter := bson.M{
		"user_id":    userID,
		"revoked_at": bson.M{"$exists": false},
	}
	update := bson.M{
		"$set": bson.M{
			"revoked_at": time.Now().UTC(),
		},
	}

	result, err := r.collection.UpdateMany(ctx, filter, update)
	if err != nil {
		logger.L().Error("failed to revoke all refresh tokens for user", "error", err, "user_id", userID.Hex())
		return err
	}

	logger.L().Debug("revoked all refresh tokens for user", "user_id", userID.Hex(), "revoked_count", result.ModifiedCount)

	return nil
}
