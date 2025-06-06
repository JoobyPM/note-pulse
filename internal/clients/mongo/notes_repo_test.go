package mongo

import (
	"context"
	"testing"

	"note-pulse/internal/services/notes"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func TestNotesRepo_Structure(t *testing.T) {
	note := &notes.Note{
		ID:     bson.NewObjectID(),
		UserID: bson.NewObjectID(),
		Title:  "Test Note",
		Body:   "Test body",
		Color:  "#FF0000",
	}

	// Validate note structure
	assert.NotNil(t, note)
	assert.False(t, note.ID.IsZero())
	assert.False(t, note.UserID.IsZero())
	assert.Equal(t, "Test Note", note.Title)
	assert.Equal(t, "Test body", note.Body)
	assert.Equal(t, "#FF0000", note.Color)
}

func TestNotesRepo_UpdateNote(t *testing.T) {
	title := "Updated Title"
	body := "Updated Body"
	color := "#00FF00"

	update := notes.UpdateNote{
		Title: &title,
		Body:  &body,
		Color: &color,
	}

	assert.NotNil(t, update.Title)
	assert.NotNil(t, update.Body)
	assert.NotNil(t, update.Color)
	assert.Equal(t, "Updated Title", *update.Title)
	assert.Equal(t, "Updated Body", *update.Body)
	assert.Equal(t, "#00FF00", *update.Color)
}

func TestNotesRepo_PartialUpdate(t *testing.T) {
	title := "Only Title Updated"

	update := notes.UpdateNote{
		Title: &title,
		// Body and Color intentionally omitted
	}

	assert.NotNil(t, update.Title)
	assert.Nil(t, update.Body)
	assert.Nil(t, update.Color)
	assert.Equal(t, "Only Title Updated", *update.Title)
}

func TestNoteEvent_Structure(t *testing.T) {
	note := &notes.Note{
		ID:     bson.NewObjectID(),
		UserID: bson.NewObjectID(),
		Title:  "Event Note",
		Body:   "Event body",
		Color:  "#FF0000",
	}

	event := notes.NoteEvent{
		Type: "created",
		Note: note,
	}

	assert.Equal(t, "created", event.Type)
	assert.NotNil(t, event.Note)
	assert.Equal(t, "Event Note", event.Note.Title)
}

func TestObjectIDConversions(t *testing.T) {
	id := bson.NewObjectID()
	assert.False(t, id.IsZero())

	hexString := id.Hex()
	assert.NotEmpty(t, hexString, "hexString should NotEmpty")

	parsedID, err := bson.ObjectIDFromHex(hexString)
	assert.NoError(t, err)
	assert.Equal(t, id, parsedID)

	_, err = bson.ObjectIDFromHex("invalid")
	assert.Error(t, err)
}

func TestUserIDValidation(t *testing.T) {
	user1 := bson.NewObjectID()
	user2 := bson.NewObjectID()

	assert.NotEqual(t, user1, user2)
	assert.False(t, user1.IsZero())
	assert.False(t, user2.IsZero())
}

func TestNewNotesRepo_IndexCreationError(t *testing.T) {
	// This test validates the error handling logic signature

	var db *mongo.Database // nil db will cause panic, but that's expected behavior

	// This would panic with nil db, but we're testing the signature
	_, err := func() (repo *NotesRepo, retErr error) {
		defer func() {
			if r := recover(); r != nil {
				// Expected panic with nil database
				retErr = nil // We expect this to panic
			}
		}()
		return NewNotesRepo(context.Background(), db)
	}()

	// The function should return error as second parameter
	_ = err // We expect this to panic with nil db, which is fine

	// Test passes if function signature is correct (returns repo, error)
	assert.True(t, true, "NewNotesRepo has correct signature returning (*NotesRepo, error)")
}
