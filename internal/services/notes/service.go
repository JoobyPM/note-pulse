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
	HasMore              bool    `json:"has_more" example:"true"`
	TotalCount           int64   `json:"total_count" example:"125"`
	TotalCountUnfiltered int64   `json:"total_count_unfiltered" example:"200"`
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

	// Validate cursor if provided
	if req.Cursor != "" {
		return s.validateCursor(req.Cursor, req.Sort)
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

// List retrieves notes for a user with pagination
func (s *Service) List(ctx context.Context, userID bson.ObjectID, req ListNotesRequest) (*ListNotesResponse, error) {
	if err := s.validateListRequest(&req); err != nil {
		return nil, err
	}

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
