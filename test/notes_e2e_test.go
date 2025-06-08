//go:build e2e

package test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotesE2E(t *testing.T) {
	env := SetupTestEnvironment(t)
	testEmail := "noteuser@example.com"
	testPassword := "Password123"
	authToken := setupTestUser(t, env, testEmail, testPassword)
	headers := map[string]string{"Authorization": "Bearer " + authToken}

	var noteAID string

	t.Run("create_note_A", func(t *testing.T) {
		noteAID = createAndVerifyNote(t, env, headers, "A", "Note A content", "#FF0000")
	})

	t.Run("list_notes_expect_one", func(t *testing.T) {
		verifyNotesList(t, env, headers, 1, noteAID, "A")
	})

	t.Run("websocket_and_crud_operations", func(t *testing.T) {
		testWebSocketCRUDOperations(t, env, authToken, headers, noteAID)
	})

	t.Run("test_pagination_with_120_notes", func(t *testing.T) {
		testPaginationWith120Notes(t, env, headers)
	})

	t.Run("test_note_not_found_cross_user", func(t *testing.T) {
		testCrossUserAuthorization(t, env, testPassword, noteAID)
	})
}

// createAndVerifyNote creates a note and returns its ID
func createAndVerifyNote(t *testing.T, env *TestEnvironment, headers map[string]string, title, body, color string) string {
	payload := map[string]any{"title": title, "body": body, "color": color}
	noteResp := makeHTTPRequest(t, "POST", env.BaseURL+"/api/v1/notes", payload, headers, http.StatusCreated)

	note := noteResp["note"].(map[string]any)
	assert.Equal(t, title, note["title"])
	assert.Equal(t, body, note["body"])
	assert.Equal(t, color, note["color"])
	assert.Contains(t, note, "id")
	assert.Contains(t, note, "created_at")
	assert.Contains(t, note, "updated_at")

	noteID := note["id"].(string)
	require.NotEmpty(t, noteID)
	return noteID
}

// verifyNotesList verifies the notes list response
func verifyNotesList(t *testing.T, env *TestEnvironment, headers map[string]string, expectedCount int, expectedID, expectedTitle string) {
	listResp := makeHTTPRequest(t, "GET", env.BaseURL+"/api/v1/notes", nil, headers, http.StatusOK)

	notes := listResp["notes"].([]any)
	assert.Len(t, notes, expectedCount)

	note := notes[0].(map[string]any)
	assert.Equal(t, expectedTitle, note["title"])
	assert.Equal(t, expectedID, note["id"])
}

// testWebSocketCRUDOperations tests WebSocket functionality with CRUD operations
func testWebSocketCRUDOperations(t *testing.T, env *TestEnvironment, authToken string, headers map[string]string, noteAID string) {
	ws := setupWebSocket(t, env, authToken)
	defer ws.Close()

	messages := make(chan map[string]any, 10)
	startWebSocketListener(ws, messages)
	time.Sleep(100 * time.Millisecond) // Allow connection to establish

	// Create note B and verify WebSocket event
	noteBID := createNoteAndVerifyWebSocketEvent(t, env, headers, messages, "B", "Note B content", "#00FF00", "created")

	// Update note A and verify WebSocket event
	updateNoteAndVerifyWebSocketEvent(t, env, headers, messages, noteAID, "A Updated", "Updated content for note A")

	// Delete note B and verify WebSocket event
	deleteNoteAndVerifyWebSocketEvent(t, env, headers, messages, noteBID)
}

// setupWebSocket creates and returns a WebSocket connection
func setupWebSocket(t *testing.T, env *TestEnvironment, authToken string) *websocket.Conn {
	wsURL := "ws://localhost" + env.BaseURL[len("http://localhost"):] + "/ws/notes/stream?token=" + authToken
	c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	return c
}

// startWebSocketListener starts a goroutine to read WebSocket messages
func startWebSocketListener(c *websocket.Conn, messages chan map[string]any) {
	go func() {
		for {
			var msg map[string]any
			if c.ReadJSON(&msg) != nil {
				return
			}
			messages <- msg
		}
	}()
}

// createNoteAndVerifyWebSocketEvent creates a note and verifies the WebSocket event
func createNoteAndVerifyWebSocketEvent(t *testing.T, env *TestEnvironment, headers map[string]string, messages chan map[string]any, title, body, color, eventType string) string {
	payload := map[string]any{"title": title, "body": body, "color": color}
	noteResp := makeHTTPRequest(t, "POST", env.BaseURL+"/api/v1/notes", payload, headers, http.StatusCreated)

	note := noteResp["note"].(map[string]any)
	noteID := note["id"].(string)

	verifyWebSocketMessage(t, messages, eventType, noteID, title, "", "")
	return noteID
}

// updateNoteAndVerifyWebSocketEvent updates a note and verifies the WebSocket event
func updateNoteAndVerifyWebSocketEvent(t *testing.T, env *TestEnvironment, headers map[string]string, messages chan map[string]any, noteID, newTitle, newBody string) {
	payload := map[string]any{"title": newTitle, "body": newBody}
	makeHTTPRequest(t, "PATCH", env.BaseURL+"/api/v1/notes/"+noteID, payload, headers, http.StatusOK)
	verifyWebSocketMessage(t, messages, "updated", noteID, newTitle, newBody, "")
}

