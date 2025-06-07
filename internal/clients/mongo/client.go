package mongo

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"note-pulse/internal/config"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	client       *mongo.Client
	db           *mongo.Database
	initErr      error
	mu           sync.RWMutex
	drv          driver = mongoDriver{}
	txnProbeOnce sync.Once
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

	cli, err := drv.Connect(ctx, opts)
	if err != nil {
		log.Error("failed to connect to mongo", "err", err)
		return nil, nil, err
	}

	// Retry ping with backoff for a total of ~5 seconds
	retries := []time.Duration{500 * time.Millisecond, 1 * time.Second, 2 * time.Second}
	var pingErr error

	for i, delay := range retries {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		pingErr = drv.Ping(pingCtx, cli)
		cancel()

		if pingErr == nil {
			break
		}

		log.Error("ping attempt failed", "attempt", i+1, "total_attempts", len(retries), "err", pingErr)

		// Don't sleep on the last attempt
		if i < len(retries)-1 {
			select {
			case <-ctx.Done():
				_ = drv.Disconnect(ctx, cli)
				return nil, nil, ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	if pingErr != nil {
		log.Error("failed to ping mongo after retries", "err", pingErr)
		_ = drv.Disconnect(ctx, cli)
		return nil, nil, fmt.Errorf("mongo ping: %w", pingErr)
	}

	// Detect replica-set / txn support after first successful ping
	txnProbeOnce.Do(func() {
		probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		var hello bson.M
		err := cli.Database("admin").RunCommand(probeCtx, bson.D{{Key: "hello", Value: 1}}).Decode(&hello)
		if err != nil {
			log.Warn("failed to probe transaction support, assuming standalone", "err", err)
			isReplicaSet.Store(false)
		} else {
			isReplicaSet.Store(hello["setName"] != nil)
			log.Info("detected MongoDB replica set", "replica_set", IsReplicaSet())
		}
	})

	database := cli.Database(cfg.MongoDBName)

	client = cli
	db = database
	initErr = nil

	log.Info("successfully connected to mongo", "db", cfg.MongoDBName)

	return client, db, nil
}

// Client returns the singleton MongoDB client instance.
func Client() *mongo.Client {
	mu.RLock()
	defer mu.RUnlock()
	return client
}

// DB returns the singleton MongoDB database instance.
func DB() *mongo.Database {
	mu.RLock()
	defer mu.RUnlock()
	return db
}

// SupportsTransactions returns whether the MongoDB instance supports transactions.
// This is detected during initialization via the "hello" command.
func SupportsTransactions() bool {
	return IsReplicaSet()
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

	err := drv.Disconnect(ctx, client)

	client = nil
	db = nil
	initErr = nil

	return err
}
