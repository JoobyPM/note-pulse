package mongo

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"note-pulse/internal/config"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

var (
	client  *mongo.Client
	db      *mongo.Database
	initErr error
	mu      sync.Mutex
)

// Init initializes the MongoDB connection (first call wins, thread-safe).
func Init(ctx context.Context, cfg config.Config, log *slog.Logger) (*mongo.Client, *mongo.Database, error) {
	mu.Lock()
	defer mu.Unlock()

	if client != nil && db != nil {
		return client, db, initErr
	}

	opts := options.Client().
		ApplyURI(cfg.MongoURI).
		SetServerAPIOptions(options.ServerAPI(options.ServerAPIVersion1)).
		SetConnectTimeout(10 * time.Second).
		SetAppName("note-pulse")

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cli, err := mongo.Connect(opts)
	if err != nil {
		log.Error("failed to connect to mongo", "err", err)
		return nil, nil, err
	}

	pingErr := cli.Ping(ctx, readpref.Primary())
	if pingErr != nil {
		log.Error("failed to ping mongo", "err", pingErr)
	}

	database := cli.Database(cfg.MongoDBName)

	client = cli
	db = database
	initErr = pingErr

	if pingErr == nil {
		log.Info("successfully connected to mongo", "db", cfg.MongoDBName)
	}

	return client, db, pingErr
}

// Client returns the singleton MongoDB client instance.
func Client() *mongo.Client {
	mu.Lock()
	defer mu.Unlock()
	return client
}

// DB returns the singleton MongoDB database instance.
func DB() *mongo.Database {
	mu.Lock()
	defer mu.Unlock()
	return db
}

// Shutdown gracefully shuts down the MongoDB connection.
// Safe to call more than once.
func Shutdown(ctx context.Context) error {
	mu.Lock()
	defer mu.Unlock()

	if client == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := client.Disconnect(ctx)

	client = nil
	db = nil
	initErr = nil

	return err
}
