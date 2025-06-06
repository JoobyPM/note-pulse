package mongo

import (
	"context"
	"errors"
	"time"

	"note-pulse/internal/logger"
	"note-pulse/internal/services/notes"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const softLimit = 100

// NotesRepo implements the notes.NotesRepo interface for MongoDB
type NotesRepo struct {
	collection *mongo.Collection
}

// NewNotesRepo creates a new notes repository
func NewNotesRepo(db *mongo.Database) (*NotesRepo, error) {
	collection := db.Collection("notes")

	// Create compound index {user_id:1, _id:-1} for fast pagination
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "_id", Value: -1},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := collection.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		// Check if it's a duplicate key error (IndexOptionsConflict)
		if mongo.IsDuplicateKeyError(err) {
			logger.L().Debug("index already exists, continuing", "collection", "notes", "index", "user_id_-_id_-1")
		} else {
			logger.L().Error("failed to create index", "collection", "notes", "error", err)
			return nil, err
		}
	}

	return &NotesRepo{
		collection: collection,
	}, nil
}

// Create creates a new note in the database
func (r *NotesRepo) Create(ctx context.Context, note *notes.Note) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now()
	note.CreatedAt = now
	note.UpdatedAt = now

	_, err := r.collection.InsertOne(ctx, note)
	return err
}

// List retrieves notes for a user with cursor-based pagination
func (r *NotesRepo) List(ctx context.Context, userID bson.ObjectID, after bson.ObjectID, limit int) ([]*notes.Note, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Apply soft limit of 100
	if limit > softLimit {
		logger.L().Debug("list limit truncated", "user_id", userID.Hex(), "requested_limit", limit, "applied_limit", softLimit)
		limit = softLimit
	}

	filter := bson.M{"user_id": userID}

	// Add cursor condition if provided
	if !after.IsZero() {
		filter["_id"] = bson.M{"$lt": after}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: -1}}). // Sort by _id descending
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var notesList []*notes.Note
	if err := cursor.All(ctx, &notesList); err != nil {
		return nil, err
	}

	return notesList, nil
}

// Update updates a note belonging to the specified user
func (r *NotesRepo) Update(ctx context.Context, userID, noteID bson.ObjectID, patch notes.UpdateNote) (*notes.Note, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{
		"_id":     noteID,
		"user_id": userID,
	}

	update := bson.M{
		"$set": bson.M{
			"updated_at": time.Now(),
		},
	}

	// Only update fields that are provided
	if patch.Title != nil {
		update["$set"].(bson.M)["title"] = *patch.Title
	}
	if patch.Body != nil {
		update["$set"].(bson.M)["body"] = *patch.Body
	}
	if patch.Color != nil {
		update["$set"].(bson.M)["color"] = *patch.Color
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var updatedNote notes.Note
	err := r.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updatedNote)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, notes.ErrNoteNotFound
		}
		return nil, err
	}

	return &updatedNote, nil
}

// Delete deletes a note belonging to the specified user
func (r *NotesRepo) Delete(ctx context.Context, userID, noteID bson.ObjectID) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{
		"_id":     noteID,
		"user_id": userID,
	}

	result, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return notes.ErrNoteNotFound
	}

	return nil
}
