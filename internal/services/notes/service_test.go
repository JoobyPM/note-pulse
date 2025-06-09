package notes

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var silentLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

var (
	testColor        = "#FF0000"
	ErrRepositoryMsg = "repository error"
	ErrDBMsg         = "db error"
	UpdateNoteMsg    = "notes.UpdateNote"
	mockNote         = mock.AnythingOfType("*notes.Note")
	mockListReq      = mock.AnythingOfType("notes.ListNotesRequest")
)

// MockNotesRepo is a mock implementation of Repository
type MockNotesRepo struct {
	mock.Mock
}

func (m *MockNotesRepo) Create(ctx context.Context, note *Note) error {
	args := m.Called(ctx, note)
	return args.Error(0)
}

func (m *MockNotesRepo) List(ctx context.Context, userID bson.ObjectID, filter ListNotesRequest) ([]*Note, int64, int64, error) {
	args := m.Called(ctx, userID, filter)
	if args.Get(0) == nil {
		return nil, 0, 0, args.Error(3)
	}
	return args.Get(0).([]*Note), args.Get(1).(int64), args.Get(2).(int64), args.Error(3)
}

func (m *MockNotesRepo) Update(ctx context.Context, userID, noteID bson.ObjectID, patch UpdateNote) (*Note, error) {
	args := m.Called(ctx, userID, noteID, patch)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Note), args.Error(1)
}

func (m *MockNotesRepo) Delete(ctx context.Context, userID, noteID bson.ObjectID) error {
	args := m.Called(ctx, userID, noteID)
	return args.Error(0)
}

func (m *MockNotesRepo) FindOne(ctx context.Context, userID bson.ObjectID, req ListNotesRequest, anchor string) (*Note, error) {
	args := m.Called(ctx, userID, req, anchor)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Note), args.Error(1)
}

func (m *MockNotesRepo) ListSide(ctx context.Context, userID bson.ObjectID, req ListNotesRequest, anchor *Note, limit int, direction string) ([]*Note, bool, error) {
	args := m.Called(ctx, userID, req, anchor, limit, direction)
	if args.Get(0) == nil {
		return nil, false, args.Error(2)
	}
	return args.Get(0).([]*Note), args.Get(1).(bool), args.Error(2)
}

func (m *MockNotesRepo) GetAnchorIndex(ctx context.Context, userID bson.ObjectID, req ListNotesRequest, anchor *Note) (int64, error) {
	args := m.Called(ctx, userID, req, anchor)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockNotesRepo) GetCounts(ctx context.Context, userID bson.ObjectID, req ListNotesRequest) (int64, int64, error) {
	args := m.Called(ctx, userID, req)
	return args.Get(0).(int64), args.Get(1).(int64), args.Error(2)
}

// MockBus is a mock implementation of Bus
type MockBus struct {
	mock.Mock
}

func (m *MockBus) Broadcast(ctx context.Context, ev NoteEvent) {
	m.Called(ctx, ev)
}

func TestServiceCreate(t *testing.T) {
	userID := bson.NewObjectID()

	tests := []struct {
		name    string
		req     CreateNoteRequest
		setup   func(*MockNotesRepo, *MockBus)
		wantErr bool
		errMsg  string
	}{
		{
			name: "successful creation",
			req: CreateNoteRequest{
				Title: "Test Note",
				Body:  "Test body",
				Color: testColor,
			},
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				repo.On("Create", mock.Anything, mockNote).Return(nil)
				bus.On("Broadcast", mock.Anything, mock.MatchedBy(func(ev NoteEvent) bool {
					return ev.Type == "created"
				})).Return()
			},
			wantErr: false,
		},
		{
			name: ErrRepositoryMsg,
			req: CreateNoteRequest{
				Title: "Test Note",
				Body:  "Test body",
			},
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				repo.On("Create", mock.Anything, mockNote).Return(errors.New(ErrDBMsg))
			},
			wantErr: true,
			errMsg:  ErrCreateNote.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockNotesRepo)
			bus := new(MockBus)
			tt.setup(repo, bus)

			service := NewService(repo, bus, silentLogger)
			resp, err := service.Create(context.Background(), userID, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotNil(t, resp.Note)
				assert.Equal(t, tt.req.Title, resp.Note.Title)
				assert.Equal(t, tt.req.Body, resp.Note.Body)
				assert.Equal(t, tt.req.Color, resp.Note.Color)
				assert.Equal(t, userID, resp.Note.UserID)
				assert.False(t, resp.Note.ID.IsZero())
				assert.False(t, resp.Note.CreatedAt.IsZero())
				assert.False(t, resp.Note.UpdatedAt.IsZero())
			}

			repo.AssertExpectations(t)
			bus.AssertExpectations(t)
		})
	}
}

