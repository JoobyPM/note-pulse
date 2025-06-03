package mongo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func TestRefreshTokensRepo_Create(t *testing.T) {
	_, db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRefreshTokensRepo(db)
	ctx := context.Background()

	userID := bson.NewObjectID()
	rawToken := "test-refresh-token-123"
	expiresAt := time.Now().Add(30 * 24 * time.Hour)

	err := repo.Create(ctx, userID, rawToken, expiresAt)
	require.NoError(t, err)

	// Verify token was created
	token, err := repo.FindActive(ctx, rawToken)
	require.NoError(t, err)
	assert.Equal(t, userID, token.UserID)
	assert.WithinDuration(t, expiresAt, token.ExpiresAt, time.Second)
	assert.NotEmpty(t, token.TokenHash)
	assert.NotEqual(t, rawToken, token.TokenHash, "should be hashed")
}

func TestRefreshTokensRepo_FindActive(t *testing.T) {
	_, db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRefreshTokensRepo(db)
	ctx := context.Background()

	userID := bson.NewObjectID()
	rawToken := "test-refresh-token-123"
	expiresAt := time.Now().Add(30 * 24 * time.Hour)

	err := repo.Create(ctx, userID, rawToken, expiresAt)
	require.NoError(t, err, "should create token")

	token, err := repo.FindActive(ctx, rawToken)
	require.NoError(t, err, "should find token")
	assert.Equal(t, userID, token.UserID, "should have correct user ID")

	_, err = repo.FindActive(ctx, "wrong-token")
	assert.Equal(t, mongo.ErrNoDocuments, err, "should not find token")
}

func TestRefreshTokensRepo_FindActive_Expired(t *testing.T) {
	_, db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRefreshTokensRepo(db)
	ctx := context.Background()

	userID := bson.NewObjectID()
	rawToken := "test-refresh-token-123"
	expiresAt := time.Now().Add(-1 * time.Hour)

	err := repo.Create(ctx, userID, rawToken, expiresAt)
	require.NoError(t, err, "should create token")

	_, err = repo.FindActive(ctx, rawToken)
	assert.Equal(t, mongo.ErrNoDocuments, err, "should not find token")
}

func TestRefreshTokensRepo_Revoke(t *testing.T) {
	_, db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRefreshTokensRepo(db)
	ctx := context.Background()

	userID := bson.NewObjectID()
	rawToken := "test-refresh-token-123"
	expiresAt := time.Now().Add(30 * 24 * time.Hour)

	err := repo.Create(ctx, userID, rawToken, expiresAt)
	require.NoError(t, err, "should create token")

	token, err := repo.FindActive(ctx, rawToken)
	require.NoError(t, err, "should find token")

	err = repo.Revoke(ctx, token.ID)
	require.NoError(t, err, "should revoke token")

	_, err = repo.FindActive(ctx, rawToken)
	assert.Equal(t, mongo.ErrNoDocuments, err, "should not find token")

	err = repo.Revoke(ctx, bson.NewObjectID())
	assert.Equal(t, mongo.ErrNoDocuments, err, "should not find token")
}

func TestRefreshTokensRepo_RevokeAllForUser(t *testing.T) {
	_, db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRefreshTokensRepo(db)
	ctx := context.Background()

	userID := bson.NewObjectID()
	otherUserID := bson.NewObjectID()
	expiresAt := time.Now().Add(30 * 24 * time.Hour)

	token1 := "token1"
	token2 := "token2"
	otherToken := "other-token"

	err := repo.Create(ctx, userID, token1, expiresAt)
	require.NoError(t, err, "should create token")

	err = repo.Create(ctx, userID, token2, expiresAt)
	require.NoError(t, err, "should create token")

	err = repo.Create(ctx, otherUserID, otherToken, expiresAt)
	require.NoError(t, err, "should create token")

	err = repo.RevokeAllForUser(ctx, userID)
	require.NoError(t, err, "should revoke all tokens for user")

	_, err = repo.FindActive(ctx, token1)
	assert.Equal(t, mongo.ErrNoDocuments, err, "should not find token")

	_, err = repo.FindActive(ctx, token2)
	assert.Equal(t, mongo.ErrNoDocuments, err, "should not find token")

	_, err = repo.FindActive(ctx, otherToken)
	assert.NoError(t, err, "should find other token")
}

func TestRefreshTokensRepo_FindActive_MultipleTokens(t *testing.T) {
	_, db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewRefreshTokensRepo(db)
	ctx := context.Background()

	userID := bson.NewObjectID()
	expiresAt := time.Now().Add(30 * 24 * time.Hour)

	token1 := "token1"
	token2 := "token2"

	err := repo.Create(ctx, userID, token1, expiresAt)
	require.NoError(t, err)

	err = repo.Create(ctx, userID, token2, expiresAt)
	require.NoError(t, err)

	foundToken1, err := repo.FindActive(ctx, token1)
	require.NoError(t, err, "should find token")
	assert.Equal(t, userID, foundToken1.UserID)

	foundToken2, err := repo.FindActive(ctx, token2)
	require.NoError(t, err, "should find token")
	assert.Equal(t, userID, foundToken2.UserID)

	assert.NotEqual(t, foundToken1.ID, foundToken2.ID, "should have different IDs")
}
