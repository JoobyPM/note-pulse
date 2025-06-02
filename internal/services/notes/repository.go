package notes

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// NotesRepo defines the interface for notes repository operations
type NotesRepo interface {
	Create(ctx context.Context, n *Note) error
	List(ctx context.Context, userID bson.ObjectID, after bson.ObjectID, limit int) ([]*Note, error)
	Update(ctx context.Context, userID, noteID bson.ObjectID, patch UpdateNote) (*Note, error)
	Delete(ctx context.Context, userID, noteID bson.ObjectID) error
}

// Bus defines the interface for event broadcasting
type Bus interface {
	Broadcast(ctx context.Context, ev NoteEvent)
}
