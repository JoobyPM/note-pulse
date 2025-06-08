package notes

import (
	"context"
	"errors"
	"log/slog"
	"time"

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
	Order  string `query:"order"  validate:"omitempty,oneof=asc desc" example:"desc"`
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

// TODO: [validation] idea «Add HTML sanitisation. Decide on required feature set (Markdown? plaintext?) and use `github.com/microcosm-cc/bluemonday` or similar to sanitise on write, not on read.»

// Create creates a new note
func (s *Service) Create(ctx context.Context, userID bson.ObjectID, req CreateNoteRequest) (*NoteResponse, error) {
	now := time.Now()
	note := &Note{
		ID:        bson.NewObjectID(),
		UserID:    userID,
		Title:     req.Title,
		Body:      req.Body,
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

// List retrieves notes for a user with pagination
func (s *Service) List(ctx context.Context, userID bson.ObjectID, req ListNotesRequest) (*ListNotesResponse, error) {
	// Set default limit if not provided
	if req.Limit == 0 {
		req.Limit = 50
	}

	// Validate limit - return error instead of silently clipping
	if req.Limit > 100 {
		return nil, ErrInvalidLimit
	}

	// Validate cursor if provided
	if req.Cursor != "" {
		_, err := bson.ObjectIDFromHex(req.Cursor)
		if err != nil {
			return nil, ErrInvalidCursor
		}
	}

	// Fetch limit+1 to determine if there are more results
	fetchLimit := req.Limit + 1
	fetchReq := req
	fetchReq.Limit = fetchLimit

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
	if hasMore && len(notes) > 0 {
		response.NextCursor = notes[len(notes)-1].ID.Hex()
	}

	return response, nil
}

// Update updates a note belonging to the user
func (s *Service) Update(ctx context.Context, userID, noteID bson.ObjectID, req UpdateNoteRequest) (*NoteResponse, error) {
	patch := UpdateNote(req)

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
