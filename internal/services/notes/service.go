package notes

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"note-pulse/internal/utils/sanitize"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Service handles notes business logic
type Service struct {
	repo Repository
	bus  Bus
	log  *slog.Logger
}

// NewService creates a new notes service
func NewService(repo Repository, bus Bus, log *slog.Logger) *Service {
	return &Service{
		repo: repo,
		bus:  bus,
		log:  log,
	}
}

// CreateNoteRequest represents a note creation request
type CreateNoteRequest struct {
	Title string `json:"title" validate:"required" example:"Meeting Notes"`
	Body  string `json:"body" example:"Remember to discuss the quarterly targets"`
	Color string `json:"color" validate:"omitempty,hexcolor" example:"#FFD700"`
}

// UpdateNoteRequest represents a note update request
type UpdateNoteRequest struct {
	Title *string `json:"title,omitempty" validate:"omitempty,min=1" example:"Updated Meeting Notes"`
	Body  *string `json:"body,omitempty" example:"Updated content for the meeting"`
	Color *string `json:"color,omitempty" validate:"omitempty,hexcolor" example:"#FF6B6B"`
}

// ListNotesRequest represents a list notes request
type ListNotesRequest struct {
	Limit  int    `query:"limit"  validate:"omitempty,min=1,max=100" example:"50"`
	Cursor string `query:"cursor" validate:"omitempty" example:"683cdb8aa96ad71e8e075bd1"`
	Anchor string `query:"anchor" validate:"omitempty" example:"683cdb8aa96ad71e8e075bd1"`
	Span   int    `query:"span"   validate:"omitempty,min=3,max=100" example:"40"`
	Q      string `query:"q"      validate:"omitempty,min=1,max=256" example:"meeting"`
	Color  string `query:"color"  validate:"omitempty" example:"#FF0000"`
	Sort   string `query:"sort"   validate:"omitempty,oneof=created_at updated_at title" example:"created_at"`
	Order  string `query:"order"  validate:"omitempty,oneof=asc desc" example:"desc"` // order is case-insensitive.
}

// NoteResponse represents a single note response
type NoteResponse struct {
	Note *Note `json:"note"`
}

// ListNotesResponse represents a list of notes response
type ListNotesResponse struct {
	Notes                []*Note `json:"notes"`
	NextCursor           string  `json:"next_cursor,omitempty" example:"683cdb8aa96ad71e8e075bd2"`
	PrevCursor           string  `json:"prev_cursor,omitempty" example:"683cdb8aa96ad71e8e075bd0"`
	HasMore              bool    `json:"has_more" example:"true"`
	TotalCount           int64   `json:"total_count" example:"125"`
	TotalCountUnfiltered int64   `json:"total_count_unfiltered" example:"200"`
	AnchorIndex          int64   `json:"anchor_index,omitempty" example:"24"`
}

// ErrNoteNotFound - note not found in DB
var ErrNoteNotFound = errors.New("note not found")

