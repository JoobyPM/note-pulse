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

func repoCtx(parent context.Context) (context.Context, context.CancelFunc) {
	return WithRepoTimeout(parent, OpTimeout)
}

// calcCounts returns the filtered and unfiltered document counts in one place.
func (r *NotesRepo) calcCounts(
	ctx context.Context,
	userID bson.ObjectID,
	filter bson.M,
	hasFilters bool,
) (int64, int64, error) {
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to count documents: %w", err)
	}
	if !hasFilters {
		return total, total, nil
	}
	unfilteredFilter := bson.M{"user_id": userID}
	unfiltered, err := r.collection.CountDocuments(ctx, unfilteredFilter)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to count unfiltered documents: %w", err)
	}
	return total, unfiltered, nil
}

// translateNotFound maps the driver ErrNoDocuments to the domain-level ErrNoteNotFound.
func translateNotFound(err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return notes.ErrNoteNotFound
	}
	return fmt.Errorf("failed to translate not found: %w", err)
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
			Options: options.Index().SetName("user_title_asc_id_asc"),
		},
		{
			Keys: bson.D{
				{Key: "user_id", Value: 1},
				{Key: "title", Value: -1},
				{Key: "_id", Value: -1},
			},
			Options: options.Index().SetName("user_title_desc_id_desc"),
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
	ctx, cancel := repoCtx(ctx)
	defer cancel()

	now := time.Now()
	note.CreatedAt = now
	note.UpdatedAt = now

	if _, err := r.collection.InsertOne(ctx, note); err != nil {
		return fmt.Errorf("failed to insert note: %w", err)
	}
	return nil
}

// List retrieves notes for a user with filtering, search, sorting, and cursor-based pagination
func (r *NotesRepo) List(ctx context.Context, userID bson.ObjectID, req notes.ListNotesRequest, offset int) ([]*notes.Note, int64, int64, error) {
	ctx, cancel := repoCtx(ctx)
	defer cancel()

	// Use cursor filter for cursor-based pagination, or basic filter for offset-based
	var filter bson.M
	var err error
	if offset >= 0 {
		// Offset-based pagination: no cursor filters
		filter = r.buildBasicListFilter(userID, req)
	} else {
		// Cursor-based pagination: include cursor filters
		filter, err = r.buildListFilter(userID, req)
	}
	if err != nil {
		return nil, 0, 0, err
	}

	opts := r.buildFindOptions(req, req.Limit, offset)

	// Check if any actual filters are applied (excluding pagination)
	hasFilters := req.Color != "" || req.Q != ""

	totalCount, totalCountUnfiltered, err := r.calcCounts(ctx, userID, filter, hasFilters)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to calculate counts: %w", err)
	}

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, totalCount, totalCountUnfiltered, fmt.Errorf("failed to find notes: %w", err)
	}
	defer func(ctxToClose context.Context) {
		if cerr := cursor.Close(ctxToClose); cerr != nil {
			logger.L().Error("failed to close cursor", "error", cerr)
		}
	}(ctx)

	var notesList []*notes.Note
	if err := cursor.All(ctx, &notesList); err != nil {
		return nil, totalCount, totalCountUnfiltered, fmt.Errorf("failed to decode notes: %w", err)
	}

	return notesList, totalCount, totalCountUnfiltered, nil
}

// buildBasicListFilter constructs the MongoDB filter for offset-based queries (no cursor filters)
func (r *NotesRepo) buildBasicListFilter(userID bson.ObjectID, req notes.ListNotesRequest) bson.M {
	filter := bson.M{"user_id": userID}

	if req.Color != "" {
		filter["color"] = req.Color
	}

	r.addSearchFilter(filter, req.Q)

	// No cursor filter for offset-based pagination

	return filter
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
			return nil, fmt.Errorf("failed to build list filter: %w", err)
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
		return fmt.Errorf("invalid cursor format: %w", err)
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
func (r *NotesRepo) buildFindOptions(req notes.ListNotesRequest, limit int, offset int) *options.FindOptionsBuilder {
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

	opts := options.Find().
		SetSort(bson.D{{Key: sortKey, Value: dir}, {Key: "_id", Value: dir}}).
		SetLimit(int64(limit))

	// Only add skip when offset >= 0 (for offset-based pagination)
	if offset >= 0 {
		opts.SetSkip(int64(offset))
	}

	return opts
}

// Update updates a note belonging to the specified user
func (r *NotesRepo) Update(ctx context.Context, userID, noteID bson.ObjectID, patch notes.UpdateNote) (*notes.Note, error) {
	ctx, cancel := repoCtx(ctx)
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
			return nil, translateNotFound(err)
		}
		return &existingNote, nil
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)

	var updatedNote notes.Note
	err := r.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&updatedNote)
	if err != nil {
		return nil, translateNotFound(err)
	}

	return &updatedNote, nil
}

