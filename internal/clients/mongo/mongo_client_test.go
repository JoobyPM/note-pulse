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
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// stubDriver implements the driver interface for testing
type stubDriver struct{}

func (stubDriver) Connect(_ context.Context, _ *options.ClientOptions) (*mongo.Client, error) {
	return nil, context.DeadlineExceeded // fail immediately to avoid retry delays
}

func (stubDriver) Ping(_ context.Context, _ *mongo.Client) error {
	return context.DeadlineExceeded
}

func (stubDriver) Disconnect(_ context.Context, _ *mongo.Client) error { return nil }

// withStubDriver temporarily replaces the global driver with a stub for testing
func withStubDriver(t *testing.T) func() {
	t.Helper()
	old := drv
	drv = stubDriver{}
	return func() { drv = old }
}

func TestMongoClient_Idempotency(t *testing.T) {
	defer withStubDriver(t)()
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

	// With new behavior, failed connections return nil
	assert.Nil(t, client1, "client should be nil on connection failure")
	assert.Nil(t, db1, "db should be nil on connection failure")
	assert.Nil(t, client2, "client should be nil on connection failure")
	assert.Nil(t, db2, "db should be nil on connection failure")
	assert.Error(t, err1)
	assert.Error(t, err2)
}

func TestMongoClient_ShutdownResets(t *testing.T) {
	defer withStubDriver(t)()
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
	assert.Nil(t, client1, "client should be nil on connection failure")
	assert.Nil(t, db1, "db should be nil on connection failure")

	err = Shutdown(ctx)
	assert.NoError(t, err)

	client2, db2, initErr := Init(ctx, cfg, log)
	require.Error(t, initErr)
	assert.Nil(t, client2, "client should be nil on connection failure")
	assert.Nil(t, db2, "db should be nil on connection failure")

	// Both should be nil, so they're "equal" in that sense
	assert.Equal(t, client1, client2, "both clients should be nil")
	assert.Equal(t, db1, db2, "both databases should be nil")
}

func TestMongoClient_Concurrency(t *testing.T) {
	defer withStubDriver(t)()
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

	// With new behavior, all should be nil since connection fails
	require.Nil(t, clients[0])
	require.Nil(t, dbs[0])

	for i := 1; i < goroutines; i++ {
		assert.Equal(t, clients[0], clients[i], "all clients should be nil")
		assert.Equal(t, dbs[0], dbs[i], "all databases should be nil")
	}
}

func TestMongoClient_AccessorsAfterInit(t *testing.T) {
	defer withStubDriver(t)()
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
	defer withStubDriver(t)()
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
	defer withStubDriver(t)()
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
	assert.Nil(t, client1, "client should be nil on connection failure")
	assert.Nil(t, db1, "db should be nil on connection failure")

	client2, db2, err2 := Init(ctx, cfg, log)
	assert.Equal(t, client1, client2, "both clients should be nil")
	assert.Equal(t, db1, db2, "both databases should be nil")
	assert.Error(t, err2)
}

// reset clears the singleton without going through Shutdown (helper for tests).
// test helper - do not call from prod code
func reset() {
	mu.Lock()
	defer mu.Unlock()
	client = nil
	db = nil
	initErr = nil
}
