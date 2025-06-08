package mongo

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	testRefreshToken = "test-refresh-token-123"
	testExpiresAt    = 30 * 24 * time.Hour
	msgShouldCreate  = "should create token"
	msgShouldFind    = "should find token"
	msgShouldNotFind = "should not find token"
	msgShouldRevoke  = "should revoke token"
)

// setupRefreshTokensRepo is a helper function that sets up a test repository with database and context
func setupRefreshTokensRepo(t *testing.T) (context.Context, *RefreshTokensRepo, *mongo.Database, func()) {
	_, db, cleanup := setupTestDB(t)
	ctx := context.Background()
	repo := NewRefreshTokensRepo(ctx, db)
	return ctx, repo, db, cleanup
}

// createTestRefreshToken is a helper that creates a refresh token in the repo for testing.
// It returns the userID, rawToken, expiresAt, and any error from creation.
func createTestRefreshToken(t *testing.T, ctx context.Context, repo *RefreshTokensRepo) (bson.ObjectID, string, time.Time, error) {
	userID := bson.NewObjectID()
	rawToken := testRefreshToken
	expiresAt := time.Now().UTC().Add(30 * 24 * time.Hour)

	err := repo.Create(ctx, userID, rawToken, expiresAt)
	require.NoError(t, err, msgShouldCreate)

	return userID, rawToken, expiresAt, err
}

func TestRefreshTokensRepoCreate(t *testing.T) {
	ctx, repo, _, cleanup := setupRefreshTokensRepo(t)
	defer cleanup()

	userID, rawToken, expiresAt, err := createTestRefreshToken(t, ctx, repo)
	require.NoError(t, err)

	// Verify token was created
	token, err := repo.FindActive(ctx, rawToken)
	require.NoError(t, err)
	assert.Equal(t, userID, token.UserID)
	assert.WithinDuration(t, expiresAt, token.ExpiresAt, time.Second)
	assert.NotEmpty(t, token.TokenHash)
	assert.NotEqual(t, rawToken, token.TokenHash, "should be hashed")
}

func TestRefreshTokensRepoFindActive(t *testing.T) {
	ctx, repo, _, cleanup := setupRefreshTokensRepo(t)
	defer cleanup()

	userID, rawToken, _, err := createTestRefreshToken(t, ctx, repo)
	require.NoError(t, err, msgShouldCreate)

	token, err := repo.FindActive(ctx, rawToken)
	require.NoError(t, err, msgShouldFind)
	assert.Equal(t, userID, token.UserID, "should have correct user ID")

	_, err = repo.FindActive(ctx, "wrong-token")
	assert.Equal(t, mongo.ErrNoDocuments, err, msgShouldNotFind)
}

func TestRefreshTokensRepoFindActiveExpired(t *testing.T) {
	ctx, repo, _, cleanup := setupRefreshTokensRepo(t)
	defer cleanup()

	userID := bson.NewObjectID()
	rawToken := testRefreshToken
	expiresAt := time.Now().UTC().Add(-1 * time.Hour)

	err := repo.Create(ctx, userID, rawToken, expiresAt)
	require.NoError(t, err, msgShouldCreate)

	_, err = repo.FindActive(ctx, rawToken)
	assert.Equal(t, mongo.ErrNoDocuments, err, msgShouldNotFind)
}

func TestRefreshTokensRepoRevoke(t *testing.T) {
	ctx, repo, _, cleanup := setupRefreshTokensRepo(t)
	defer cleanup()

	userID := bson.NewObjectID()
	rawToken := testRefreshToken
	expiresAt := time.Now().UTC().Add(30 * 24 * time.Hour)

	err := repo.Create(ctx, userID, rawToken, expiresAt)
	require.NoError(t, err, msgShouldCreate)

	token, err := repo.FindActive(ctx, rawToken)
	require.NoError(t, err, msgShouldFind)

	err = repo.Revoke(ctx, token.ID)
	require.NoError(t, err, msgShouldRevoke)

	_, err = repo.FindActive(ctx, rawToken)
	assert.Equal(t, mongo.ErrNoDocuments, err, msgShouldNotFind)

	err = repo.Revoke(ctx, bson.NewObjectID())
	assert.Equal(t, mongo.ErrNoDocuments, err, msgShouldNotFind)
}