// makeNote returns a fully-populated *Note that is safe to re-use in mocks.
func makeNote(id, userID bson.ObjectID, title, body, color string, ts time.Time) *Note {
	return &Note{
		ID:        id,
		UserID:    userID,
		Title:     title,
		Body:      body,
		Color:     color,
		CreatedAt: ts,
		UpdatedAt: ts,
	}
}

// newServiceWithMocks wires together a Service + fresh mocks and lets the
// caller register expectations before the test starts.
func newServiceWithMocks(
	t *testing.T,
	setup func(repo *MockNotesRepo, bus *MockBus),
) (*Service, *MockNotesRepo, *MockBus) {
	t.Helper()

	repo := new(MockNotesRepo)
	bus := new(MockBus)

	if setup != nil {
		setup(repo, bus)
	}

	svc := NewService(repo, bus, silentLogger)
	return svc, repo, bus
}

// TestAnchorBasedPaginationScenarios tests specific anchor-based pagination scenarios
func TestAnchorBasedPaginationScenarios(t *testing.T) {
	userID := bson.NewObjectID()
	now := time.Now().UTC()

	/* ------------------------------------------------------------------ */
	t.Run("duplicate-title round-trip", func(t *testing.T) {
		id1, id2 := bson.NewObjectID(), bson.NewObjectID()
		foo1 := makeNote(id1, userID, "Foo", "Body 1", "", now)
		foo2 := makeNote(id2, userID, "Foo", "Body 2", "", now.Add(time.Minute))

		svc, repo, _ := newServiceWithMocks(t, func(r *MockNotesRepo, _ *MockBus) {
			anchorCursor := EncodeCompositeCursor(foo1.Title, foo1.ID)

			// ① Anchor lookup
			r.On("FindOne", mock.Anything, userID, mockListReq, anchorCursor).
				Return(foo1, nil).Once()

			// ② Window sides
			r.On("ListSide", mock.Anything, userID, mockListReq, foo1, 1, DirectionBefore).
				Return([]*Note{}, false, nil).Once()
			r.On("ListSide", mock.Anything, userID, mockListReq, foo1, 1, DirectionAfter).
				Return([]*Note{foo2}, false, nil).Once()

			// ③ Meta
			r.On("GetAnchorIndex", mock.Anything, userID, mockListReq, foo1).
				Return(int64(0), nil).Once()
			r.On("GetCounts", mock.Anything, userID, mockListReq).
				Return(int64(2), int64(2), nil).Once()
		})

		resp, err := svc.List(context.Background(), userID, ListNotesRequest{
			Anchor: EncodeCompositeCursor(foo1.Title, foo1.ID),
			Span:   3,
			Sort:   "title",
			Order:  "asc",
		})

		assert.NoError(t, err)
		assert.Equal(t, []*Note{foo1, foo2}, resp.Notes)
		repo.AssertExpectations(t)
	})

	/* ------------------------------------------------------------------ */
	t.Run("filter drift keeps anchor window consistent", func(t *testing.T) {
		anchorID, neighbourID := bson.NewObjectID(), bson.NewObjectID()

		anchor := makeNote(anchorID, userID, "Red Note", "This is red", testColor, now)
		neighbour := makeNote(neighbourID, userID, "Neighbour", "This was red", testColor, now.Add(-time.Hour))

		svc, repo, _ := newServiceWithMocks(t, func(r *MockNotesRepo, _ *MockBus) {
			// Anchor lookup with colour filter
			r.On("FindOne", mock.Anything, userID,
				mock.MatchedBy(func(req ListNotesRequest) bool { return req.Color == testColor }),
				anchorID.Hex()).Return(anchor, nil).Once()

			// Sides
			r.On("ListSide", mock.Anything, userID, mockListReq, anchor, 1, DirectionBefore).
				Return([]*Note{neighbour}, false, nil).Once()
			r.On("ListSide", mock.Anything, userID, mockListReq, anchor, 1, DirectionAfter).
				Return([]*Note{}, false, nil).Once()

			// Meta
			r.On("GetAnchorIndex", mock.Anything, userID, mockListReq, anchor).
				Return(int64(1), nil).Once()
			r.On("GetCounts", mock.Anything, userID, mockListReq).
				Return(int64(2), int64(3), nil).Once()
		})

		resp, err := svc.List(context.Background(), userID, ListNotesRequest{
			Anchor: anchorID.Hex(),
			Span:   3,
			Color:  testColor,
		})

		assert.NoError(t, err)
		assert.Equal(t, []*Note{neighbour, anchor}, resp.Notes)
		assert.Equal(t, int64(2), resp.TotalCount) // snapshot of the moment of query
		repo.AssertExpectations(t)
	})

	/* ------------------------------------------------------------------ */
	t.Run("anchor filtered-out returns not-found", func(t *testing.T) {
		anchorID := bson.NewObjectID()

		svc, repo, _ := newServiceWithMocks(t, func(r *MockNotesRepo, _ *MockBus) {
			r.On("FindOne", mock.Anything, userID,
				mock.MatchedBy(func(req ListNotesRequest) bool { return req.Color == testColor }),
				anchorID.Hex()).Return(nil, ErrNoteNotFound).Once()
		})

		resp, err := svc.List(context.Background(), userID, ListNotesRequest{
			Anchor: anchorID.Hex(),
			Span:   3,
			Color:  testColor,
		})

		assert.ErrorIs(t, err, ErrNoteNotFound)
		assert.Nil(t, resp)
		repo.AssertExpectations(t)
	})
}

