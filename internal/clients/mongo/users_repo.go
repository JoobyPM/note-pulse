package mongo

import (
	"context"
	"errors"
	"fmt"

	"note-pulse/internal/logger"
	"note-pulse/internal/services/auth"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// UsersRepo implements the auth.UsersRepo interface for MongoDB
type UsersRepo struct {
	collection *mongo.Collection
}

// NewUsersRepo creates a new users repository
func NewUsersRepo(parentCtx context.Context, db *mongo.Database) (*UsersRepo, error) {
	collection := db.Collection("users")

	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	}

	ctx, cancel := context.WithTimeout(parentCtx, OpTimeout)
	defer cancel()

	if _, err := collection.Indexes().CreateOne(ctx, indexModel); err != nil {
		// Duplicate index definition is fine - ignore it.
		if mongo.IsDuplicateKeyError(err) {
			logger.L().Debug("users index already exists")
		} else {
			// Anything else is unexpected -> bubble up so that Init() can fail fast.
			return nil, fmt.Errorf("create users index: %w", err)
		}
	}

	return &UsersRepo{
		collection: collection,
	}, nil
}

// Create creates a new user in the database
func (r *UsersRepo) Create(ctx context.Context, user *auth.User) error {
	ctx, cancel := WithRepoTimeout(ctx, OpTimeout)
	defer cancel()

	_, err := r.collection.InsertOne(ctx, user)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return auth.ErrDuplicate
		}
		return fmt.Errorf("failed to insert user: %w", err)
	}

	return nil
}

// FindByEmail finds a user by email address
func (r *UsersRepo) FindByEmail(ctx context.Context, email string) (*auth.User, error) {
	ctx, cancel := WithRepoTimeout(ctx, OpTimeout)
	defer cancel()

	var user auth.User
	err := r.collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, auth.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to find user by email: %w", err)
	}

	return &user, nil
}

// FindByID finds a user by their ID
func (r *UsersRepo) FindByID(ctx context.Context, id bson.ObjectID) (*auth.User, error) {
	var user auth.User
	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, auth.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to find user by ID: %w", err)
	}
	return &user, nil
}
