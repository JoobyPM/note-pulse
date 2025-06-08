package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type mongoDriver struct{}

func (mongoDriver) Connect(_ context.Context, opts *options.ClientOptions) (*mongo.Client, error) {
	return mongo.Connect(opts)
}

func (mongoDriver) Ping(ctx context.Context, cli *mongo.Client) error {
	return cli.Ping(ctx, readpref.Primary())
}

func (mongoDriver) Disconnect(ctx context.Context, cli *mongo.Client) error {
	return cli.Disconnect(ctx)
}