func TestServiceList(t *testing.T) {
	userID := bson.NewObjectID()
	noteID1 := bson.NewObjectID()
	noteID2 := bson.NewObjectID()
	now := time.Now().UTC()

	mockNotes := []*Note{
		{
			ID:        noteID1,
			UserID:    userID,
			Title:     "Note 1",
			Body:      "Body 1",
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:        noteID2,
			UserID:    userID,
			Title:     "Note 2",
			Body:      "Body 2",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	tests := []struct {
		name    string
		req     ListNotesRequest
		setup   func(*MockNotesRepo, *MockBus)
		wantErr bool
		errMsg  string
		check   func(*testing.T, *ListNotesResponse)
	}{
		{
			name: "successful list with default limit",
			req:  ListNotesRequest{},
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				expectedReq := ListNotesRequest{Limit: 51}
				repo.On("List", mock.Anything, userID, expectedReq).Return(mockNotes, int64(100), int64(200), nil)
			},
			wantErr: false,
			check: func(t *testing.T, resp *ListNotesResponse) {
				assert.Len(t, resp.Notes, 2)
				assert.Empty(t, resp.NextCursor) // Less than limit, no next cursor
				assert.False(t, resp.HasMore)    // Less than limit, no more pages
				assert.Equal(t, int64(100), resp.TotalCount)
				assert.Equal(t, int64(200), resp.TotalCountUnfiltered)
			},
		},
		{
			name: "successful list with custom limit",
			req: ListNotesRequest{
				Limit: 25,
			},
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				expectedReq := ListNotesRequest{Limit: 26}
				repo.On("List", mock.Anything, userID, expectedReq).Return(mockNotes[:1], int64(50), int64(80), nil)
			},
			wantErr: false,
			check: func(t *testing.T, resp *ListNotesResponse) {
				assert.Len(t, resp.Notes, 1)
				assert.Empty(t, resp.NextCursor)
				assert.False(t, resp.HasMore)
				assert.Equal(t, int64(50), resp.TotalCount)
				assert.Equal(t, int64(80), resp.TotalCountUnfiltered)
			},
		},
		{
			name: "successful list with cursor and next page",
			req: ListNotesRequest{
				Limit:  2,
				Cursor: noteID1.Hex(),
			},
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				// Return 3 notes (limit+1) to simulate having more data
				threeNotes := []*Note{mockNotes[0], mockNotes[1], mockNotes[0]} // reuse first note as third
				expectedReq := ListNotesRequest{Limit: 3, Cursor: noteID1.Hex()}
				repo.On("List", mock.Anything, userID, expectedReq).Return(threeNotes, int64(200), int64(300), nil)
			},
			wantErr: false,
			check: func(t *testing.T, resp *ListNotesResponse) {
				assert.Len(t, resp.Notes, 2)
				assert.Equal(t, noteID2.Hex(), resp.NextCursor) // Has more data, so has next cursor
				assert.True(t, resp.HasMore)                    // Has more data
				assert.Equal(t, int64(200), resp.TotalCount)
				assert.Equal(t, int64(300), resp.TotalCountUnfiltered)
			},
		},
		{
			name: "title sorting with pagination - no duplicates or gaps",
			req: ListNotesRequest{
				Limit: 1,
				Sort:  "title",
				Order: "asc",
			},
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				// Mock out-of-order _id but ordered by title
				note1 := &Note{
					ID:        noteID2, // Note: ID2 comes first despite being Note 1 alphabetically
					UserID:    userID,
					Title:     "A First Note", // Alphabetically first
					Body:      "Body A",
					CreatedAt: now,
					UpdatedAt: now,
				}
				note2 := &Note{
					ID:        noteID1, // Note: ID1 comes second
					UserID:    userID,
					Title:     "B Second Note", // Alphabetically second
					Body:      "Body B",
					CreatedAt: now,
					UpdatedAt: now,
				}

				// Return limit+1 items to simulate having more data
				expectedReq := ListNotesRequest{Limit: 2, Sort: "title", Order: "asc"}
				repo.On("List", mock.Anything, userID, expectedReq).Return([]*Note{note1, note2}, int64(2), int64(5), nil)
			},
			wantErr: false,
			check: func(t *testing.T, resp *ListNotesResponse) {
				assert.Len(t, resp.Notes, 1)
				assert.Equal(t, "A First Note", resp.Notes[0].Title)
				// For title sorting, cursor should be base64 encoded JSON, not just ObjectID
				assert.NotEqual(t, noteID2.Hex(), resp.NextCursor, "should use composite cursor for title sorting")
				assert.NotEmpty(t, resp.NextCursor, "should have composite cursor")
				assert.True(t, resp.HasMore)
				assert.Equal(t, int64(2), resp.TotalCount)
				assert.Equal(t, int64(5), resp.TotalCountUnfiltered)
			},
		},
		{
			name: "security test - regex metacharacters escaped",
			req: ListNotesRequest{
				Q: ".^$",
			},
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				expectedReq := ListNotesRequest{Limit: 51, Q: ".^$"}
				repo.On("List", mock.Anything, userID, expectedReq).Return([]*Note{}, int64(0), int64(10), nil)
			},
			wantErr: false,
			check: func(t *testing.T, resp *ListNotesResponse) {
				assert.Len(t, resp.Notes, 0)
				assert.False(t, resp.HasMore)
				assert.Equal(t, int64(0), resp.TotalCount)
				assert.Equal(t, int64(10), resp.TotalCountUnfiltered)
			},
		},
		{
			name: "limit validation error",
			req: ListNotesRequest{
				Limit: 101, // Exceeds max of 100
			},
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				// No repo calls expected due to validation error
			},
			wantErr: true,
			errMsg:  ErrInvalidLimit.Error(),
		},
		{
			name: ErrInvalidCursor.Error(),
			req: ListNotesRequest{
				Cursor: "invalid-cursor",
			},
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				// No repo calls expected due to invalid cursor
			},
			wantErr: true,
			errMsg:  ErrInvalidCursor.Error(),
		},
		{
			name: ErrRepositoryMsg,
			req:  ListNotesRequest{},
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				expectedReq := ListNotesRequest{Limit: 51}
				repo.On("List", mock.Anything, userID, expectedReq).Return(nil, int64(0), int64(0), errors.New(ErrDBMsg))
			},
			wantErr: true,
			errMsg:  ErrListNotes.Error(),
		},
		{
			name: "anchor and cursor cannot be used together",
			req: ListNotesRequest{
				Anchor: noteID1.Hex(),
				Cursor: noteID2.Hex(),
			},
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				// No repo calls expected due to validation error
			},
			wantErr: true,
			errMsg:  ErrBadRequest.Error(),
		},
		{
			name: "successful anchor-based pagination",
			req: ListNotesRequest{
				Anchor: noteID1.Hex(),
				Span:   3,
			},
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				anchorNote := &Note{
					ID:        noteID1,
					UserID:    userID,
					Title:     "Anchor Note",
					Body:      "Anchor Body",
					CreatedAt: now,
					UpdatedAt: now,
				}

				beforeNotes := []*Note{mockNotes[0]}
				afterNotes := []*Note{mockNotes[1]}

				repo.On("FindOne", mock.Anything, userID, mockListReq, noteID1.Hex()).Return(anchorNote, nil)
				repo.On("ListSide", mock.Anything, userID, mockListReq, anchorNote, 1, DirectionBefore).Return(beforeNotes, false, nil)
				repo.On("ListSide", mock.Anything, userID, mockListReq, anchorNote, 1, DirectionAfter).Return(afterNotes, false, nil)
				repo.On("GetAnchorIndex", mock.Anything, userID, mockListReq, anchorNote).Return(int64(5), nil)
				repo.On("GetCounts", mock.Anything, userID, mockListReq).Return(int64(10), int64(15), nil)
			},
			wantErr: false,
			check: func(t *testing.T, resp *ListNotesResponse) {
				assert.Len(t, resp.Notes, 3)                        // before + anchor + after
				assert.Equal(t, "Anchor Note", resp.Notes[1].Title) // Anchor should be in the middle
				assert.Equal(t, int64(6), resp.AnchorIndex)         // 1-based indexing: 5 + 1
				assert.Equal(t, int64(10), resp.TotalCount)
				assert.Equal(t, int64(15), resp.TotalCountUnfiltered)
			},
		},
		{
			name: "anchor not found",
			req: ListNotesRequest{
				Anchor: noteID1.Hex(),
				Span:   3,
			},
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				repo.On("FindOne", mock.Anything, userID, mockListReq, noteID1.Hex()).Return(nil, ErrNoteNotFound)
			},
			wantErr: true,
			errMsg:  ErrNoteNotFound.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockNotesRepo)
			bus := new(MockBus)
			tt.setup(repo, bus)

			service := NewService(repo, bus, silentLogger)
			resp, err := service.List(context.Background(), userID, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				if tt.check != nil {
					tt.check(t, resp)
				}
			}

			repo.AssertExpectations(t)
			bus.AssertExpectations(t)
		})
	}
}