// Delete deletes a note belonging to the specified user
func (r *NotesRepo) Delete(ctx context.Context, userID, noteID bson.ObjectID) error {
	ctx, cancel := repoCtx(ctx)
	defer cancel()

	filter := bson.M{
		"_id":     noteID,
		"user_id": userID,
	}

	result, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to delete note: %w", err)
	}

	if result.DeletedCount == 0 {
		return notes.ErrNoteNotFound
	}

	return nil
}

// FindOne finds a single note by anchor and verifies it matches the filters
func (r *NotesRepo) FindOne(ctx context.Context, userID bson.ObjectID, req notes.ListNotesRequest, anchor string) (*notes.Note, error) {
	ctx, cancel := repoCtx(ctx)
	defer cancel()

	var noteID bson.ObjectID
	var err error

	if req.Sort == "title" {
		cursor, err := notes.DecodeCompositeCursor(anchor)
		if err != nil {
			return nil, fmt.Errorf("invalid anchor cursor: %w", err)
		}
		noteID = cursor.ID
	} else {
		noteID, err = bson.ObjectIDFromHex(anchor)
		if err != nil {
			return nil, fmt.Errorf("invalid anchor ID: %w", err)
		}
	}

	filter := bson.M{
		"_id":     noteID,
		"user_id": userID,
	}

	if req.Color != "" {
		filter["color"] = req.Color
	}

	r.addSearchFilter(filter, req.Q)

	var note notes.Note
	err = r.collection.FindOne(ctx, filter).Decode(&note)
	if err != nil {
		return nil, translateNotFound(err)
	}

	return &note, nil
}

// ListSide retrieves notes on one side of an anchor note
func (r *NotesRepo) ListSide(ctx context.Context, userID bson.ObjectID, req notes.ListNotesRequest, anchor *notes.Note, limit int, direction string) ([]*notes.Note, bool, error) {
	ctx, cancel := repoCtx(ctx)
	defer cancel()

	if limit <= 0 {
		return []*notes.Note{}, false, nil
	}

	// Clone the request and modify for side query
	sideReq := req
	sideReq.Limit = limit
	sideReq.Cursor = r.generateCursorFromNote(anchor, req.Sort)

	// Flip order for "before" direction to get reverse chronological order
	if direction == notes.DirectionBefore {
		if req.Order == "asc" {
			sideReq.Order = "desc"
		} else {
			sideReq.Order = "asc"
		}
	}

	// Build filter excluding the anchor note itself
	filter, err := r.buildListFilter(userID, sideReq)
	if err != nil {
		return nil, false, err
	}

	// Add cursor filter to position relative to anchor
	if err := r.addCursorFilter(filter, sideReq); err != nil {
		return nil, false, fmt.Errorf("failed to add cursor filter: %w", err)
	}

	// Offset is not used for side queries, so we pass -1
	opts := r.buildFindOptions(sideReq, limit, -1)

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, false, fmt.Errorf("failed to find notes: %w", err)
	}
	defer func(ctxToClose context.Context) {
		if cerr := cursor.Close(ctxToClose); cerr != nil {
			logger.L().Error("failed to close cursor", "error", cerr)
		}
	}(ctx)

	var notesList []*notes.Note
	if err := cursor.All(ctx, &notesList); err != nil {
		return nil, false, fmt.Errorf("failed to decode notes: %w", err)
	}

	// Return if we got a full result set (meaning there might be more)
	isFull := len(notesList) == limit

	return notesList, isFull, nil
}