// Create creates a new note
func (s *Service) Create(ctx context.Context, userID bson.ObjectID, req CreateNoteRequest) (*NoteResponse, error) {
	now := time.Now()
	note := &Note{
		ID:        bson.NewObjectID(),
		UserID:    userID,
		Title:     sanitize.Clean(req.Title),
		Body:      sanitize.Clean(req.Body),
		Color:     req.Color,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Create(ctx, note); err != nil {
		s.log.Error(ErrCreateNote.Error(), "error", err, "user_id", userID.Hex())
		return nil, ErrCreateNote
	}

	s.bus.Broadcast(ctx, NoteEvent{
		Type: "created",
		Note: note,
	})

	return &NoteResponse{Note: note}, nil
}

// validateListRequest validates the list request parameters
func (s *Service) validateListRequest(req *ListNotesRequest) error {
	// Set default limit if not provided
	if req.Limit == 0 {
		req.Limit = 50
	}

	// Normalize order field to lowercase for case-insensitive handling
	if req.Order != "" {
		req.Order = strings.ToLower(req.Order)
	}

	// Validate limit - return error instead of silently clipping
	if req.Limit > 100 {
		return ErrInvalidLimit
	}

	// Validate that anchor and cursor cannot be used together
	if req.Anchor != "" && req.Cursor != "" {
		s.log.Warn("anchor and cursor cannot be used together", "anchor", req.Anchor, "cursor", req.Cursor)
		return ErrBadRequest
	}

	// Validate cursor if provided
	if req.Cursor != "" {
		return s.validateCursor(req.Cursor, req.Sort)
	}

	// Validate anchor if provided
	if req.Anchor != "" {
		return s.validateCursor(req.Anchor, req.Sort)
	}

	return nil
}

// validateCursor validates the cursor format based on sort type
func (s *Service) validateCursor(cursor, sort string) error {
	if sort == "title" {
		// Validate composite cursor format
		_, err := DecodeCompositeCursor(cursor)
		if err != nil {
			return ErrInvalidCursor
		}
	} else {
		// Validate ObjectID cursor format
		_, err := bson.ObjectIDFromHex(cursor)
		if err != nil {
			return ErrInvalidCursor
		}
	}
	return nil
}

// generateNextCursor generates the next cursor for pagination
func (s *Service) generateNextCursor(notes []*Note, sort string) string {
	if len(notes) == 0 {
		return ""
	}

	last := notes[len(notes)-1]
	if sort == "title" {
		// Use composite cursor for title sorting
		return EncodeCompositeCursor(last.Title, last.ID)
	}
	// Use simple ObjectID cursor for other sorts
	return last.ID.Hex()
}

// generatePrevCursor generates the previous cursor for pagination
func (s *Service) generatePrevCursor(notes []*Note, sort string) string {
	if len(notes) == 0 {
		return ""
	}

	first := notes[0]
	if sort == "title" {
		return EncodeCompositeCursor(first.Title, first.ID)
	}
	return first.ID.Hex()
}

// reverse reverses a slice of notes
func (s *Service) reverse(notes []*Note) []*Note {
	for i, j := 0, len(notes)-1; i < j; i, j = i+1, j-1 {
		notes[i], notes[j] = notes[j], notes[i]
	}
	return notes
}

// List retrieves notes for a user with pagination
func (s *Service) List(ctx context.Context, userID bson.ObjectID, req ListNotesRequest) (*ListNotesResponse, error) {
	if err := s.validateListRequest(&req); err != nil {
		return nil, err
	}

	// If no anchor is provided, use the old behavior
	if req.Anchor == "" {
		return s.oldList(ctx, userID, req)
	}

	// Use the new anchor-based pagination
	return s.anchorList(ctx, userID, req)
}

// oldList implements the original pagination behavior
func (s *Service) oldList(ctx context.Context, userID bson.ObjectID, req ListNotesRequest) (*ListNotesResponse, error) {
	// Fetch limit+1 to determine if there are more results
	fetchReq := req
	fetchReq.Limit = req.Limit + 1

	notes, totalCount, totalCountUnfiltered, err := s.repo.List(ctx, userID, fetchReq)
	if err != nil {
		s.log.Error(ErrListNotes.Error(), "error", err, "user_id", userID.Hex())
		return nil, ErrListNotes
	}

	// Determine if there are more results and trim to requested limit
	hasMore := len(notes) > req.Limit
	if hasMore {
		notes = notes[:req.Limit]
	}

	response := &ListNotesResponse{
		Notes:                notes,
		HasMore:              hasMore,
		TotalCount:           totalCount,
		TotalCountUnfiltered: totalCountUnfiltered,
	}

	// Set next cursor only if we have more results
	if hasMore {
		response.NextCursor = s.generateNextCursor(notes, req.Sort)
	}

	return response, nil
}

// anchorList implements the new anchor-based pagination
func (s *Service) anchorList(ctx context.Context, userID bson.ObjectID, req ListNotesRequest) (*ListNotesResponse, error) {
	// Set default span if not provided
	winSz := req.Span
	if winSz <= 0 || winSz > 100 {
		winSz = req.Limit // Use existing limit as default
		if winSz <= 0 {
			winSz = 50
		}
	}

	// 1. Load the anchor note and verify it matches filters
	anchor, err := s.repo.FindOne(ctx, userID, req, req.Anchor)
	if err != nil {
		s.log.Info("anchor note not found or filtered out", "user_id", userID.Hex(), "anchor", req.Anchor)
		return nil, ErrNoteNotFound
	}

	// 2. Split the window
	beforeN := winSz / 2
	afterN := winSz - beforeN - 1 // 1 slot for the anchor itself

	// 3. Get notes before the anchor (parallel execution would be ideal, but sequential is simpler)
	before, err := s.repo.ListSide(ctx, userID, req, anchor, beforeN, "before")
	if err != nil {
		s.log.Error("failed to get notes before anchor", "error", err, "user_id", userID.Hex())
		return nil, ErrListNotes
	}

	// 4. Get notes after the anchor
	after, err := s.repo.ListSide(ctx, userID, req, anchor, afterN, "after")
	if err != nil {
		s.log.Error("failed to get notes after anchor", "error", err, "user_id", userID.Hex())
		return nil, ErrListNotes
	}

	// 5. Get anchor index for absolute positioning
	anchorIndex, err := s.repo.GetAnchorIndex(ctx, userID, req, anchor)
	if err != nil {
		s.log.Error("failed to get anchor index", "error", err, "user_id", userID.Hex())
		// Don't fail the request, just omit the index
		anchorIndex = -1
	}

	// 6. Combine results, keeping overall order stable
	notes := append(s.reverse(before), anchor)
	notes = append(notes, after...)

	// 7. Get total counts
	totalCount, totalCountUnfiltered, err := s.repo.GetCounts(ctx, userID, req)
	if err != nil {
		s.log.Error("failed to get counts", "error", err, "user_id", userID.Hex())
		// Don't fail the request, just use 0 counts
		totalCount, totalCountUnfiltered = 0, 0
	}

	response := &ListNotesResponse{
		Notes:                notes,
		HasMore:              len(after) == afterN || len(before) == beforeN,
		TotalCount:           totalCount,
		TotalCountUnfiltered: totalCountUnfiltered,
	}

	// Set cursors based on the window edges
	if len(before) > 0 {
		response.PrevCursor = s.generatePrevCursor(before, req.Sort)
	}
	if len(after) > 0 {
		response.NextCursor = s.generateNextCursor(after, req.Sort)
	}

	// Set anchor index if we got it
	if anchorIndex >= 0 {
		response.AnchorIndex = anchorIndex
	}

	return response, nil
}

// sanitizedUpdateNote creates an UpdateNote with sanitized title and body
func sanitizedUpdateNote(req UpdateNoteRequest) UpdateNote {
	patch := UpdateNote(req)

	if patch.Title != nil {
		sanitized := sanitize.Clean(*patch.Title)
		patch.Title = &sanitized
	}
	if patch.Body != nil {
		sanitized := sanitize.Clean(*patch.Body)
		patch.Body = &sanitized
	}

	return patch
}

// Update updates a note belonging to the user
func (s *Service) Update(ctx context.Context, userID, noteID bson.ObjectID, req UpdateNoteRequest) (*NoteResponse, error) {
	patch := sanitizedUpdateNote(req)

	updatedNote, err := s.repo.Update(ctx, userID, noteID, patch)
	if err != nil {
		if errors.Is(err, ErrNoteNotFound) {
			s.log.Info("note not found for update", "user_id", userID.Hex(), "note_id", noteID.Hex())
			return nil, ErrNoteNotFound
		}
		s.log.Error(ErrUpdateNote.Error(), "error", err, "user_id", userID.Hex(), "note_id", noteID.Hex())
		return nil, ErrUpdateNote
	}

	s.bus.Broadcast(ctx, NoteEvent{
		Type: "updated",
		Note: updatedNote,
	})

	return &NoteResponse{Note: updatedNote}, nil
}

// Delete deletes a note belonging to the user
func (s *Service) Delete(ctx context.Context, userID, noteID bson.ObjectID) error {
	if err := s.repo.Delete(ctx, userID, noteID); err != nil {
		if errors.Is(err, ErrNoteNotFound) {
			s.log.Info("note not found for delete", "user_id", userID.Hex(), "note_id", noteID.Hex())
			return ErrNoteNotFound
		}
		s.log.Error(ErrDeleteNote.Error(), "error", err, "user_id", userID.Hex(), "note_id", noteID.Hex())
		return ErrDeleteNote
	}

	// Broadcast deletion event with minimal note data
	deletedNote := &Note{
		ID:     noteID,
		UserID: userID,
	}

	s.bus.Broadcast(ctx, NoteEvent{
		Type: "deleted",
		Note: deletedNote,
	})

	return nil
}