func TestRefreshTokensRepoRevokeAllForUser(t *testing.T) {
	ctx, repo, _, cleanup := setupRefreshTokensRepo(t)
	defer cleanup()

	userID := bson.NewObjectID()
	otherUserID := bson.NewObjectID()
	expiresAt := time.Now().UTC().Add(30 * 24 * time.Hour)

	token1 := "token1"
	token2 := "token2"
	otherToken := "other-token"

	err := repo.Create(ctx, userID, token1, expiresAt)
	require.NoError(t, err, msgShouldCreate)

	err = repo.Create(ctx, userID, token2, expiresAt)
	require.NoError(t, err, msgShouldCreate)

	err = repo.Create(ctx, otherUserID, otherToken, expiresAt)
	require.NoError(t, err, msgShouldCreate)

	err = repo.RevokeAllForUser(ctx, userID)
	require.NoError(t, err, msgShouldRevoke)

	_, err = repo.FindActive(ctx, token1)
	assert.Equal(t, mongo.ErrNoDocuments, err, msgShouldNotFind)

	_, err = repo.FindActive(ctx, token2)
	assert.Equal(t, mongo.ErrNoDocuments, err, msgShouldNotFind)

	_, err = repo.FindActive(ctx, otherToken)
	assert.NoError(t, err, msgShouldFind)
}

func TestRefreshTokensRepoFindActiveMultipleTokens(t *testing.T) {
	ctx, repo, _, cleanup := setupRefreshTokensRepo(t)
	defer cleanup()

	userID := bson.NewObjectID()
	expiresAt := time.Now().UTC().Add(30 * 24 * time.Hour)

	token1 := "token1"
	token2 := "token2"

	err := repo.Create(ctx, userID, token1, expiresAt)
	require.NoError(t, err, msgShouldCreate)

	err = repo.Create(ctx, userID, token2, expiresAt)
	require.NoError(t, err, msgShouldCreate)

	foundToken1, err := repo.FindActive(ctx, token1)
	require.NoError(t, err, msgShouldFind)
	assert.Equal(t, userID, foundToken1.UserID)

	foundToken2, err := repo.FindActive(ctx, token2)
	require.NoError(t, err, msgShouldFind)
	assert.Equal(t, userID, foundToken2.UserID)

	assert.NotEqual(t, foundToken1.ID, foundToken2.ID, "should have different IDs")
}

// TestRefreshTokensRepo_Create_Duplicate tests that concurrent creation of the same token
// is handled gracefully via duplicate key error handling
func TestRefreshTokensRepoCreateDuplicate(t *testing.T) {
	_, db, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	repo := NewRefreshTokensRepo(ctx, db)

	userID := bson.NewObjectID()
	rawToken := "same-token-for-both-goroutines"
	expiresAt := time.Now().UTC().Add(30 * 24 * time.Hour)

	errors := make(chan error, 2)
	var wg sync.WaitGroup

	// Launch two goroutines that try to create the same token simultaneously
	wg.Add(2)
	go func() {
		defer wg.Done()
		err := repo.Create(ctx, userID, rawToken, expiresAt)
		errors <- err
	}()

	go func() {
		defer wg.Done()
		err := repo.Create(ctx, userID, rawToken, expiresAt)
		errors <- err
	}()

	wg.Wait()
	close(errors)

	for err := range errors {
		require.NoError(t, err, "both create operations should succeed")
	}

	token, err := repo.FindActive(ctx, rawToken)
	require.NoError(t, err, msgShouldFind)
	assert.Equal(t, userID, token.UserID)

	// Count documents to ensure only one was created
	count, err := db.Collection("refresh_tokens").CountDocuments(ctx, bson.M{"user_id": userID})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "exactly one token should exist")
}
