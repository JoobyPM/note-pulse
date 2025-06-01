package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type driver interface {
	Connect(ctx context.Context, opts *options.ClientOptions) (*mongo.Client, error)
	Ping(ctx context.Context, cli *mongo.Client) error
	Disconnect(ctx context.Context, cli *mongo.Client) error
}
