package mongo

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"note-pulse/internal/logger"
	"note-pulse/internal/services/notes"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// NotesRepo implements the notes.Repository interface for MongoDB
type NotesRepo struct {
	collection *mongo.Collection
}

// NewNotesRepo creates a new notes repository
func NewNotesRepo(parentCtx context.Context, db *mongo.Database) (*NotesRepo, error) {
	collection := db.Collection("notes")

	// Create compound indexes for performance
	indexes := []mongo.IndexModel{
		// Existing index for default pagination
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "_id", Value: -1},
			},
		},
		// Index for updated_at sorting
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "updated_at", Value: -1},
				{Key: "_id", Value: -1},
			},
		},
		// Index for created_at sorting
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "created_at", Value: -1},
				{Key: "_id", Value: -1},
			},
		},
		// Index for title sorting with composite cursor support
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "title", Value: 1},
				{Key: "_id", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "title", Value: -1},
				{Key: "_id", Value: -1},
			},
		},
		// Text search index for title and body
		{
			Keys: bson.D{
				{Key: "title", Value: "text"},
				{Key: "body", Value: "text"},
			},
		},
	}

	ctx, cancel := context.WithTimeout(parentCtx, OpTimeout)
	defer cancel()

	for _, indexModel := range indexes {
		_, err := collection.Indexes().CreateOne(ctx, indexModel)
		if err != nil {
			// Check if it's a duplicate key error (IndexOptionsConflict)
			if mongo.IsDuplicateKeyError(err) {
				logger.L().Debug("index already exists, continuing", "collection", "notes")
			} else {
				logger.L().Error("failed to create index", "collection", "notes", "error", err)
				return nil, fmt.Errorf("failed to create notes collection index: %w", err)
			}
		}
	}

	return &NotesRepo{
		collection: collection,
	}, nil
}

// Create creates a new note in the database
func (r *NotesRepo) Create(ctx context.Context, note *notes.Note) error {
	ctx, cancel := WithRepoTimeout(ctx, OpTimeout)
	defer cancel()

	now := time.Now()
	note.CreatedAt = now
	note.UpdatedAt = now

	_, err := r.collection.InsertOne(ctx, note)
	return err
}

// List retrieves notes for a user with filtering, search, sorting, and cursor-based pagination
func (r *NotesRepo) List(ctx context.Context, userID bson.ObjectID, req notes.ListNotesRequest) ([]*notes.Note, int64, int64, error) {
	ctx, cancel := WithRepoTimeout(ctx, OpTimeout)
	defer cancel()

	filter, err := r.buildListFilter(userID, req)
	if err != nil {
		return nil, 0, 0, err
	}

	opts := r.buildFindOptions(req, req.Limit)

	var totalCount, totalCountUnfiltered int64

	// Check if any actual filters are applied (excluding pagination cursor)
	hasFilters := req.Color != "" || req.Q != ""

	if hasFilters {
		// Get total count with filters
		var errCountWithFilters error
		totalCount, errCountWithFilters = r.collection.CountDocuments(ctx, filter)
		if errCountWithFilters != nil {
			return nil, 0, 0, errCountWithFilters
		}

		// Get total count without filters (only user_id filter)
		unfilteredFilter := bson.M{"user_id": userID}
		var errCountUnfiltered error
		totalCountUnfiltered, errCountUnfiltered = r.collection.CountDocuments(ctx, unfilteredFilter)
		if errCountUnfiltered != nil {
			return nil, 0, 0, errCountUnfiltered
		}
	} else {
		// No filters applied, so filtered count = unfiltered count
		var errCount error
		totalCount, errCount = r.collection.CountDocuments(ctx, filter)
		if errCount != nil {
			return nil, 0, 0, errCount
		}
		totalCountUnfiltered = totalCount
	}

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, totalCount, totalCountUnfiltered, err
	}
	defer func(ctxToClose context.Context) {
		if err := cursor.Close(ctxToClose); err != nil {
			logger.L().Error("failed to close cursor", "error", err)
		}
	}(ctx)

	var notesList []*notes.Note
	if err := cursor.All(ctx, &notesList); err != nil {
		return nil, totalCount, totalCountUnfiltered, err
	}

	return notesList, totalCount, totalCountUnfiltered, nil
}

