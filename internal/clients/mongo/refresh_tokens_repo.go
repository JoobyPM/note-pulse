package mongo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"golang.org/x/crypto/bcrypt"

	"note-pulse/internal/logger"
	"note-pulse/internal/services/auth"
)

// safeLog safely logs using the logger if it's available, otherwise uses default logger
func safeLog() *slog.Logger {
	if l := logger.L(); l != nil {
		return l
	}
	return slog.Default()
}

// refreshTokenOpTimeout is the timeout for refresh token index operations (longer than regular ops)
const refreshTokenOpTimeout = 10 * time.Second

// RefreshTokensRepo manages refresh token operations in MongoDB
type RefreshTokensRepo struct {
	collection *mongo.Collection
	bcryptCost int
}

// NewRefreshTokensRepo creates a new RefreshTokensRepo instance
func NewRefreshTokensRepo(parentCtx context.Context, db *mongo.Database, bcryptCost int) *RefreshTokensRepo {
	collection := db.Collection("refresh_tokens")

	indexes := []mongo.IndexModel{
		// TTL index on expires_at (simpler approach)
		{
			Keys:    bson.D{{Key: "expires_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
		// TTL index on revoked_at - removes revoked tokens after 1 hour grace period
		// This gives ongoing requests time to finish while keeping working set small
		{
			Keys:    bson.D{{Key: "revoked_at", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(3600),
		},
		// Fast lookup index - unique on user_id + lookup_hash (partial for compatibility)
		{
			Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "lookup_hash", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetPartialFilterExpression(bson.M{"lookup_hash": bson.M{"$type": "string"}}),
		},
		// Keep old index for backwards compatibility during migration
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}, {Key: "token_hash", Value: 1}},
			Options: options.Index().SetUnique(false), // Remove uniqueness to avoid conflicts
		},
	}

	ctx, cancel := context.WithTimeout(parentCtx, refreshTokenOpTimeout)
	defer cancel()

	if _, err := collection.Indexes().CreateMany(ctx, indexes); err != nil {
		// We don't panic as indexes might already exist
		safeLog().Error("failed to create refresh_tokens indexes", "error", err)
	}

	return &RefreshTokensRepo{
		collection: collection,
		bcryptCost: bcryptCost,
	}
}

// Create creates a new refresh token record
func (r *RefreshTokensRepo) Create(ctx context.Context, userID bson.ObjectID, rawToken string, expiresAt time.Time) error {
	tokenHash, err := bcrypt.GenerateFromPassword([]byte(rawToken), r.bcryptCost)
	if err != nil {
		safeLog().Error("failed to hash refresh token", "error", err, "user_id", userID.Hex())
		return err
	}

	// Generate fast lookup hash
	h := sha256.Sum256([]byte(rawToken))
	lookupHash := hex.EncodeToString(h[:])

	refreshToken := auth.RefreshToken{
		UserID:     userID,
		TokenHash:  string(tokenHash),
		LookupHash: lookupHash,
		ExpiresAt:  expiresAt,
		CreatedAt:  time.Now().UTC(),
	}

	_, err = r.collection.InsertOne(ctx, refreshToken)
	if err != nil {
		// Handle duplicate key error gracefully
		if mongo.IsDuplicateKeyError(err) {
			// There is potential race-condition, but I ignore it, as this project already consumed too much
			// And it's very unlikly.
			safeLog().Debug("duplicate refresh token creation detected, treating as success", "user_id", userID.Hex(), "lookup_hash", lookupHash)
			return nil
		}
		safeLog().Error("failed to create refresh token", "error", err, "user_id", userID.Hex())
		return err
	}

	safeLog().Debug("refresh token created successfully", "user_id", userID.Hex(), "expires_at", expiresAt)

	return nil
}

// Client returns the MongoDB client for transaction support
func (r *RefreshTokensRepo) Client() *mongo.Client {
	return r.collection.Database().Client()
}

// FindActive finds an active (non-revoked, non-expired) refresh token by raw token
func (r *RefreshTokensRepo) FindActive(ctx context.Context, rawToken string) (*auth.RefreshToken, error) {
	// First try: Fast O(1) lookup using lookup_hash (for new tokens)
	h := sha256.Sum256([]byte(rawToken))
	lookupHash := hex.EncodeToString(h[:])

	filter := bson.M{
		"lookup_hash": lookupHash,
		"revoked_at":  bson.M{"$exists": false},
		"expires_at":  bson.M{"$gt": time.Now().UTC()},
	}

	var token auth.RefreshToken
	err := r.collection.FindOne(ctx, filter).Decode(&token)
	if err == nil {
		// Verify the token with bcrypt as a security measure
		if err := bcrypt.CompareHashAndPassword([]byte(token.TokenHash), []byte(rawToken)); err != nil {
			safeLog().Warn("refresh token lookup hash matched but bcrypt verification failed", "token_id", token.ID.Hex())
			return nil, mongo.ErrNoDocuments
		}
		safeLog().Debug("active refresh token found via lookup_hash", "token_id", token.ID.Hex(), "user_id", token.UserID.Hex())
		return &token, nil
	}

	// If fast lookup failed and it's not a "no documents" error, return the error
	if err != mongo.ErrNoDocuments {
		safeLog().Error("failed to query refresh token via lookup_hash", "error", err)
		return nil, err
	}

	// TODO: [perf] this condidate for optimizatin, idea - «Create a background worker that rewrites old documents with the new `lookup_hash` and drop the fallback»
	// Fallback: Use slower O(N) scan for tokens without lookup_hash (backward compatibility)
	safeLog().Debug("falling back to bcrypt scan for tokens without lookup_hash")
	fallbackFilter := bson.M{
		"lookup_hash": bson.M{"$exists": false}, // Only check tokens without lookup_hash
		"revoked_at":  bson.M{"$exists": false},
		"expires_at":  bson.M{"$gt": time.Now().UTC()},
	}

	cursor, err := r.collection.Find(ctx, fallbackFilter)
	if err != nil {
		safeLog().Error("failed to query refresh tokens for fallback", "error", err)
		return nil, err
	}
	defer cursor.Close(ctx)

	// Check each token hash against the provided raw token
	for cursor.Next(ctx) {
		var fallbackToken auth.RefreshToken
		if err := cursor.Decode(&fallbackToken); err != nil {
			safeLog().Error("failed to decode refresh token in fallback", "error", err)
			continue
		}

		if err := bcrypt.CompareHashAndPassword([]byte(fallbackToken.TokenHash), []byte(rawToken)); err == nil {
			safeLog().Debug("active refresh token found via fallback scan", "token_id", fallbackToken.ID.Hex(), "user_id", fallbackToken.UserID.Hex())
			return &fallbackToken, nil
		}
	}

	if err := cursor.Err(); err != nil {
		safeLog().Error("cursor error while finding refresh token in fallback", "error", err)
		return nil, err
	}

	safeLog().Debug("no active refresh token found in fast lookup or fallback scan")
	return nil, mongo.ErrNoDocuments
}

// Revoke revokes a specific refresh token by setting revoked_at
// Only revokes if the token is not already revoked (prevents race conditions)
func (r *RefreshTokensRepo) Revoke(ctx context.Context, id bson.ObjectID) error {
	// Only revoke tokens that are not already revoked (race-safe)
	filter := bson.M{
		"_id":        id,
		"revoked_at": bson.M{"$exists": false},
	}
	update := bson.M{
		"$set": bson.M{
			"revoked_at": time.Now().UTC(),
		},
	}

	result, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		safeLog().Error("failed to revoke refresh token", "error", err, "token_id", id.Hex())
		return err
	}

	if result.MatchedCount == 0 {
		safeLog().Debug("refresh token not found or already revoked", "token_id", id.Hex())
		return mongo.ErrNoDocuments
	}

	safeLog().Debug("refresh token revoked successfully", "token_id", id.Hex())

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
		safeLog().Error("failed to revoke all refresh tokens for user", "error", err, "user_id", userID.Hex())
		return err
	}

	safeLog().Debug("revoked all refresh tokens for user", "user_id", userID.Hex(), "revoked_count", result.ModifiedCount)

	return nil
}
