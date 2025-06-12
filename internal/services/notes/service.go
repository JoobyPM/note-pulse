package notes

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"time"

	"note-pulse/internal/utils/sanitize"

	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/sync/errgroup"
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
	Span   int    `query:"span"   validate:"omitempty,min=1,max=100" example:"40"`
	Q      string `query:"q"      validate:"omitempty,min=1,max=256" example:"meeting"`
	Color  string `query:"color"  validate:"omitempty" example:"#FF0000"`
	Sort   string `query:"sort"   validate:"omitempty,oneof=created_at updated_at title" example:"created_at"` // sort is case-insensitive.
	Order  string `query:"order"  validate:"omitempty,oneof=asc desc" example:"desc"`                          // order is case-insensitive.
	// nil   parameter was absent
	// 0..N  parameter was supplied
	Offset *int `query:"offset" json:"offset,omitempty" validate:"omitempty,min=0,max=10000" example:"300"`
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
	WindowSize           int     `json:"window_size" example:"20"`
	Offset               int     `json:"offset,omitempty" example:"300"`
	TotalPages           int     `json:"total_pages,omitempty" example:"25"`
}

// ErrNoteNotFound - note not found in DB
var ErrNoteNotFound = errors.New("note not found")

const (
	defaultLimit = 50
	maxLimit     = 100
	maxOffset    = 10_000
)

