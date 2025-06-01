package mongo

import (
	"context"
	"sync"
	"testing"
	"time"

	"note-pulse/internal/config"
	"note-pulse/internal/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func TestMongoClient_Idempotency(t *testing.T) {
	reset()
	defer reset()

	cfg := config.Config{
		MongoURI:    "mongodb://invalid/?connectTimeoutMS=1&serverSelectionTimeoutMS=1",
		MongoDBName: "test",
		LogLevel:    "error",
		LogFormat:   "json",
	}

	log, err := logger.Init(cfg)
	require.NoError(t, err)

	ctx := context.Background()

	client1, db1, err1 := Init(ctx, cfg, log)
	client2, db2, err2 := Init(ctx, cfg, log)

	assert.Equal(t, client1, client2, "clients should be the same pointer")
	assert.Equal(t, db1, db2, "databases should be the same pointer")
	assert.Error(t, err1)
	assert.Equal(t, err1, err2)
}

func TestMongoClient_ShutdownResets(t *testing.T) {
	reset()
	defer reset()

	cfg := config.Config{
		MongoURI:    "mongodb://invalid/?connectTimeoutMS=1&serverSelectionTimeoutMS=1",
		MongoDBName: "test",
		LogLevel:    "error",
		LogFormat:   "json",
	}

	log, err := logger.Init(cfg)
	require.NoError(t, err)

	ctx := context.Background()

	client1, db1, initErr := Init(ctx, cfg, log)
	require.Error(t, initErr)

	err = Shutdown(ctx)
	assert.NoError(t, err)

	client2, db2, initErr := Init(ctx, cfg, log)
	require.Error(t, initErr)

	assert.NotEqual(t, client1, client2, "clients should be different pointers after shutdown")
	assert.NotEqual(t, db1, db2, "databases should be different pointers after shutdown")
}

func TestMongoClient_Concurrency(t *testing.T) {
	reset()
	defer reset()

	cfg := config.Config{
		MongoURI:    "mongodb://invalid/?connectTimeoutMS=1&serverSelectionTimeoutMS=1",
		MongoDBName: "test",
		LogLevel:    "error",
		LogFormat:   "json",
	}

	log, err := logger.Init(cfg)
	require.NoError(t, err)

	ctx := context.Background()

	const goroutines = 10
	var wg sync.WaitGroup
	clients := make([]*mongo.Client, goroutines)
	dbs := make([]*mongo.Database, goroutines)

	wg.Add(goroutines)

	for i := range goroutines {
		go func(index int) {
			defer wg.Done()
			client, db, err := Init(ctx, cfg, log)
			if err == nil {
				t.Errorf("Init should fail: %v", err)
			}
			clients[index] = client
			dbs[index] = db
		}(i)
	}

	wg.Wait()

	require.NotNil(t, clients[0])
	require.NotNil(t, dbs[0])

	for i := 1; i < goroutines; i++ {
		assert.Equal(t, clients[0], clients[i], "all clients should be the same pointer")
		assert.Equal(t, dbs[0], dbs[i], "all databases should be the same pointer")
	}
}

func TestMongoClient_AccessorsAfterInit(t *testing.T) {
	reset()
	defer reset()

	cfg := config.Config{
		MongoURI:    "mongodb://invalid/?connectTimeoutMS=1&serverSelectionTimeoutMS=1",
		MongoDBName: "test",
		LogLevel:    "error",
		LogFormat:   "json",
	}

	log, err := logger.Init(cfg)
	require.NoError(t, err)

	ctx := context.Background()

	initClient, initDB, initErr := Init(ctx, cfg, log)
	require.Error(t, initErr)

	accessorClient := Client()
	accessorDB := DB()

	assert.Equal(t, initClient, accessorClient, "Client() should return the same instance as Init")
	assert.Equal(t, initDB, accessorDB, "DB() should return the same instance as Init")
}

func TestMongoClient_ShutdownIdempotency(t *testing.T) {
	reset()
	defer reset()

	cfg := config.Config{
		MongoURI:    "mongodb://invalid/?connectTimeoutMS=1&serverSelectionTimeoutMS=1",
		MongoDBName: "test",
		LogLevel:    "error",
		LogFormat:   "json",
	}

	log, err := logger.Init(cfg)
	require.NoError(t, err)

	ctx := context.Background()

	_, _, err = Init(ctx, cfg, log)
	require.Error(t, err)

	err1 := Shutdown(ctx)
	err2 := Shutdown(ctx)
	err3 := Shutdown(ctx)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.NoError(t, err3)

	assert.Nil(t, Client())
	assert.Nil(t, DB())
}

func TestMongoClient_RetryAfterFailure(t *testing.T) {
	reset()
	defer reset()

	cfg := config.Config{
		MongoURI:    "mongodb://invalid/?connectTimeoutMS=1&serverSelectionTimeoutMS=1",
		MongoDBName: "test",
		LogLevel:    "error",
		LogFormat:   "json",
	}

	log, err := logger.Init(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	client1, db1, err1 := Init(ctx, cfg, log)
	assert.Error(t, err1, "first Init should fail with invalid URI")
	assert.NotNil(t, client1, "client should be returned even on ping failure")
	assert.NotNil(t, db1, "db should be returned even on ping failure")

	client2, db2, err2 := Init(ctx, cfg, log)
	assert.Equal(t, client1, client2, "should return same client on retry after ping failure")
	assert.Equal(t, db1, db2, "should return same db on retry after ping failure")
	assert.Equal(t, err1, err2)
}
