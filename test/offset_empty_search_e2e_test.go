//go:build e2e

package test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOffsetZeroEmptySearchReturns200(t *testing.T) {
	env := SetupTestEnvironment(t)

	// test user
	token := setupTestUser(t, env, "emptysearch@example.com", "Password123")
	h := getAuthHeaders(t, token)

	// create one unrelated note so DB isn't empty
	createAndVerifyNote(t, env, h, NoteParams{
		Title: "Unrelated",
		Body:  "Won't match search",
		Color: testColor,
	})

	// query that yields zero results, with explicit offset=0
	url := env.BaseURL + "/api/v1/notes?offset=0&q=__no_match_expected__"
	resp := makeHTTPRequest(t, "GET", url, nil, h, http.StatusOK)

	// assertions
	notes := resp["notes"].([]any)
	assert.Len(t, notes, 0, "notes list should be empty")

	assert.False(t, resp["has_more"].(bool))
	assert.Equal(t, float64(0), resp["total_count"].(float64))
	assert.Equal(t, float64(1), resp["total_count_unfiltered"].(float64))
	assert.Equal(t, float64(0), resp["window_size"].(float64))

	// offset field is omitted when zero, but if present it must be 0
	if off, ok := resp["offset"]; ok {
		assert.Equal(t, float64(0), off.(float64))
	}

	// cursors must be empty
	require.Empty(t, resp["next_cursor"])
	require.Empty(t, resp["prev_cursor"])
}
