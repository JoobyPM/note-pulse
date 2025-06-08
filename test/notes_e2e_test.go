//go:build e2e

package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	notesPath = "/api/v1/notes"
	authPath  = "/api/v1/auth"
	testColor = "#FF0000"
)

func getAuthHeaders(t *testing.T, token string) map[string]string {
	t.Helper()
	return map[string]string{"Authorization": "Bearer " + token}
}

// NoteParams holds the parameters for creating a note
type NoteParams struct {
	Title string
	Body  string
	Color string
}

func TestNotesE2E(t *testing.T) {
	env := SetupTestEnvironment(t)
	testEmail := "noteuser@example.com"
	testPassword := "Password123"
	authToken := setupTestUser(t, env, testEmail, testPassword)
	headers := getAuthHeaders(t, authToken)

	var noteAID string

	t.Run("create_note_A", func(t *testing.T) {
		noteAID = createAndVerifyNote(t, env, headers, NoteParams{
			Title: "A",
			Body:  "Note A content",
			Color: testColor,
		})
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

	t.Run("test_advanced_listing_features", func(t *testing.T) {
		testAdvancedListingFeatures(t, env, headers)
	})

	t.Run("test_note_not_found_cross_user", func(t *testing.T) {
		testCrossUserAuthorization(t, env, testPassword, noteAID)
	})

	t.Run("test_compound_indexes_exist", func(t *testing.T) {
		// Ensure notes repository is initialized by creating a note first
		_ = createAndVerifyNote(t, env, headers, NoteParams{
			Title: "Index Test Note",
			Body:  "This note ensures the notes repository is initialized",
			Color: testColor,
		})
		testCompoundIndexesExist(t, env)
	})
}

// createAndVerifyNote creates a note and returns its ID
func createAndVerifyNote(t *testing.T, env *TestEnvironment, headers map[string]string, params NoteParams) string {
	payload := map[string]any{"title": params.Title, "body": params.Body, "color": params.Color}
	noteResp := makeHTTPRequest(t, "POST", env.BaseURL+notesPath, payload, headers, http.StatusCreated)

	note := noteResp["note"].(map[string]any)
	assert.Equal(t, params.Title, note["title"])
	assert.Equal(t, params.Body, note["body"])
	assert.Equal(t, params.Color, note["color"])
	assert.Contains(t, note, "id")
	assert.Contains(t, note, "created_at")
	assert.Contains(t, note, "updated_at")

	noteID := note["id"].(string)
	require.NotEmpty(t, noteID)
	return noteID
}

// verifyNotesList verifies the notes list response
func verifyNotesList(t *testing.T, env *TestEnvironment, headers map[string]string, expectedCount int, expectedID, expectedTitle string) {
	listResp := makeHTTPRequest(t, "GET", env.BaseURL+notesPath, nil, headers, http.StatusOK)

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
	noteBID := createNoteAndVerifyWebSocketEvent(t, env, headers, messages, NoteParams{
		Title: "B",
		Body:  "Note B content",
		Color: testColor,
	}, "created")

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
func createNoteAndVerifyWebSocketEvent(t *testing.T, env *TestEnvironment, headers map[string]string, messages chan map[string]any, params NoteParams, eventType string) string {
	payload := map[string]any{"title": params.Title, "body": params.Body, "color": params.Color}
	noteResp := makeHTTPRequest(t, "POST", env.BaseURL+notesPath, payload, headers, http.StatusCreated)

	note := noteResp["note"].(map[string]any)
	noteID := note["id"].(string)

	verifyWebSocketMessage(t, messages, eventType, noteID, params.Title, "", "")
	return noteID
}

// updateNoteAndVerifyWebSocketEvent updates a note and verifies the WebSocket event
func updateNoteAndVerifyWebSocketEvent(t *testing.T, env *TestEnvironment, headers map[string]string, messages chan map[string]any, noteID, newTitle, newBody string) {
	payload := map[string]any{"title": newTitle, "body": newBody}
	makeHTTPRequest(t, "PATCH", env.BaseURL+notesPath+"/"+noteID, payload, headers, http.StatusOK)
	verifyWebSocketMessage(t, messages, "updated", noteID, newTitle, newBody, "")
}

// deleteNoteAndVerifyWebSocketEvent deletes a note and verifies the WebSocket event
func deleteNoteAndVerifyWebSocketEvent(t *testing.T, env *TestEnvironment, headers map[string]string, messages chan map[string]any, noteID string) {
	makeHTTPRequest(t, "DELETE", env.BaseURL+notesPath+"/"+noteID, nil, headers, http.StatusNoContent)
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
		makeHTTPRequest(t, "POST", env.BaseURL+notesPath, payload, headers, http.StatusCreated)
	}
}

// testPaginationPages tests the pagination through multiple pages
func testPaginationPages(t *testing.T, env *TestEnvironment, headers map[string]string) {
	var totalNotes []any
	cursor := ""

	// Test 3 pages of pagination
	for page := 1; page <= 3; page++ {
		url := env.BaseURL + notesPath + "?limit=50"
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
	otherHeaders := getAuthHeaders(t, otherToken)

	testUnauthorizedNoteAccess(t, env, otherHeaders, noteAID, "PATCH", map[string]any{"title": "Hacked"})
	testUnauthorizedNoteAccess(t, env, otherHeaders, noteAID, "DELETE", nil)
}

// testUnauthorizedNoteAccess tests unauthorized access to notes
func testUnauthorizedNoteAccess(t *testing.T, env *TestEnvironment, headers map[string]string, noteID, method string, payload map[string]any) {
	url := env.BaseURL + notesPath + "/" + noteID
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

	resp, err := httpJSON("POST", env.BaseURL+authPath+"/sign-up", signUpPayload, nil)
	require.NoError(t, err)
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf(msgFailedToCloseResponseBody, err)
		}
	}()

	if resp.StatusCode == http.StatusBadRequest {
		// User might already exist, try sign in
		resp, err = httpJSON("POST", env.BaseURL+authPath+"/sign-in", signUpPayload, nil)
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

// testAdvancedListingFeatures tests the new search, filter, sort, and pagination features
func testAdvancedListingFeatures(t *testing.T, env *TestEnvironment, _ map[string]string) {
	// Use a separate user for this test to ensure isolation
	advancedTestToken := setupTestUser(t, env, "advancedtest@example.com", "Password123")
	advancedHeaders := getAuthHeaders(t, advancedTestToken)
	// Create test notes with different colors and content
	redNote1 := createAndVerifyNote(t, env, advancedHeaders, NoteParams{
		Title: "Meeting Notes",
		Body:  "Important meeting about quarterly targets",
		Color: testColor,
	})

	redNote2 := createAndVerifyNote(t, env, advancedHeaders, NoteParams{
		Title: "Project Planning",
		Body:  "Meeting to discuss project milestones",
		Color: testColor,
	})

	blueNote := createAndVerifyNote(t, env, advancedHeaders, NoteParams{
		Title: "Shopping List",
		Body:  "Groceries and household items",
		Color: "#0000FF", // Blue color instead of red
	})

	// Test color filter - should return 2 red notes
	t.Run("color_filter", func(t *testing.T) {
		url := env.BaseURL + notesPath + "?color=%23FF0000"
		resp := makeHTTPRequest(t, "GET", url, nil, advancedHeaders, http.StatusOK)

		notes := resp["notes"].([]any)
		assert.Len(t, notes, 2, "should find 2 red notes")

		// Check response includes pagination metadata
		assert.Contains(t, resp, "has_more")
		assert.Contains(t, resp, "total_count")
		assert.False(t, resp["has_more"].(bool))
		assert.Equal(t, float64(2), resp["total_count"].(float64))

		// Verify all returned notes are red
		for _, note := range notes {
			noteMap := note.(map[string]any)
			assert.Equal(t, testColor, noteMap["color"])
		}
	})

	// Test search functionality - should find notes containing "meeting"
	t.Run("search_functionality", func(t *testing.T) {
		url := env.BaseURL + notesPath + "?q=meeting"
		resp := makeHTTPRequest(t, "GET", url, nil, advancedHeaders, http.StatusOK)

		notes := resp["notes"].([]any)
		assert.Len(t, notes, 2, "should find 2 notes containing 'meeting'")

		// Verify search results contain the search term
		for _, note := range notes {
			noteMap := note.(map[string]any)
			title := noteMap["title"].(string)
			body := noteMap["body"].(string)

			// Should contain "meeting" or "Meeting" in title or body
			containsInTitle := contains(title, "meeting") || contains(title, "Meeting")
			containsInBody := contains(body, "meeting") || contains(body, "Meeting")
			assert.True(t, containsInTitle || containsInBody,
				"note should contain 'meeting' in title or body")
		}
	})

	// Test sorting by title ascending - should return notes alphabetically
	t.Run("sort_by_title_asc", func(t *testing.T) {
		url := env.BaseURL + notesPath + "?sort=title&order=asc"
		resp := makeHTTPRequest(t, "GET", url, nil, advancedHeaders, http.StatusOK)

		notes := resp["notes"].([]any)
		assert.GreaterOrEqual(t, len(notes), 3, "should have at least 3 notes")

		// Verify first note has alphabetically smallest title among our test notes
		firstNote := notes[0].(map[string]any)
		title := firstNote["title"].(string)

		// Among our test notes, "Meeting Notes" should come first alphabetically
		assert.Equal(t, "Meeting Notes", title, "first note should be 'Meeting Notes' when sorted by title asc")
	})

	// Test combined filters and pagination metadata
	t.Run("combined_filters_and_metadata", func(t *testing.T) {
		url := env.BaseURL + notesPath + "?color=%23FF0000&q=meeting&sort=title&order=desc&limit=1"
		resp := makeHTTPRequest(t, "GET", url, nil, advancedHeaders, http.StatusOK)

		notes := resp["notes"].([]any)
		assert.Len(t, notes, 1, "should return 1 note with combined filters")

		// Should have pagination metadata
		assert.True(t, resp["has_more"].(bool), "should indicate more results available")
		assert.NotEmpty(t, resp["next_cursor"], "should provide next cursor")

		// Should be a red note containing "meeting"
		note := notes[0].(map[string]any)
		assert.Equal(t, testColor, note["color"])
		title := note["title"].(string)
		body := note["body"].(string)
		containsInTitle := contains(title, "meeting") || contains(title, "Meeting")
		containsInBody := contains(body, "meeting") || contains(body, "Meeting")
		assert.True(t, containsInTitle || containsInBody)
	})

	// Clean up test notes
	makeHTTPRequest(t, "DELETE", env.BaseURL+notesPath+"/"+redNote1, nil, advancedHeaders, http.StatusNoContent)
	makeHTTPRequest(t, "DELETE", env.BaseURL+notesPath+"/"+redNote2, nil, advancedHeaders, http.StatusNoContent)
	makeHTTPRequest(t, "DELETE", env.BaseURL+notesPath+"/"+blueNote, nil, advancedHeaders, http.StatusNoContent)
}

// contains is a helper function to check if a string contains a substring (case-insensitive)
func contains(str, substr string) bool {
	return len(str) >= len(substr) &&
		(str == substr ||
			(len(str) > len(substr) &&
				containsHelper(str, substr)))
}

func containsHelper(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if equalIgnoreCase(str[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalIgnoreCase(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range len(a) {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// testCompoundIndexesExist verifies that the required compound indexes are created
func testCompoundIndexesExist(t *testing.T, _ *TestEnvironment) {
	// Connect to MongoDB using environment variables
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoURI := os.Getenv("MONGO_URI")
	require.NotEmpty(t, mongoURI, "MONGO_URI environment variable should be set")

	client, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	require.NoError(t, err)
	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			t.Errorf("failed to disconnect from MongoDB: %v", err)
		}
	}()

	dbName := os.Getenv("MONGO_DB_NAME")
	if dbName == "" {
		dbName = "e2e" // fallback to default
	}

	db := client.Database(dbName)
	collection := db.Collection("notes")

	// List all indexes
	cursor, err := collection.Indexes().List(ctx)
	require.NoError(t, err)
	defer cursor.Close(ctx)

	var indexes []bson.M
	err = cursor.All(ctx, &indexes)
	require.NoError(t, err)

	// Verify we have the expected number of indexes (at least the 3 compound indexes + default _id)
	assert.GreaterOrEqual(t, len(indexes), 4, "should have at least 4 indexes including compound indexes")

	// Expected index names that should exist
	expectedIndexNames := []string{
		"user_id_1__id_-1",               // Basic pagination index
		"user_id_1_updated_at_-1__id_-1", // Updated_at sorting index
		"user_id_1_created_at_-1__id_-1", // Created_at sorting index
	}

	// Check each expected index exists by name
	for _, expectedName := range expectedIndexNames {
		found := false
		for _, index := range indexes {
			if name, ok := index["name"].(string); ok && name == expectedName {
				t.Logf("Found expected index: %s", expectedName)
				found = true
				break
			}
		}
		assert.True(t, found, "expected compound index '%s' not found", expectedName)
	}
}