// Direction constants for ListSide
const (
	DirectionBefore = "before"
	DirectionAfter  = "after"
)

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
	// Normalize order field to lowercase for case-insensitive handling
	if req.Order != "" {
		req.Order = strings.ToLower(req.Order)
	}

	// Normalize sort field to lowercase for case-insensitive handling
	if req.Sort != "" {
		req.Sort = strings.ToLower(req.Sort)
	}

	// Validate limit - return error instead of silently clipping
	if req.Limit > maxLimit {
		return ErrInvalidLimit
	}

	// Validate span - return error instead of silently clipping
	if req.Span > maxLimit {
		return ErrInvalidLimit
	}

	// Validate offset range
	if req.Offset != nil && (*req.Offset < 0 || *req.Offset > maxOffset) {
		s.log.Warn("offset out of range", "offset", *req.Offset)
		return ErrBadRequest
	}

	// Validate that offset cannot be used with cursor or anchor
	if req.Offset != nil && (req.Cursor != "" || req.Anchor != "") {
		s.log.Warn("offset cannot be used with cursor or anchor", "offset", *req.Offset, "cursor", req.Cursor, "anchor", req.Anchor)
		return ErrBadRequest
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

// setListRequestDefaults sets default values for list request parameters
func (s *Service) setListRequestDefaults(req *ListNotesRequest) {
	// Set default limit if not provided
	if req.Limit == 0 {
		req.Limit = defaultLimit
	}
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

// reverse reverses a slice of notes in place and returns it for convenience.
// Note: This function mutates the input slice.
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

	s.setListRequestDefaults(&req)

	// If offset is provided (not nil) and no cursor/anchor, use offset-based pagination
	if req.Offset != nil && req.Cursor == "" && req.Anchor == "" {
		return s.offsetList(ctx, userID, req)
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

	notes, totalCount, totalCountUnfiltered, err := s.repo.List(ctx, userID, fetchReq, -1)
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

// offsetList implements offset-based pagination
func (s *Service) offsetList(ctx context.Context, userID bson.ObjectID, req ListNotesRequest) (*ListNotesResponse, error) {
	offset := *req.Offset // safe: caller ensured non-nil

	// Get total counts first for validation and total_pages calculation
	totalCount, totalCountUnfiltered, err := s.repo.GetCounts(ctx, userID, req)
	if err != nil {
		s.log.Error(ErrListNotes.Error(), "error", err, "user_id", userID.Hex())
		return nil, ErrListNotes
	}

	// If the query returns zero rows, just reply with an empty page.
	if totalCount == 0 {
		return &ListNotesResponse{
			Notes:                []*Note{},
			HasMore:              false,
			TotalCount:           0,
			TotalCountUnfiltered: totalCountUnfiltered,
			WindowSize:           0,
			Offset:               0,
			TotalPages:           0,
		}, nil
	}

	// Return 416 if offset is beyond total count
	if int64(offset) >= totalCount {
		s.log.Info("offset beyond total count", "offset", offset, "total_count", totalCount)
		return nil, ErrOffsetBeyondTotal
	}

	// Optimize window overflow: avoid over-asking when near the end
	remaining := totalCount - int64(offset)
	fetchLimit := req.Limit + 1
	if remaining < int64(fetchLimit) {
		fetchLimit = int(remaining)
	}

	fetchReq := req
	fetchReq.Limit = fetchLimit

	// make Offset pointer independent of the caller's struct (long‑lived copy)
	offsetCopy := new(int)
	*offsetCopy = offset
	fetchReq.Offset = offsetCopy

	notes, _, _, err := s.repo.List(ctx, userID, fetchReq, offset)
	if err != nil {
		s.log.Error(ErrListNotes.Error(), "error", err, "user_id", userID.Hex())
		return nil, ErrListNotes
	}

	// Determine if there are more results and trim to requested limit
	hasMore := len(notes) > req.Limit
	if hasMore {
		notes = notes[:req.Limit]
	}

	totalPages := int((totalCount + int64(req.Limit) - 1) / int64(req.Limit)) // Ceiling division

	response := &ListNotesResponse{
		Notes:                notes,
		NextCursor:           "", // Empty cursors for offset mode
		PrevCursor:           "",
		HasMore:              hasMore,
		TotalCount:           totalCount,
		TotalCountUnfiltered: totalCountUnfiltered,
		WindowSize:           len(notes),
		Offset:               offset,
		TotalPages:           totalPages,
	}

	return response, nil
}

// anchorList implements the new anchor-based pagination
func (s *Service) anchorList(ctx context.Context, userID bson.ObjectID, req ListNotesRequest) (*ListNotesResponse, error) {
	winSz := s.calculateWindowSize(req)

	anchor, err := s.loadAndValidateAnchor(ctx, userID, req)
	if err != nil {
		return nil, err
	}

	beforeResult, afterResult, err := s.fetchSideNotes(ctx, userID, req, anchor, winSz)
	if err != nil {
		return nil, err
	}

	return s.buildAnchorResponse(ctx, userID, req, anchor, beforeResult, afterResult)
}

// calculateWindowSize determines the window size for anchor pagination
func (s *Service) calculateWindowSize(req ListNotesRequest) int {
	if req.Span > 0 && req.Span <= maxLimit {
		return req.Span
	}

	if req.Limit > 0 {
		return req.Limit
	}

	return defaultLimit
}

// loadAndValidateAnchor loads the anchor note and validates it matches filters
func (s *Service) loadAndValidateAnchor(ctx context.Context, userID bson.ObjectID, req ListNotesRequest) (*Note, error) {
	anchor, err := s.repo.FindOne(ctx, userID, req, req.Anchor)
	if err != nil {
		s.log.Info("anchor note not found or filtered out", "user_id", userID.Hex(), "anchor_id", req.Anchor)
		return nil, ErrNoteNotFound
	}
	return anchor, nil
}

// sideResult holds the results from fetching notes on one side of the anchor
type sideResult struct {
	notes []*Note
	full  bool
}

// fetchSideNotes fetches notes before and after the anchor in parallel
func (s *Service) fetchSideNotes(ctx context.Context, userID bson.ObjectID, req ListNotesRequest, anchor *Note, winSz int) (sideResult, sideResult, error) {
	beforeN := winSz / 2
	afterN := winSz - beforeN - 1 // 1 slot for the anchor itself

	var beforeResult, afterResult sideResult
	g, gCtx := errgroup.WithContext(ctx)

	if beforeN > 0 {
		g.Go(func() error {
			// pass gCtx so the Mongo query is aborted if the *other*
			// goroutine returns an error first
			return s.fetchSideNotesWorker(
				gCtx, userID, req, anchor,
				beforeN, DirectionBefore, &beforeResult)
		})
	}

	if afterN > 0 {
		g.Go(func() error {
			return s.fetchSideNotesWorker(
				gCtx, userID, req, anchor,
				afterN, DirectionAfter, &afterResult)
		})
	}

	if err := g.Wait(); err != nil {
		s.log.Error("failed to get side notes", "error", err, "user_id", userID.Hex())
		return sideResult{}, sideResult{}, ErrListNotes
	}

	// Ensure zero‑value slices are non‑nil for downstream logic
	s.ensureNonNilSlices(&beforeResult, &afterResult)

	return beforeResult, afterResult, nil
}

// fetchSideNotesWorker fetches notes for one side of the anchor
func (s *Service) fetchSideNotesWorker(ctx context.Context, userID bson.ObjectID, req ListNotesRequest, anchor *Note, count int, direction string, result *sideResult) error {
	notes, full, err := s.repo.ListSide(ctx, userID, req, anchor, count, direction)
	if err != nil {
		// Returning the error cancels gCtx --> aborts the sibling query.
		return err
	}
	result.notes, result.full = notes, full
	return nil
}

// ensureNonNilSlices ensures that note slices are non-nil for downstream logic
func (s *Service) ensureNonNilSlices(beforeResult, afterResult *sideResult) {
	if beforeResult.notes == nil {
		beforeResult.notes = []*Note{}
	}
	if afterResult.notes == nil {
		afterResult.notes = []*Note{}
	}
}

// buildAnchorResponse constructs the final response for anchor-based pagination
func (s *Service) buildAnchorResponse(ctx context.Context, userID bson.ObjectID, req ListNotesRequest, anchor *Note, beforeResult, afterResult sideResult) (*ListNotesResponse, error) {
	anchorIndex := s.getAnchorIndex(ctx, userID, req, anchor)

	beforeN := s.calculateWindowSize(req) / 2

	reversedBefore := s.reverse(slices.Clone(beforeResult.notes))
	notes := s.combineNotes(reversedBefore, anchor, afterResult.notes)

	totalCount, totalCountUnfiltered := s.getTotalCounts(ctx, userID, req)

	// when beforeN > 0 the 'full' flag already captures "more before"
	hasMoreBefore := beforeN == 0 && anchorIndex > 0
	response := &ListNotesResponse{
		Notes:                notes,
		HasMore:              hasMoreBefore || beforeResult.full || afterResult.full,
		TotalCount:           totalCount,
		TotalCountUnfiltered: totalCountUnfiltered,
		WindowSize:           len(notes),
	}

	s.setCursors(response, beforeResult, afterResult, req.Sort)
	s.setAnchorIndex(response, anchorIndex)

	return response, nil
}

// getAnchorIndex gets the absolute position index of the anchor note
func (s *Service) getAnchorIndex(ctx context.Context, userID bson.ObjectID, req ListNotesRequest, anchor *Note) int64 {
	anchorIndex, err := s.repo.GetAnchorIndex(ctx, userID, req, anchor)
	if err != nil {
		s.log.Error("failed to get anchor index", "error", err, "user_id", userID.Hex())
		return -1
	}
	return anchorIndex
}

// combineNotes combines before notes, anchor, and after notes in the correct order
func (s *Service) combineNotes(reversedBefore []*Note, anchor *Note, afterNotes []*Note) []*Note {
	notes := append(reversedBefore, anchor)
	return append(notes, afterNotes...)
}

// getTotalCounts retrieves total counts, returning 0 values on error
func (s *Service) getTotalCounts(ctx context.Context, userID bson.ObjectID, req ListNotesRequest) (int64, int64) {
	totalCount, totalCountUnfiltered, err := s.repo.GetCounts(ctx, userID, req)
	if err != nil {
		s.log.Error("failed to get counts", "error", err, "user_id", userID.Hex())
		return 0, 0
	}
	return totalCount, totalCountUnfiltered
}

// setCursors fills Prev/Next on an AnchorResponse.
func (s *Service) setCursors(
	response *ListNotesResponse,
	beforeResult sideResult, // original order: newest--> oldest
	afterResult sideResult,
	sort string,
) {
	// NEXT
	if len(afterResult.notes) > 0 {
		response.NextCursor = s.generateNextCursor(afterResult.notes, sort)
	}

	// PREV
	if len(beforeResult.notes) > 0 {
		newest := beforeResult.notes[0] // beforeResult is newest-->oldest
		response.PrevCursor = s.generatePrevCursor([]*Note{newest}, sort)
	}
}

// setAnchorIndex sets the anchor index in the response if valid
func (s *Service) setAnchorIndex(response *ListNotesResponse, anchorIndex int64) {
	if anchorIndex >= 0 {
		response.AnchorIndex = anchorIndex + 1 // 1-based indexing
	}
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