// GetAnchorIndex gets the absolute index of an anchor note in the full sorted result set
func (r *NotesRepo) GetAnchorIndex(ctx context.Context, userID bson.ObjectID, req notes.ListNotesRequest, anchor *notes.Note) (int64, error) {
	ctx, cancel := repoCtx(ctx)
	defer cancel()

	sortKey := r.getSortKey(req.Sort)
	order := r.getSortOrder(req.Order)

	beforeFilter := r.buildBeforeFilter(userID, sortKey, order, anchor)
	r.applyFilters(beforeFilter, req)

	// Optional hint for large workspaces with duplicate titles
	opts := options.Count()
	if sortKey == "title" {
		// Check workspace size for hint optimization
		totalCount, err := r.collection.EstimatedDocumentCount(ctx)
		if err == nil && totalCount > 50000 {
			opts.SetHint(bson.D{{Key: "title", Value: 1}, {Key: "_id", Value: 1}})
		}
	}

	count, err := r.collection.CountDocuments(ctx, beforeFilter, opts)
	if err != nil {
		return -1, fmt.Errorf("failed to count documents: %w", err)
	}

	return count, nil
}

// getSortKey returns the validated sort key
func (r *NotesRepo) getSortKey(sort string) string {
	switch sort {
	case "created_at", "updated_at", "title":
		return sort
	default:
		return "created_at"
	}
}

// getSortOrder returns the validated sort order
func (r *NotesRepo) getSortOrder(order string) string {
	if order == "asc" {
		return "asc"
	}
	return "desc"
}

// buildBeforeFilter builds the filter for documents before the anchor
func (r *NotesRepo) buildBeforeFilter(userID bson.ObjectID, sortKey, order string, anchor *notes.Note) bson.M {
	if sortKey == "title" {
		return r.buildTitleBeforeFilter(userID, order, anchor)
	}
	return r.buildDateBeforeFilter(userID, sortKey, order, anchor)
}

// buildTitleBeforeFilter builds the before filter for title sorting
func (r *NotesRepo) buildTitleBeforeFilter(userID bson.ObjectID, order string, anchor *notes.Note) bson.M {
	operator := "$lt"
	if order == "desc" {
		operator = "$gt"
	}

	return bson.M{
		"user_id": userID,
		"$or": bson.A{
			bson.M{"title": bson.M{operator: anchor.Title}},
			bson.M{
				"title": anchor.Title,
				"_id":   bson.M{operator: anchor.ID},
			},
		},
	}
}

// buildDateBeforeFilter builds the before filter for date/time sorting
func (r *NotesRepo) buildDateBeforeFilter(userID bson.ObjectID, sortKey, order string, anchor *notes.Note) bson.M {
	anchorValue := anchor.CreatedAt
	if sortKey == "updated_at" {
		anchorValue = anchor.UpdatedAt
	}

	operator := "$lt"
	if order == "desc" {
		operator = "$gt"
	}

	return bson.M{
		"user_id": userID,
		"$or": bson.A{
			bson.M{sortKey: bson.M{operator: anchorValue}},
			bson.M{
				sortKey: anchorValue,
				"_id":   bson.M{operator: anchor.ID},
			},
		},
	}
}

// applyFilters applies color and search filters to the given filter
func (r *NotesRepo) applyFilters(filter bson.M, req notes.ListNotesRequest) {
	if req.Color != "" {
		filter["color"] = req.Color
	}
	r.addSearchFilter(filter, req.Q)
}

// GetCounts gets the total and unfiltered counts for the current request
func (r *NotesRepo) GetCounts(ctx context.Context, userID bson.ObjectID, req notes.ListNotesRequest) (int64, int64, error) {
	ctx, cancel := repoCtx(ctx)
	defer cancel()

	filter := bson.M{"user_id": userID}
	if req.Color != "" {
		filter["color"] = req.Color
	}
	r.addSearchFilter(filter, req.Q)

	hasFilters := req.Color != "" || req.Q != ""

	return r.calcCounts(ctx, userID, filter, hasFilters)
}

// generateCursorFromNote generates a cursor string from a note based on sort criteria
func (r *NotesRepo) generateCursorFromNote(note *notes.Note, sort string) string {
	if sort == "title" {
		return notes.EncodeCompositeCursor(note.Title, note.ID)
	}
	return note.ID.Hex()
}
