package notes

import (
	"encoding/base64"
	"encoding/json"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// CompositeCursor represents a cursor for title-based pagination
type CompositeCursor struct {
	Title string        `json:"title"`
	ID    bson.ObjectID `json:"id"`
}

// EncodeCompositeCursor encodes a composite cursor to a URL-safe base64 string
func EncodeCompositeCursor(title string, id bson.ObjectID) string {
	cursor := CompositeCursor{Title: title, ID: id}
	b, _ := json.Marshal(&cursor)
	return base64.URLEncoding.EncodeToString(b)
}

// DecodeCompositeCursor decodes a URL-safe base64 string to a composite cursor
func DecodeCompositeCursor(encoded string) (*CompositeCursor, error) {
	decoded, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	var cursor CompositeCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil {
		return nil, err
	}

	return &cursor, nil
}
