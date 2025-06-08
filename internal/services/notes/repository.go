package notes

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Repository defines the interface for notes repository operations
type Repository interface {
	Create(ctx context.Context, n *Note) error
	List(ctx context.Context, userID bson.ObjectID, filter ListNotesRequest) ([]*Note, int64, int64, error)
	Update(ctx context.Context, userID, noteID bson.ObjectID, patch UpdateNote) (*Note, error)
	Delete(ctx context.Context, userID, noteID bson.ObjectID) error

	// New methods for anchor-based pagination
	FindOne(ctx context.Context, userID bson.ObjectID, req ListNotesRequest, anchor string) (*Note, error)
	ListSide(ctx context.Context, userID bson.ObjectID, req ListNotesRequest, anchor *Note, limit int, direction string) ([]*Note, error)
	GetAnchorIndex(ctx context.Context, userID bson.ObjectID, req ListNotesRequest, anchor *Note) (int64, error)
	GetCounts(ctx context.Context, userID bson.ObjectID, req ListNotesRequest) (int64, int64, error)
}

// Bus defines the interface for event broadcasting
type Bus interface {
	Broadcast(ctx context.Context, ev NoteEvent)
}
