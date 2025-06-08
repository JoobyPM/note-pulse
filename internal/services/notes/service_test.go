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
	ErrRepositoryMsg = "repository error"
	ErrDBMsg         = "db error"
	UpdateNoteMsg    = "notes.UpdateNote"
)

// MockNotesRepo is a mock implementation of Repository
type MockNotesRepo struct {
	mock.Mock
}

func (m *MockNotesRepo) Create(ctx context.Context, note *Note) error {
	args := m.Called(ctx, note)
	return args.Error(0)
}

func (m *MockNotesRepo) List(ctx context.Context, userID bson.ObjectID, after bson.ObjectID, limit int) ([]*Note, error) {
	args := m.Called(ctx, userID, after, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Note), args.Error(1)
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
				Color: "#FF0000",
			},
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				repo.On("Create", mock.Anything, mock.AnythingOfType("*notes.Note")).Return(nil)
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
				repo.On("Create", mock.Anything, mock.AnythingOfType("*notes.Note")).Return(errors.New(ErrDBMsg))
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
				repo.On("List", mock.Anything, userID, bson.ObjectID{}, 50).Return(mockNotes, nil)
			},
			wantErr: false,
			check: func(t *testing.T, resp *ListNotesResponse) {
				assert.Len(t, resp.Notes, 2)
				assert.Empty(t, resp.NextCursor) // Less than limit, no next cursor
			},
		},
		{
			name: "successful list with custom limit",
			req: ListNotesRequest{
				Limit: 25,
			},
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				repo.On("List", mock.Anything, userID, bson.ObjectID{}, 25).Return(mockNotes[:1], nil)
			},
			wantErr: false,
			check: func(t *testing.T, resp *ListNotesResponse) {
				assert.Len(t, resp.Notes, 1)
				assert.Empty(t, resp.NextCursor)
			},
		},
		{
			name: "successful list with cursor and next page",
			req: ListNotesRequest{
				Limit:  2,
				Cursor: noteID1.Hex(),
			},
			setup: func(repo *MockNotesRepo, bus *MockBus) {
				repo.On("List", mock.Anything, userID, noteID1, 2).Return(mockNotes, nil)
			},
			wantErr: false,
			check: func(t *testing.T, resp *ListNotesResponse) {
				assert.Len(t, resp.Notes, 2)
				assert.Equal(t, noteID2.Hex(), resp.NextCursor) // Full page, has next cursor
			},
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
				repo.On("List", mock.Anything, userID, bson.ObjectID{}, 50).Return(nil, errors.New(ErrDBMsg))
			},
			wantErr: true,
			errMsg:  ErrListNotes.Error(),
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