// buildListFilter constructs the MongoDB filter for the List query
func (r *NotesRepo) buildListFilter(userID bson.ObjectID, req notes.ListNotesRequest) (bson.M, error) {
	filter := bson.M{"user_id": userID}

	if req.Color != "" {
		filter["color"] = req.Color
	}

	r.addSearchFilter(filter, req.Q)

	if req.Cursor != "" {
		if err := r.addCursorFilter(filter, req); err != nil {
			return nil, err
		}
	}

	return filter, nil
}

// addSearchFilter adds search conditions to the filter
func (r *NotesRepo) addSearchFilter(filter bson.M, query string) {
	if query == "" {
		return
	}

	if len(query) >= 3 {
		// Use MongoDB text search for better performance
		filter["$text"] = bson.M{"$search": query}
	} else {
		// Fall back to regex for short queries
		pattern := regexp.QuoteMeta(query)
		regex := bson.M{"$regex": pattern, "$options": "i"}
		filter["$or"] = bson.A{
			bson.M{"title": regex},
			bson.M{"body": regex},
		}
	}
}

// addCursorFilter adds cursor pagination conditions to the filter
func (r *NotesRepo) addCursorFilter(filter bson.M, req notes.ListNotesRequest) error {
	if req.Sort == "title" {
		return r.addTitleCursorFilter(filter, req.Cursor, req.Order)
	}
	return r.addObjectIDCursorFilter(filter, req.Cursor, req.Order)
}

// addObjectIDCursorFilter adds simple ObjectID cursor pagination filter
func (r *NotesRepo) addObjectIDCursorFilter(filter bson.M, cursorStr, order string) error {
	after, err := bson.ObjectIDFromHex(cursorStr)
	if err != nil {
		return err
	}

	if after.IsZero() {
		return nil
	}

	if order == "asc" {
		filter["_id"] = bson.M{"$gt": after}
	} else {
		filter["_id"] = bson.M{"$lt": after}
	}

	return nil
}

// addTitleCursorFilter adds cursor pagination filter for title-based sorting
func (r *NotesRepo) addTitleCursorFilter(filter bson.M, cursorStr, order string) error {
	cursor, err := notes.DecodeCompositeCursor(cursorStr)
	if err != nil {
		return fmt.Errorf("invalid cursor format: %w", err)
	}

	// Build compound filter based on sort order
	if order == "asc" {
		// For ascending: title > cursor.Title OR (title = cursor.Title AND _id > cursor.ID)
		filter["$or"] = bson.A{
			bson.M{"title": bson.M{"$gt": cursor.Title}},
			bson.M{
				"title": cursor.Title,
				"_id":   bson.M{"$gt": cursor.ID},
			},
		}
	} else {
		// For descending: title < cursor.Title OR (title = cursor.Title AND _id < cursor.ID)
		filter["$or"] = bson.A{
			bson.M{"title": bson.M{"$lt": cursor.Title}},
			bson.M{
				"title": cursor.Title,
				"_id":   bson.M{"$lt": cursor.ID},
			},
		}
	}

	return nil
}

// buildFindOptions constructs the MongoDB find options for sorting and pagination
func (r *NotesRepo) buildFindOptions(req notes.ListNotesRequest, limit int) *options.FindOptionsBuilder {
	sortKey := "created_at"
	if req.Sort != "" {
		switch req.Sort {
		case "created_at", "updated_at", "title":
			sortKey = req.Sort
		default:
			sortKey = "created_at"
		}
	}

	dir := -1 // Default to descending
	if req.Order == "asc" {
		dir = 1
	}

	return options.Find().
		SetSort(bson.D{{Key: sortKey, Value: dir}, {Key: "_id", Value: dir}}).
		SetLimit(int64(limit))
}

// Update updates a note belonging to the specified user
func (r *NotesRepo) Update(ctx context.Context, userID, noteID bson.ObjectID, patch notes.UpdateNote) (*notes.Note, error) {
	ctx, cancel := WithRepoTimeout(ctx, OpTimeout)
	defer cancel()

	filter := bson.M{
		"_id":     noteID,
		"user_id": userID,
	}

	update := bson.M{
		"$set": bson.M{
			"updated_at": time.Now().UTC(),
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

	// Skip update if only updated_at would be set (micro-optimization)
	if len(update["$set"].(bson.M)) == 1 {
		var existingNote notes.Note
		err := r.collection.FindOne(ctx, filter).Decode(&existingNote)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, notes.ErrNoteNotFound
			}
			return nil, err
		}
		return &existingNote, nil
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
	ctx, cancel := WithRepoTimeout(ctx, OpTimeout)
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
