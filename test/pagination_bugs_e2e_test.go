//go:build e2e

package test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const window = 5 // keep pages small so the failure is obvious
const limitParam = "limit="
const cursorParam = "&cursor="

func TestPrevCursorDoesNotSkipWindowObjectID(t *testing.T) {
	env := SetupTestEnvironment(t)

	// create a user + a bunch of notes
	token := setupTestUser(t, env, "prevcursor-obj@example.com", "Password123")
	h := getAuthHeaders(t, token)
	createNotesForPagination(t, env, h, 2*window+window/2) // 1.5 windows extra

	// Page 1 (newest notes)
	page1 := listNotes(t, env, h, "?"+limitParam+itoa(window))
	p1IDs := extractIDs(page1["notes"].([]any))
	next := page1["next_cursor"].(string)
	require.NotEmpty(t, next, "page1 should provide next_cursor")

	// Page 2 (older notes)
	page2 := listNotes(t, env, h, "?"+limitParam+itoa(window)+cursorParam+next)
	p2Prev := reqStr(page2, "prev_cursor") // must exist so we can go back

	// Page 3 (go *backwards* using prev_cursor)
	pageBack := listNotes(t, env, h, "?"+limitParam+itoa(window)+cursorParam+p2Prev)
	backIDs := extractIDs(pageBack["notes"].([]any))

	// ✔ expectation: back-page must equal the original first page
	assert.Equal(t, p1IDs, backIDs,
		"paging back with prev_cursor must return the same notes without gaps/duplicates")
}

// same test but with a *composite* cursor (title sorting)
func TestPrevCursorDoesNotSkipWindowComposite(t *testing.T) {
	env := SetupTestEnvironment(t)

	// deterministic titles so alphabetical order differs from creation order
	titles := []string{"A-note", "B-note", "C-note", "D-note", "E-note",
		"F-note", "G-note", "H-note", "I-note", "J-note", "K-note"}
	token := setupTestUser(t, env, "prevcursor-comp@example.com", "Password123")
	h := getAuthHeaders(t, token)

	for _, ttl := range titles {
		createAndVerifyNote(t, env, h, NoteParams{Title: ttl, Body: ttl + " body", Color: testColor})
	}

	q := "?sort=title&order=asc&" + limitParam + itoa(window)
	page1 := listNotes(t, env, h, q)
	p1IDs := extractIDs(page1["notes"].([]any))
	next := reqStr(page1, "next_cursor")

	page2 := listNotes(t, env, h, q+cursorParam+next)
	prev := reqStr(page2, "prev_cursor")

	pageBack := listNotes(t, env, h, q+cursorParam+prev)
	backIDs := extractIDs(pageBack["notes"].([]any))

	assert.Equal(t, p1IDs, backIDs,
		"[title asc] paging back with prev_cursor must return identical window")
}

// The public API exposes anchor-based pagination through `anchor={id}&span={n}`.
// We pick the *middle* note as anchor so that there *are* older ones, and we
// expect `has_more == true`.  Until the bug is fixed this assertion fails.
func TestSpanOneHasMoreTrueWhenEarlierNotesExist(t *testing.T) {
	env := SetupTestEnvironment(t)

	token := setupTestUser(t, env, "spanone@example.com", "Password123")
	h := getAuthHeaders(t, token)
	createNotesForPagination(t, env, h, 10) // plenty of earlier notes

	// grab the 5th-newest note to use as the anchor
	firstPage := listNotes(t, env, h, "?"+limitParam+"5")
	anchorNote := firstPage["notes"].([]any)[4].(map[string]any) // index 4 == 5th newest
	anchorID := anchorNote["id"].(string)

	// request a *single* note around the anchor
	url := env.BaseURL + notesPath + "?anchor=" + anchorID + "&span=1"
	resp := makeHTTPRequest(t, "GET", url, nil, h, http.StatusOK)

	assert.True(t, resp["has_more"].(bool),
		"when span==1 and earlier notes exist, has_more must be true")
	assert.GreaterOrEqual(t, resp["total_count"].(float64), float64(1))
}

// -----------------------------------------------------------------------------
// small helpers
// -----------------------------------------------------------------------------

// listNotes is a tiny wrapper that keeps the calling site readable
func listNotes(t *testing.T, env *TestEnvironment, h map[string]string, query string) map[string]any {
	return makeHTTPRequest(t, "GET", env.BaseURL+notesPath+query, nil, h, http.StatusOK)
}

func extractIDs(notes []any) []string {
	out := make([]string, len(notes))
	for i, n := range notes {
		out[i] = n.(map[string]any)["id"].(string)
	}
	return out
}

func reqStr(m map[string]any, field string) string {
	if v, ok := m[field].(string); ok {
		return v
	}
	return ""
}

func itoa(i int) string { return fmt.Sprintf("%d", i) }
