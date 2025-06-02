//go:build !short

package mongo

import (
	"context"
	"os"
	"testing"
	"time"

	"note-pulse/internal/services/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestUsersRepo_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping MongoDB integration test")
	}

	ctx := context.Background()
	_, db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewUsersRepo(db)

	user := &auth.User{
		ID:           bson.NewObjectID(),
		Email:        "test@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err := repo.Create(ctx, user)
	require.NoError(t, err)

	err = repo.Create(ctx, user)
	assert.Equal(t, auth.ErrDuplicate, err, "expected duplicate error")

	found, err := repo.FindByEmail(ctx, user.Email)
	require.NoError(t, err, "expected no error")
	assert.Equal(t, user.Email, found.Email, "expected email to be the same")
	assert.Equal(t, user.PasswordHash, found.PasswordHash, "expected password hash to be the same")
}

func TestUsersRepo_FindByEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping MongoDB integration test")
	}

	ctx := context.Background()
	_, db, cleanup := setupTestDB(t)
	defer cleanup()

	repo := NewUsersRepo(db)

	_, err := repo.FindByEmail(ctx, "nonexistent@example.com")
	assert.Error(t, err, "expected error")
	assert.Contains(t, err.Error(), "user not found", "expected error message")

	user := &auth.User{
		ID:           bson.NewObjectID(),
		Email:        "test@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err = repo.Create(ctx, user)
	require.NoError(t, err, "expected no error")

	found, err := repo.FindByEmail(ctx, user.Email)
	require.NoError(t, err, "expected no error")
	assert.Equal(t, user.Email, found.Email, "expected email to be the same")
	assert.Equal(t, user.PasswordHash, found.PasswordHash, "expected password hash to be the same")
}

func setupTestDB(t *testing.T) (*mongo.Client, *mongo.Database, func()) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Allow override, useful on CI
	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		uri = "mongodb://root:example@localhost:27017/?authSource=admin"
	}

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Skip("MongoDB not available for testing:", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		t.Skip("MongoDB ping failed:", err)
	}

	dbName := "test_notepulse_" + bson.NewObjectID().Hex()
	db := client.Database(dbName)

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		_ = db.Drop(ctx)
		_ = client.Disconnect(ctx)
	}

	return client, db, cleanup
}