// deleteNoteAndVerifyWebSocketEvent deletes a note and verifies the WebSocket event
func deleteNoteAndVerifyWebSocketEvent(t *testing.T, env *TestEnvironment, headers map[string]string, messages chan map[string]any, noteID string) {
	makeHTTPRequest(t, "DELETE", env.BaseURL+"/api/v1/notes/"+noteID, nil, headers, http.StatusNoContent)
	verifyWebSocketMessage(t, messages, "deleted", noteID, "", "", "deleted")
}

// verifyWebSocketMessage waits for and verifies a WebSocket message
func verifyWebSocketMessage(t *testing.T, messages chan map[string]any, eventType, noteID, expectedTitle, expectedBody, specialCase string) {
	select {
	case msg := <-messages:
		assert.Equal(t, eventType, msg["type"])
		assert.Contains(t, msg, "note")
		wsNote := msg["note"].(map[string]any)
		assert.Equal(t, noteID, wsNote["id"])

		if specialCase == "deleted" {
			assert.NotContains(t, wsNote, "title")
			assert.NotContains(t, wsNote, "body")
		} else {
			if expectedTitle != "" {
				assert.Equal(t, expectedTitle, wsNote["title"])
			}
			if expectedBody != "" {
				assert.Equal(t, expectedBody, wsNote["body"])
			}
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("did not receive %s event within 1 second", eventType)
	}
}

// testPaginationWith120Notes tests pagination functionality
func testPaginationWith120Notes(t *testing.T, env *TestEnvironment, headers map[string]string) {
	createNotesForPagination(t, env, headers, 120)
	testPaginationPages(t, env, headers)
}

// createNotesForPagination creates the specified number of notes
func createNotesForPagination(t *testing.T, env *TestEnvironment, headers map[string]string, count int) {
	for i := 1; i <= count; i++ {
		payload := map[string]any{
			"title": fmt.Sprintf("Note %d", i),
			"body":  fmt.Sprintf("Content for note %d", i),
		}
		makeHTTPRequest(t, "POST", env.BaseURL+"/api/v1/notes", payload, headers, http.StatusCreated)
	}
}

// testPaginationPages tests the pagination through multiple pages
func testPaginationPages(t *testing.T, env *TestEnvironment, headers map[string]string) {
	var totalNotes []any
	cursor := ""

	// Test 3 pages of pagination
	for page := 1; page <= 3; page++ {
		url := env.BaseURL + "/api/v1/notes?limit=50"
		if cursor != "" {
			url += "&cursor=" + cursor
		}

		pageResp := makeHTTPRequest(t, "GET", url, nil, headers, http.StatusOK)
		notes := pageResp["notes"].([]any)

		if page < 3 {
			assert.Len(t, notes, 50)
			nextCursor, ok := pageResp["next_cursor"].(string)
			require.True(t, ok, "should have next_cursor")
			require.NotEmpty(t, nextCursor)
			cursor = nextCursor
		} else {
			assert.True(t, len(notes) <= 50)
			if len(notes) < 50 {
				assert.Empty(t, pageResp["next_cursor"])
			}
		}

		totalNotes = append(totalNotes, notes...)
	}

	assert.True(t, len(totalNotes) >= 120, "should have at least 120 notes")
}

// testCrossUserAuthorization tests cross-user authorization
func testCrossUserAuthorization(t *testing.T, env *TestEnvironment, testPassword, noteAID string) {
	otherToken := setupTestUser(t, env, "otheruser@example.com", testPassword)
	otherHeaders := map[string]string{"Authorization": "Bearer " + otherToken}

	testUnauthorizedNoteAccess(t, env, otherHeaders, noteAID, "PATCH", map[string]any{"title": "Hacked"})
	testUnauthorizedNoteAccess(t, env, otherHeaders, noteAID, "DELETE", nil)
}

// testUnauthorizedNoteAccess tests unauthorized access to notes
func testUnauthorizedNoteAccess(t *testing.T, env *TestEnvironment, headers map[string]string, noteID, method string, payload map[string]any) {
	url := env.BaseURL + "/api/v1/notes/" + noteID
	errorResp := makeHTTPRequest(t, method, url, payload, headers, http.StatusNotFound)

	if msg, ok := errorResp["message"].(string); ok {
		assert.Contains(t, msg, "note not found")
	}
}

// makeHTTPRequest is a helper function to make HTTP requests with proper cleanup
func makeHTTPRequest(t *testing.T, method, url string, payload map[string]any, headers map[string]string, expectedStatus int) map[string]any {
	resp, err := httpJSON(method, url, payload, headers)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf(msgFailedToCloseResponseBody, err)
		}
	}()

	require.Equal(t, expectedStatus, resp.StatusCode)

	var result map[string]any
	if resp.StatusCode != http.StatusNoContent {
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	}

	return result
}

// setupTestUser creates a test user and returns the auth token
func setupTestUser(t *testing.T, env *TestEnvironment, email, password string) string {
	signUpPayload := map[string]string{
		"email":    email,
		"password": password,
	}

	resp, err := httpJSON("POST", env.BaseURL+"/api/v1/auth/sign-up", signUpPayload, nil)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf(msgFailedToCloseResponseBody, err)
		}
	}()

	if resp.StatusCode == http.StatusBadRequest {
		// User might already exist, try sign in
		resp, err = httpJSON("POST", env.BaseURL+"/api/v1/auth/sign-in", signUpPayload, nil)
		require.NoError(t, err)
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Errorf(msgFailedToCloseResponseBody, err)
			}
		}()
	}

	require.True(t, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK)

	var authResp map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&authResp))

	token, ok := authResp["token"].(string)
	require.True(t, ok)
	require.NotEmpty(t, token)

	return token
}
