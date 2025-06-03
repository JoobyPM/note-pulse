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

	headers := map[string]string{
		"Authorization": "Bearer " + authToken,
	}

	var noteAID, noteBID string

	t.Run("create_note_A", func(t *testing.T) {
		payload := map[string]any{
			"title": "A",
			"body":  "Note A content",
			"color": "#FF0000",
		}

		resp, err := httpJSON("POST", env.BaseURL+"/api/v1/notes", payload, headers)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var noteResp map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&noteResp))

		assert.Contains(t, noteResp, "note")
		note := noteResp["note"].(map[string]any)
		assert.Equal(t, "A", note["title"])
		assert.Equal(t, "Note A content", note["body"])
		assert.Equal(t, "#FF0000", note["color"])
		assert.Contains(t, note, "id")
		assert.Contains(t, note, "created_at")
		assert.Contains(t, note, "updated_at")

		noteAID = note["id"].(string)
		require.NotEmpty(t, noteAID)
	})

	t.Run("list_notes_expect_one", func(t *testing.T) {
		resp, err := httpJSON("GET", env.BaseURL+"/api/v1/notes", nil, headers)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var listResp map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&listResp))

		assert.Contains(t, listResp, "notes")
		notes := listResp["notes"].([]any)
		assert.Len(t, notes, 1)

		note := notes[0].(map[string]any)
		assert.Equal(t, "A", note["title"])
		assert.Equal(t, noteAID, note["id"])
	})

	t.Run("websocket_and_crud_operations", func(t *testing.T) {
		// Open WebSocket connection
		// Convert HTTP URL to WebSocket URL
		wsURL := "ws://localhost" + env.BaseURL[len("http://localhost"):] + "/ws/notes/stream?token=" + authToken
		c, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer c.Close()

		// Channel to receive WebSocket messages
		messages := make(chan map[string]any, 10)
		done := make(chan bool)

		// Start goroutine to read WebSocket messages
		go func() {
			defer close(done)
			for {
				var msg map[string]any
				err := c.ReadJSON(&msg)
				if err != nil {
					return
				}
				messages <- msg
			}
		}()

		// Give WebSocket connection time to establish
		time.Sleep(100 * time.Millisecond)

		// Create note B
		payload := map[string]any{
			"title": "B",
			"body":  "Note B content",
			"color": "#00FF00",
		}

		resp, err := httpJSON("POST", env.BaseURL+"/api/v1/notes", payload, headers)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode)

		var noteResp map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&noteResp))
		resp.Body.Close()
		note := noteResp["note"].(map[string]any)
		noteBID = note["id"].(string)

		// Wait for WebSocket message and verify "created" event
		select {
		case msg := <-messages:
			assert.Equal(t, "created", msg["type"])
			assert.Contains(t, msg, "note")
			wsNote := msg["note"].(map[string]any)
			assert.Equal(t, "B", wsNote["title"])
			assert.Equal(t, noteBID, wsNote["id"])
		case <-time.After(1 * time.Second):
			t.Fatal("did not receive created event within 1 second")
		}

		// Update note A
		updatePayload := map[string]any{
			"title": "A Updated",
			"body":  "Updated content for note A",
		}

		resp, err = httpJSON("PATCH", env.BaseURL+"/api/v1/notes/"+noteAID, updatePayload, headers)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()

		// Wait for WebSocket message and verify "updated" event
		select {
		case msg := <-messages:
			assert.Equal(t, "updated", msg["type"])
			assert.Contains(t, msg, "note")
			wsNote := msg["note"].(map[string]any)
			assert.Equal(t, "A Updated", wsNote["title"])
			assert.Equal(t, "Updated content for note A", wsNote["body"])
			assert.Equal(t, noteAID, wsNote["id"])
		case <-time.After(1 * time.Second):
			t.Fatal("did not receive updated event within 1 second")
		}

		// Delete note B
		resp, err = httpJSON("DELETE", env.BaseURL+"/api/v1/notes/"+noteBID, nil, headers)
		require.NoError(t, err)
		require.Equal(t, http.StatusNoContent, resp.StatusCode)
		resp.Body.Close()

		// Wait for WebSocket message and verify "deleted" event
		select {
		case msg := <-messages:
			assert.Equal(t, "deleted", msg["type"])
			assert.Contains(t, msg, "note")
			wsNote := msg["note"].(map[string]any)
			assert.Equal(t, noteBID, wsNote["id"])
			// Deleted events should only contain ID
			assert.NotContains(t, wsNote, "title")
			assert.NotContains(t, wsNote, "body")
		case <-time.After(1 * time.Second):
			t.Fatal("did not receive deleted event within 1 second")
		}

		c.Close()
		<-done
	})

	t.Run("test_pagination_with_120_notes", func(t *testing.T) {
		// Create 120 notes for pagination testing
		for i := 1; i <= 120; i++ {
			payload := map[string]any{
				"title": fmt.Sprintf("Note %d", i),
				"body":  fmt.Sprintf("Content for note %d", i),
			}

			resp, err := httpJSON("POST", env.BaseURL+"/api/v1/notes", payload, headers)
			require.NoError(t, err)
			resp.Body.Close()
			require.Equal(t, http.StatusCreated, resp.StatusCode)
		}

		// Test pagination: 3 pages of 50/50/20
		var totalNotes []any
		var cursor string

		// Page 1: 50 notes
		url := env.BaseURL + "/api/v1/notes?limit=50"
		resp, err := httpJSON("GET", url, nil, headers)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var page1 map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&page1))

		notes1 := page1["notes"].([]any)
		assert.Len(t, notes1, 50)
		totalNotes = append(totalNotes, notes1...)

		nextCursor, ok := page1["next_cursor"].(string)
		require.True(t, ok, "should have next_cursor")
		require.NotEmpty(t, nextCursor)
		cursor = nextCursor

		// Page 2: 50 notes
		url = env.BaseURL + "/api/v1/notes?limit=50&cursor=" + cursor
		resp, err = httpJSON("GET", url, nil, headers)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var page2 map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&page2))

		notes2 := page2["notes"].([]any)
		assert.Len(t, notes2, 50)
		totalNotes = append(totalNotes, notes2...)

		nextCursor, ok = page2["next_cursor"].(string)
		require.True(t, ok, "should have next_cursor")
		require.NotEmpty(t, nextCursor)
		cursor = nextCursor

		// Page 3: remaining notes (should be less than 50)
		url = env.BaseURL + "/api/v1/notes?limit=50&cursor=" + cursor
		resp, err = httpJSON("GET", url, nil, headers)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var page3 map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&page3))

		notes3 := page3["notes"].([]any)
		// Should have remaining notes (120 total - we already created 1 note A that wasn't deleted)
		assert.True(t, len(notes3) <= 50)
		totalNotes = append(totalNotes, notes3...)

		// Page 3 should not have a next_cursor if we've reached the end
		if len(notes3) < 50 {
			assert.Empty(t, page3["next_cursor"])
		}

		// Verify we got all notes (plus the original note A)
		assert.True(t, len(totalNotes) >= 120, "should have at least 120 notes")
	})

	t.Run("test_note_not_found_cross_user", func(t *testing.T) {
		// Create another user
		otherEmail := "otheruser@example.com"
		otherToken := setupTestUser(t, env, otherEmail, testPassword)

		otherHeaders := map[string]string{
			"Authorization": "Bearer " + otherToken,
		}

		// Try to update note A (belongs to first user) with second user's token
		updatePayload := map[string]any{
			"title": "Hacked",
		}

		resp, err := httpJSON("PATCH", env.BaseURL+"/api/v1/notes/"+noteAID, updatePayload, otherHeaders)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var errorResp map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errorResp))
		if msg, ok := errorResp["message"].(string); ok {
			assert.Contains(t, msg, "note not found")
		}

		// Try to delete note A with second user's token
		resp, err = httpJSON("DELETE", env.BaseURL+"/api/v1/notes/"+noteAID, nil, otherHeaders)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusBadRequest, resp.StatusCode)

		errorResp = make(map[string]any)
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&errorResp))
		if msg, ok := errorResp["message"].(string); ok {
			assert.Contains(t, msg, "note not found")
		}
	})
}

// setupTestUser creates a test user and returns the auth token
func setupTestUser(t *testing.T, env *TestEnvironment, email, password string) string {
	signUpPayload := map[string]string{
		"email":    email,
		"password": password,
	}

	resp, err := httpJSON("POST", env.BaseURL+"/api/v1/auth/sign-up", signUpPayload, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		// User might already exist, try sign in
		resp, err = httpJSON("POST", env.BaseURL+"/api/v1/auth/sign-in", signUpPayload, nil)
		require.NoError(t, err)
		defer resp.Body.Close()
	}

	require.True(t, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK)

	var authResp map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&authResp))

	token, ok := authResp["token"].(string)
	require.True(t, ok)
	require.NotEmpty(t, token)

	return token
}