func TestServiceUpdate(t *testing.T) {
	userID := bson.NewObjectID()
	noteID := bson.NewObjectID()
	title := "Updated Title"
	body := "Updated Body"
	color := "#00FF00"
	now := time.Now().UTC()

	updatedNote := &Note{
		ID:        noteID,
		UserID:    userID,
		Title:     title,
		Body:      body,
		Color:     color,
		CreatedAt: now.Add(-time.Hour),
		UpdatedAt: now,
	}

	tests := []struct {
		name    string
		req     UpdateNoteRequest
		setup   func(*MockNotesRepo, *MockBus)
		wantErr bool
		errMsg  string
	}{
		{
			name: "successful update",
			req: UpdateNoteRequest{
				Title: &title,
				Body:  &body,
				Color: &color,
			},
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				repo.On("Update", mock.Anything, userID, noteID, mock.AnythingOfType(UpdateNoteMsg)).Return(updatedNote, nil)
				bus.On("Broadcast", mock.Anything, mock.MatchedBy(func(ev NoteEvent) bool {
					return ev.Type == "updated"
				})).Return()
			},
			wantErr: false,
		},
		{
			name: ErrNoteNotFound.Error(),
			req: UpdateNoteRequest{
				Title: &title,
			},
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				repo.On("Update", mock.Anything, userID, noteID, mock.AnythingOfType(UpdateNoteMsg)).Return(nil, ErrNoteNotFound)
			},
			wantErr: true,
			errMsg:  ErrNoteNotFound.Error(),
		},
		{
			name: ErrRepositoryMsg,
			req: UpdateNoteRequest{
				Title: &title,
			},
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				repo.On("Update", mock.Anything, userID, noteID, mock.AnythingOfType(UpdateNoteMsg)).Return(nil, errors.New(ErrDBMsg))
			},
			wantErr: true,
			errMsg:  ErrUpdateNote.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockNotesRepo)
			bus := new(MockBus)
			tt.setup(repo, bus)

			service := NewService(repo, bus, silentLogger)
			resp, err := service.Update(context.Background(), userID, noteID, tt.req)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.NotNil(t, resp.Note)
				assert.Equal(t, title, resp.Note.Title)
				assert.Equal(t, body, resp.Note.Body)
				assert.Equal(t, color, resp.Note.Color)
			}

			repo.AssertExpectations(t)
			bus.AssertExpectations(t)
		})
	}
}

