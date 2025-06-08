package notes

import "errors"

// ErrCreateNote is returned when note creation fails.
var ErrCreateNote = errors.New("failed to create note")

// ErrUpdateNote is returned when note update fails.
var ErrUpdateNote = errors.New("failed to update note")

// ErrDeleteNote is returned when note deletion fails.
var ErrDeleteNote = errors.New("failed to delete note")

// ErrCreateNotesRepo is returned when notes repository creation fails.
var ErrCreateNotesRepo = errors.New("failed to create notes repository")

// ErrListNotes is returned when notes listing fails.
var ErrListNotes = errors.New("failed to list notes")

// ErrInvalidCursor is returned when cursor is invalid.
var ErrInvalidCursor = errors.New("invalid cursor")

// ErrBadRequest is returned when request parameters are invalid.
var ErrBadRequest = errors.New("bad request")

// ErrInvalidLimit is returned when limit is invalid.
var ErrInvalidLimit = errors.New("invalid limit")
