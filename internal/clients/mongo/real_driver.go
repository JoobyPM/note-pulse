package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type mongoDriver struct{}

func (mongoDriver) Connect(_ context.Context, opts *options.ClientOptions) (*mongo.Client, error) {
	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to mongo: %w", err)
	}
	return client, nil
}

func (mongoDriver) Ping(ctx context.Context, cli *mongo.Client) error {
	err := cli.Ping(ctx, readpref.Primary())
	if err != nil {
		return fmt.Errorf("failed to ping mongo: %w", err)
	}
	return nil
}

func (mongoDriver) Disconnect(ctx context.Context, cli *mongo.Client) error {
	err := cli.Disconnect(ctx)
	if err != nil {
		return fmt.Errorf("failed to disconnect from mongo: %w", err)
	}
	return nil
}
