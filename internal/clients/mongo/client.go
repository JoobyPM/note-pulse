// Package mongo provides a thread-safe singleton connection to MongoDB.
// It guarantees at most one Connect / Ping / Disconnect sequence.
package mongo

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"note-pulse/internal/config"

	"github.com/cenkalti/backoff/v4"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	// sentinel errors
	ErrNotInitialized = errors.New("mongo: Init has not completed successfully")
	ErrShutdown       = errors.New("mongo: Shutdown has already run")

	// immutable state after Init
	client  *mongo.Client
	db      *mongo.Database
	initErr error

	initOnce     sync.Once
	shutdownOnce sync.Once
	txnProbeOnce sync.Once
	drv          driver = mongoDriver{}
)

const (
	txnProbeTimeout  = 2 * time.Second
	connectTimeout   = 10 * time.Second
	pingTimeout      = 2 * time.Second
	shutdownTimeout  = 5 * time.Second
	totalPingBudget  = 5 * time.Second
	initialPingDelay = 500 * time.Millisecond
	maxPingDelay     = 2 * time.Second
	appName          = "note-pulse"
)

// Init establishes the MongoDB connection once. Callers always receive
// the same (*mongo.Client, *mongo.Database, error) triple.
func Init(ctx context.Context, cfg config.Config, log *slog.Logger) (*mongo.Client, *mongo.Database, error) {
	initOnce.Do(func() {
		opts := options.Client().
			ApplyURI(cfg.MongoURI).
			SetServerAPIOptions(options.ServerAPI(options.ServerAPIVersion1)).
			SetConnectTimeout(connectTimeout).
			SetAppName(appName)

		ctxConnect, cancelConnect := context.WithTimeout(ctx, connectTimeout)
		defer cancelConnect()

		var cli *mongo.Client
		cli, initErr = drv.Connect(ctxConnect, opts)
		if initErr != nil {
			initErr = fmt.Errorf("mongo: connect %q: %w", cfg.MongoURI, initErr)
			log.Error("connect failed", "err", initErr)
			return
		}

		// exponential back-off with jitter while Ping keeps failing
		boCfg := backoff.NewExponentialBackOff()
		boCfg.InitialInterval = initialPingDelay
		boCfg.MaxInterval = maxPingDelay
		boCfg.MaxElapsedTime = totalPingBudget

		initErr = backoff.RetryNotify(
			func() error {
				pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
				defer cancel()
				return drv.Ping(pingCtx, cli)
			},
			backoff.WithContext(boCfg, ctx),
			func(err error, next time.Duration) {
				log.Warn("ping failed, retrying", "retry_in", next, "err", err)
			},
		)
		if initErr != nil {
			// try to close the half-open connection; join errors if both fail
			if discErr := drv.Disconnect(ctx, cli); discErr != nil {
				initErr = errors.Join(
					fmt.Errorf("mongo: ping after connect: %w", initErr),
					fmt.Errorf("mongo: disconnect after failed ping: %w", discErr),
				)
			} else {
				initErr = fmt.Errorf("mongo: ping after connect: %w", initErr)
			}
			log.Error("unable to initialise mongo", "err", initErr)
			return
		}

		// detect replica-set support only once
		txnProbeOnce.Do(func() {
			probeCtx, cancel := context.WithTimeout(ctx, txnProbeTimeout)
			defer cancel()

			var hello bson.M
			if err := cli.Database("admin").RunCommand(
				probeCtx, bson.D{{Key: "hello", Value: 1}},
			).Decode(&hello); err != nil {
				log.Warn("txn probe failed, assuming standalone", "err", err)
				isReplicaSet.Store(false)
			} else {
				isReplicaSet.Store(hello["setName"] != nil)
				log.Info("replica set status determined", "replica_set", IsReplicaSet())
			}
		})

		db = cli.Database(cfg.MongoDBName)
		client = cli
		log.Info("mongo connected", "db", cfg.MongoDBName)
	})

	return client, db, initErr
}

// Client returns the singleton client or nil if Init failed.
func Client() *mongo.Client { return client }

// DB returns the singleton database or nil if Init failed.
func DB() *mongo.Database { return db }

// MustClient returns the client or panics with a helpful message.
// Handy in main() when you want to fail fast.
func MustClient() *mongo.Client {
	if client == nil {
		panic(ErrNotInitialized)
	}
	return client
}

// MustDB returns the database or panics.
func MustDB() *mongo.Database {
	if db == nil {
		panic(ErrNotInitialized)
	}
	return db
}

// Shutdown disconnects exactly once and is safe to call from many goroutines.
// Further calls return ErrShutdown.
func Shutdown(ctx context.Context) error {
	var result error
	called := false

	shutdownOnce.Do(func() {
		called = true
		if client == nil {
			result = ErrNotInitialized
			return
		}
		ctx, cancel := context.WithTimeout(ctx, shutdownTimeout)
		defer cancel()
		if err := drv.Disconnect(ctx, client); err != nil {
			result = fmt.Errorf("mongo: disconnect: %w", err)
			return
		}
	})

	if !called {
		return ErrShutdown
	}
	return result
}