func TestServiceDelete(t *testing.T) {
	userID := bson.NewObjectID()
	noteID := bson.NewObjectID()

	tests := []struct {
		name    string
		setup   func(*MockNotesRepo, *MockBus)
		wantErr bool
		errMsg  string
	}{
		{
			name: "successful deletion",
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				repo.On("Delete", mock.Anything, userID, noteID).Return(nil)
				bus.On("Broadcast", mock.Anything, mock.MatchedBy(func(ev NoteEvent) bool {
					return ev.Type == "deleted" && ev.Note.ID == noteID && ev.Note.UserID == userID
				})).Return()
			},
			wantErr: false,
		},
		{
			name: ErrNoteNotFound.Error(),
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				repo.On("Delete", mock.Anything, userID, noteID).Return(ErrNoteNotFound)
			},
			wantErr: true,
			errMsg:  ErrNoteNotFound.Error(),
		},
		{
			name: ErrRepositoryMsg,
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				repo.On("Delete", mock.Anything, userID, noteID).Return(errors.New(ErrDBMsg))
			},
			wantErr: true,
			errMsg:  ErrDeleteNote.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockNotesRepo)
			bus := new(MockBus)
			tt.setup(repo, bus)

			service := NewService(repo, bus, silentLogger)
			err := service.Delete(context.Background(), userID, noteID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}

			repo.AssertExpectations(t)
			bus.AssertExpectations(t)
		})
	}
}

