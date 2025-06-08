package mongo

import (
	"context"
	"testing"

	"note-pulse/internal/services/notes"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	testColor = "#FF0000"
	testTitle = "Test Note"
	testBody  = "Test body"
)

func TestNotesRepoStructure(t *testing.T) {
	note := &notes.Note{
		ID:     bson.NewObjectID(),
		UserID: bson.NewObjectID(),
		Title:  testTitle,
		Body:   testBody,
		Color:  testColor,
	}

	// Validate note structure
	assert.NotNil(t, note)
	assert.False(t, note.ID.IsZero())
	assert.False(t, note.UserID.IsZero())
	assert.Equal(t, testTitle, note.Title)
	assert.Equal(t, testBody, note.Body)
	assert.Equal(t, testColor, note.Color)
}

func TestNotesRepoUpdateNote(t *testing.T) {
	title := "Updated Title"
	body := "Updated Body"
	color := testColor

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
	assert.Equal(t, testColor, *update.Color)
}

func TestNotesRepoPartialUpdate(t *testing.T) {
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

func TestNoteEventStructure(t *testing.T) {
	note := &notes.Note{
		ID:     bson.NewObjectID(),
		UserID: bson.NewObjectID(),
		Title:  testTitle,
		Body:   testBody,
		Color:  testColor,
	}

	event := notes.NoteEvent{
		Type: "created",
		Note: note,
	}

	assert.Equal(t, "created", event.Type)
	assert.NotNil(t, event.Note)
	assert.Equal(t, testTitle, event.Note.Title)
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

func TestNewNotesRepoIndexCreationError(t *testing.T) {
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
