package notes

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Note represents a sticky note in the system
type Note struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id,omitempty" example:"683cdb8aa96ad71e8e075bd1"`
	UserID    bson.ObjectID `bson:"user_id" json:"user_id" example:"683cdb8aa96ad71e8e075bd0"`
	Title     string        `bson:"title" json:"title" validate:"required" example:"Meeting Notes"`
	Body      string        `bson:"body" json:"body" example:"Remember to discuss the quarterly targets"`
	Color     string        `bson:"color" json:"color" validate:"omitempty,hexcolor" example:"#FFD700"`
	CreatedAt time.Time     `bson:"created_at" json:"created_at" example:"2025-06-01T23:00:26.005703677Z"`
	UpdatedAt time.Time     `bson:"updated_at" json:"updated_at" example:"2025-06-01T23:00:26.005703677Z"`
}

// UpdateNote represents the fields that can be updated in a note
type UpdateNote struct {
	Title *string `json:"title,omitempty" validate:"omitempty,min=1" example:"Updated Meeting Notes"`
	Body  *string `json:"body,omitempty" example:"Updated content for the meeting"`
	Color *string `json:"color,omitempty" validate:"omitempty,hexcolor" example:"#FF6B6B"`
}

// NoteEvent represents an event that occurred on a note
type NoteEvent struct {
	Type string `json:"type"` // "created", "updated", "deleted"
	Note *Note  `json:"note"`
}

// DeletedNoteData represents the minimal data for a deleted note event
type DeletedNoteData struct {
	ID string `json:"id"`
}
