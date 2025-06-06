package mongo

import (
	"context"
	"errors"
	"time"

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
func NewUsersRepo(db *mongo.Database) *UsersRepo {
	collection := db.Collection("users")

	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Ignore error if index already exists
	_, _ = collection.Indexes().CreateOne(ctx, indexModel)

	return &UsersRepo{
		collection: collection,
	}
}

// Create creates a new user in the database
func (r *UsersRepo) Create(ctx context.Context, user *auth.User) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := r.collection.InsertOne(ctx, user)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return auth.ErrDuplicate
		}
		return err
	}

	return nil
}

// FindByEmail finds a user by email address
func (r *UsersRepo) FindByEmail(ctx context.Context, email string) (*auth.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var user auth.User
	err := r.collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, auth.ErrUserNotFound
		}
		return nil, err
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
		return nil, err
	}
	return &user, nil
}
