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

// TestOffsetToCursorTransition tests that switching from offset mode to cursor mode works seamlessly
func TestOffsetToCursorTransition(t *testing.T) {
	env := SetupTestEnvironment(t)

	token := setupTestUser(t, env, "offset-cursor@example.com", "Password123")
	h := getAuthHeaders(t, token)

	// Create enough notes to test offset=300&limit=50
	createNotesForPagination(t, env, h, 400) // creates 400 notes

	// Request with offset=300&limit=50
	offsetURL := env.BaseURL + notesPath + "?offset=300&limit=50"
	offsetResp := makeHTTPRequest(t, "GET", offsetURL, nil, h, http.StatusOK)

	assert.Equal(t, "", reqStr(offsetResp, "next_cursor"), "offset mode should have empty next_cursor")
	assert.Equal(t, "", reqStr(offsetResp, "prev_cursor"), "offset mode should have empty prev_cursor")
	assert.Equal(t, float64(300), offsetResp["offset"].(float64), "offset should be set correctly")
	assert.Equal(t, float64(50), offsetResp["window_size"].(float64), "window_size should match returned notes")

	offsetNotes := offsetResp["notes"].([]any)
	require.Len(t, offsetNotes, 50, "should return exactly 50 notes")

	// Get the last note from offset results to use as cursor anchor
	lastOffsetNote := offsetNotes[len(offsetNotes)-1].(map[string]any)
	lastOffsetID := lastOffsetNote["id"].(string)

	// Continue pagination using cursor mode with the last note's ID
	cursorURL := env.BaseURL + notesPath + "?cursor=" + lastOffsetID + "&limit=50"
	cursorResp := makeHTTPRequest(t, "GET", cursorURL, nil, h, http.StatusOK)

	// Verify cursor response structure
	nextCursor := reqStr(cursorResp, "next_cursor")
	prevCursor := reqStr(cursorResp, "prev_cursor")
	// Note: cursors may be empty if we're at the boundaries of the data
	t.Logf("Cursor mode - next_cursor: %q, prev_cursor: %q", nextCursor, prevCursor)

	// Cursor mode should not have offset field (it should be 0 or not present)
	if offsetVal, exists := cursorResp["offset"]; exists {
		assert.Equal(t, float64(0), offsetVal.(float64), "cursor mode should have zero offset")
	}

	cursorNotes := cursorResp["notes"].([]any)
	require.True(t, len(cursorNotes) > 0, "cursor continuation should return more notes")

	// Verify no gaps: first note from cursor should be different from last offset note
	firstCursorNote := cursorNotes[0].(map[string]any)
	firstCursorID := firstCursorNote["id"].(string)

	assert.NotEqual(t, lastOffsetID, firstCursorID, "cursor continuation should not duplicate the last offset note")

	// Verify ordering is maintained (assuming default desc order by created_at)
	lastOffsetCreatedAt := lastOffsetNote["created_at"].(string)
	firstCursorCreatedAt := firstCursorNote["created_at"].(string)

	// In descending order, cursor notes should be older (smaller timestamps) than offset notes
	assert.True(t, firstCursorCreatedAt <= lastOffsetCreatedAt,
		"cursor continuation should maintain chronological ordering")
}