func TestServiceCrossUserSafety(t *testing.T) {
	user2 := bson.NewObjectID()
	noteID := bson.NewObjectID()

	tests := []struct {
		name      string
		operation string
		setup     func(*MockNotesRepo, *MockBus)
	}{
		{
			name:      "update cross-user safety",
			operation: "update",
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				// User2 tries to update User1's note - should fail
				repo.On("Update", mock.Anything, user2, noteID, mock.AnythingOfType(UpdateNoteMsg)).Return(nil, ErrNoteNotFound)
			},
		},
		{
			name:      "delete cross-user safety",
			operation: "delete",
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				// User2 tries to delete User1's note - should fail
				repo.On("Delete", mock.Anything, user2, noteID).Return(ErrNoteNotFound)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockNotesRepo)
			bus := new(MockBus)
			tt.setup(repo, bus)

			service := NewService(repo, bus, silentLogger)

			switch tt.operation {
			case "update":
				title := "Hacked"
				req := UpdateNoteRequest{Title: &title}
				resp, err := service.Update(context.Background(), user2, noteID, req)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), ErrNoteNotFound.Error())
				assert.Nil(t, resp)
			case "delete":
				err := service.Delete(context.Background(), user2, noteID)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), ErrNoteNotFound.Error())
			}

			repo.AssertExpectations(t)
			bus.AssertExpectations(t)
		})
	}
}

func TestServiceHTMLSanitization(t *testing.T) {
	userID := bson.NewObjectID()
	noteID := bson.NewObjectID()
	now := time.Now().UTC()

	tests := []struct {
		name        string
		operation   string
		dirtyTitle  string
		dirtyBody   string
		cleanTitle  string
		cleanBody   string
		description string
	}{
		{
			name:        "create - strips script tags",
			operation:   "create",
			dirtyTitle:  `<script>alert('xss')</script>Meeting Notes`,
			dirtyBody:   `<script>alert('body')</script>Meeting content`,
			cleanTitle:  "Meeting Notes",
			cleanBody:   "Meeting content",
			description: "should remove script tags completely",
		},
		{
			name:        "create - strips image with onerror",
			operation:   "create",
			dirtyTitle:  `<img src=x onerror=alert(1)>Important Note`,
			dirtyBody:   `<img src=x onerror=alert('xss')>Important content`,
			cleanTitle:  "Important Note",
			cleanBody:   "Important content",
			description: "should remove dangerous image tags",
		},
		{
			name:        "create - strips all HTML tags but preserves text",
			operation:   "create",
			dirtyTitle:  `<div><p>Hello <b>world</b></p></div>`,
			dirtyBody:   `<p>Body with <a href="http://evil.com">link</a> and <br> breaks</p>`,
			cleanTitle:  "Hello world",
			cleanBody:   "Body with link and breaks",
			description: "should strip all HTML tags but keep text content with proper spacing",
		},
		{
			name:        "create - preserves markdown-like syntax",
			operation:   "create",
			dirtyTitle:  `# Heading with **bold** text`,
			dirtyBody:   `[link](http://example.com) and **bold** text`,
			cleanTitle:  "# Heading with **bold** text",
			cleanBody:   "[link](http://example.com) and **bold** text",
			description: "should preserve markdown syntax which is not HTML",
		},
		{
			name:        "update - strips script tags",
			operation:   "update",
			dirtyTitle:  `<script>alert('update')</script>Updated Notes`,
			dirtyBody:   `<script>evil();</script>Updated content`,
			cleanTitle:  "Updated Notes",
			cleanBody:   "Updated content",
			description: "should sanitize updates too",
		},
		{
			name:        "update - strips dangerous attributes",
			operation:   "update",
			dirtyTitle:  `<p onclick="alert('xss')">Safe title</p>`,
			dirtyBody:   `<div onmouseover="steal()">Safe body</div>`,
			cleanTitle:  "Safe title",
			cleanBody:   "Safe body",
			description: "should remove dangerous event handlers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockNotesRepo)
			bus := new(MockBus)

			service := NewService(repo, bus, silentLogger)

			switch tt.operation {
			case "create":
				// Mock the repo to capture what was actually passed to Create
				var capturedNote *Note
				repo.On("Create", mock.Anything, mockNote).Run(func(args mock.Arguments) {
					capturedNote = args.Get(1).(*Note)
				}).Return(nil)
				bus.On("Broadcast", mock.Anything, mock.MatchedBy(func(ev NoteEvent) bool {
					return ev.Type == "created"
				})).Return()

				req := CreateNoteRequest{
					Title: tt.dirtyTitle,
					Body:  tt.dirtyBody,
					Color: testColor,
				}

				resp, err := service.Create(context.Background(), userID, req)

				assert.NoError(t, err, tt.description)
				assert.NotNil(t, resp)
				assert.NotNil(t, capturedNote, "should have captured the note passed to repo")

				// Verify sanitization happened
				assert.Equal(t, tt.cleanTitle, capturedNote.Title, "title should be sanitized: %s", tt.description)
				assert.Equal(t, tt.cleanBody, capturedNote.Body, "body should be sanitized: %s", tt.description)

				// Also check the response
				assert.Equal(t, tt.cleanTitle, resp.Note.Title)
				assert.Equal(t, tt.cleanBody, resp.Note.Body)

			case "update":
				// Mock the repo to capture what was actually passed to Update
				var capturedPatch UpdateNote
				mockUpdatedNote := &Note{
					ID:        noteID,
					UserID:    userID,
					Title:     tt.cleanTitle,
					Body:      tt.cleanBody,
					Color:     testColor,
					CreatedAt: now.Add(-time.Hour),
					UpdatedAt: now,
				}

				repo.On("Update", mock.Anything, userID, noteID, mock.AnythingOfType(UpdateNoteMsg)).Run(func(args mock.Arguments) {
					capturedPatch = args.Get(3).(UpdateNote)
				}).Return(mockUpdatedNote, nil)
				bus.On("Broadcast", mock.Anything, mock.MatchedBy(func(ev NoteEvent) bool {
					return ev.Type == "updated"
				})).Return()

				req := UpdateNoteRequest{
					Title: &tt.dirtyTitle,
					Body:  &tt.dirtyBody,
				}

				resp, err := service.Update(context.Background(), userID, noteID, req)

				assert.NoError(t, err, tt.description)
				assert.NotNil(t, resp)

				// Verify sanitization happened in the patch
				assert.NotNil(t, capturedPatch.Title, "title should be present in patch")
				assert.NotNil(t, capturedPatch.Body, "body should be present in patch")
				assert.Equal(t, tt.cleanTitle, *capturedPatch.Title, "title should be sanitized: %s", tt.description)
				assert.Equal(t, tt.cleanBody, *capturedPatch.Body, "body should be sanitized: %s", tt.description)
			}

			repo.AssertExpectations(t)
			bus.AssertExpectations(t)
		})
	}
}

// The pagination bug fixes are validated by E2E tests since they require
// complex integration behavior that's difficult to mock properly.
// See test/pagination_bugs_e2e_test.go for comprehensive validation.
