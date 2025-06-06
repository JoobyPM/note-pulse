package notes

import (
	"context"
	"crypto/rand"
	"net/http"
	"testing"
	"time"

	"note-pulse/internal/config"
	"note-pulse/internal/handlers/httperr"
	"note-pulse/internal/logger"
	"note-pulse/internal/services/notes"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	gorillaws "github.com/gorilla/websocket"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// MockHub implements the Hub interface for testing
type MockHub struct {
	subscribers    map[ulid.ULID]*notes.Subscriber
	subscribeCount int
}

func NewMockHub() *MockHub {
	return &MockHub{
		subscribers: make(map[ulid.ULID]*notes.Subscriber),
	}
}

func (m *MockHub) Subscribe(ctx context.Context, connULID ulid.ULID, userID bson.ObjectID) (*notes.Subscriber, func()) {
	sub := &notes.Subscriber{
		UserID: userID,
		Ch:     make(chan notes.NoteEvent, 10),
		Done:   make(chan struct{}),
	}
	m.subscribers[connULID] = sub
	m.subscribeCount++

	cancel := func() {
		m.Unsubscribe(ctx, connULID)
	}
	return sub, cancel
}

func (m *MockHub) Unsubscribe(ctx context.Context, connULID ulid.ULID) {
	if sub, exists := m.subscribers[connULID]; exists {
		close(sub.Ch)
		close(sub.Done)
		delete(m.subscribers, connULID)
	}
}

func (m *MockHub) GetSubscriberCount() int {
	return len(m.subscribers)
}

func createTestJWT(userID string, email string, secret []byte, expiry time.Duration) (string, error) {
	now := time.Now().UTC()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"exp":     now.Add(expiry).Unix(),
		"iat":     now.Unix(),
	})

	return token.SignedString(secret)
}

func TestWSUpgrade_ValidToken(t *testing.T) {
	cfg := config.Config{
		LogLevel:  "info",
		LogFormat: "text",
	}
	_, err := logger.Init(cfg)
	require.NoError(t, err)

	hub := NewMockHub()
	secret := "test-secret-key-with-32-characters"
	maxSessionSec := 900

	wsHandlers := NewWebSocketHandlers(hub, secret, maxSessionSec)

	// Create test app
	app := fiber.New(fiber.Config{
		ErrorHandler: httperr.Handler,
	})
	app.Get("/ws", wsHandlers.WSUpgrade, func(c *fiber.Ctx) error {
		userID := c.Locals("userID").(string)
		userEmail := c.Locals("userEmail").(string)
		return c.JSON(fiber.Map{
			"user_id": userID,
			"email":   userEmail,
		})
	})

	// Create valid JWT
	userID := bson.NewObjectID().Hex()
	email := "test@example.com"
	token, err := createTestJWT(userID, email, []byte(secret), time.Hour)
	require.NoError(t, err)

	// Test with valid token
	req, err := http.NewRequest("GET", "/ws?token="+token, nil)
	require.NoError(t, err)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "test-key")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestWSUpgrade_MissingToken(t *testing.T) {
	cfg := config.Config{
		LogLevel:  "info",
		LogFormat: "text",
	}
	_, err := logger.Init(cfg)
	require.NoError(t, err)

	hub := NewMockHub()
	secret := "test-secret-key-with-32-characters"
	maxSessionSec := 900

	wsHandlers := NewWebSocketHandlers(hub, secret, maxSessionSec)

	app := fiber.New(fiber.Config{
		ErrorHandler: httperr.Handler,
	})
	app.Get("/ws", wsHandlers.WSUpgrade, func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req, err := http.NewRequest("GET", "/ws", nil)
	require.NoError(t, err)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "test-key")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)
}

func TestWSUpgrade_InvalidToken(t *testing.T) {
	cfg := config.Config{
		LogLevel:  "info",
		LogFormat: "text",
	}
	_, err := logger.Init(cfg)
	require.NoError(t, err)

	hub := NewMockHub()
	secret := "test-secret-key-with-32-characters"
	maxSessionSec := 900

	wsHandlers := NewWebSocketHandlers(hub, secret, maxSessionSec)

	app := fiber.New(fiber.Config{
		ErrorHandler: httperr.Handler,
	})
	app.Get("/ws", wsHandlers.WSUpgrade, func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	req, err := http.NewRequest("GET", "/ws?token=invalid-token", nil)
	require.NoError(t, err)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "test-key")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)
}

func TestWSUpgrade_ExpiredToken(t *testing.T) {
	cfg := config.Config{
		LogLevel:  "info",
		LogFormat: "text",
	}
	_, err := logger.Init(cfg)
	require.NoError(t, err)

	hub := NewMockHub()
	secret := "test-secret-key-with-32-characters"
	maxSessionSec := 900

	wsHandlers := NewWebSocketHandlers(hub, secret, maxSessionSec)

	app := fiber.New(fiber.Config{
		ErrorHandler: httperr.Handler,
	})
	app.Get("/ws", wsHandlers.WSUpgrade, func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Create expired JWT
	userID := bson.NewObjectID().Hex()
	email := "test@example.com"
	token, err := createTestJWT(userID, email, []byte(secret), -time.Hour) // Expired 1 hour ago
	require.NoError(t, err)

	req, err := http.NewRequest("GET", "/ws?token="+token, nil)
	require.NoError(t, err)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "test-key")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 401, resp.StatusCode)
}

func TestWSUpgrade_NonWebSocketRequest(t *testing.T) {
	cfg := config.Config{
		LogLevel:  "info",
		LogFormat: "text",
	}
	_, err := logger.Init(cfg)
	require.NoError(t, err)

	hub := NewMockHub()
	secret := "test-secret-key-with-32-characters"
	maxSessionSec := 900

	wsHandlers := NewWebSocketHandlers(hub, secret, maxSessionSec)

	app := fiber.New(fiber.Config{
		ErrorHandler: httperr.Handler,
	})
	app.Get("/ws", wsHandlers.WSUpgrade, func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Regular HTTP request without WebSocket headers
	req, err := http.NewRequest("GET", "/ws", nil)
	require.NoError(t, err)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

func TestWSSessionTimeout(t *testing.T) {
	cfg := config.Config{
		LogLevel:  "info",
		LogFormat: "text",
	}
	_, err := logger.Init(cfg)
	require.NoError(t, err)

	hub := NewMockHub()
	secret := "test-secret-key-with-32-characters"
	maxSessionSec := 2 // 2 seconds for testing

	wsHandlers := NewWebSocketHandlers(hub, secret, maxSessionSec)

	// Create a test WebSocket server
	app := fiber.New()
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			userID := bson.NewObjectID().Hex()
			email := "test@example.com"
			c.Locals("userID", userID)
			c.Locals("userEmail", email)
			c.Locals("parentCtx", c.Context())
			return c.Next()
		}
		return c.SendStatus(400)
	})
	app.Get("/ws", websocket.New(wsHandlers.WSNotesStream))

	go func() {
		err := app.Listen(":8888")
		require.NoError(t, err)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Connect to WebSocket
	dialer := gorillaws.Dialer{}
	conn, _, err := dialer.Dial("ws://localhost:8888/ws", nil)
	if err != nil {
		t.Fatalf("Could not establish WebSocket connection for timeout test: %v", err)
	}
	defer conn.Close()

	now := time.Now().UTC()

	// Set read deadline to detect close
	deadline := now.Add(5 * time.Second)
	setReadDeadlineErr := conn.SetReadDeadline(deadline)
	require.NoError(t, setReadDeadlineErr)

	// Wait for the connection to be closed due to timeout
	start := time.Now().UTC()
	_, _, readMessageErr := conn.ReadMessage()
	require.Error(t, readMessageErr)
	elapsed := time.Since(start)

	// The connection should be closed due to timeout
	if readMessageErr != nil {
		// Check if it's a close error with the expected close code
		if closeErr, ok := err.(*gorillaws.CloseError); ok {
			assert.Equal(t, 1008, closeErr.Code, "Expected policy violation close code")
		}

		// Verify timing - should be close to maxSessionSec
		assert.True(t, elapsed >= 2*time.Second, "Connection should have been closed after session timeout")
		assert.True(t, elapsed < 4*time.Second, "Connection should have been closed promptly")
	}
}

// Integration test that verifies proper cleanup when WebSocket closes
func TestWSConnectionCleanup(t *testing.T) {
	hub := NewMockHub()
	now := time.Now().UTC()

	// Initial state
	require.Equal(t, 0, hub.GetSubscriberCount())

	// Simulate WebSocket connection establishment and closure
	// This is a simplified test since we can't easily mock the full WebSocket lifecycle
	userID := bson.NewObjectID()
	connULID := ulid.MustNew(ulid.Timestamp(now), rand.Reader)

	// Subscribe (simulating what happens in WSNotesStream)
	sub, cancel := hub.Subscribe(context.Background(), connULID, userID)
	require.Equal(t, 1, hub.GetSubscriberCount())

	// Simulate connection closure (cancel is called in defer)
	cancel()

	// Verify cleanup
	require.Eventually(t, func() bool {
		return hub.GetSubscriberCount() == 0
	}, 100*time.Millisecond, 10*time.Millisecond, "Hub should have no subscribers after cleanup")

	// Verify channels are closed
	select {
	case <-sub.Done:
		// Expected
	case <-time.After(50 * time.Millisecond):
		t.Fatal("Done channel should be closed after cleanup")
	}

	// Verify we can't send on the channel
	assert.Panics(t, func() {
		sub.Ch <- notes.NoteEvent{Type: "test"}
	}, "Should panic when sending to closed channel")
}

func TestValidateJWT_Success(t *testing.T) {
	hub := NewMockHub()
	secret := "test-secret-key-with-32-characters"
	maxSessionSec := 900

	wsHandlers := NewWebSocketHandlers(hub, secret, maxSessionSec)

	userID := bson.NewObjectID().Hex()
	email := "test@example.com"
	token, err := createTestJWT(userID, email, []byte(secret), time.Hour)
	require.NoError(t, err)

	parsedUserID, parsedEmail, err := wsHandlers.validateJWT(token)
	require.NoError(t, err)
	assert.Equal(t, userID, parsedUserID.Hex())
	assert.Equal(t, email, parsedEmail)
}

func TestValidateJWT_InvalidFormat(t *testing.T) {
	hub := NewMockHub()
	secret := "test-secret-key-with-32-characters"
	maxSessionSec := 900

	wsHandlers := NewWebSocketHandlers(hub, secret, maxSessionSec)

	_, _, err := wsHandlers.validateJWT("invalid.token.format")
	assert.Error(t, err)
}

func TestValidateJWT_WrongSecret(t *testing.T) {
	hub := NewMockHub()
	secret := "test-secret-key-with-32-characters"
	wrongSecret := "wrong-secret-key-with-32-characters"
	maxSessionSec := 900

	wsHandlers := NewWebSocketHandlers(hub, secret, maxSessionSec)

	userID := bson.NewObjectID().Hex()
	email := "test@example.com"
	token, err := createTestJWT(userID, email, []byte(wrongSecret), time.Hour)
	require.NoError(t, err)

	_, _, err = wsHandlers.validateJWT(token)
	assert.Error(t, err)
}

func TestValidateJWT_MissingClaims(t *testing.T) {
	hub := NewMockHub()
	secret := "test-secret-key-with-32-characters"
	maxSessionSec := 900
	now := time.Now().UTC()

	wsHandlers := NewWebSocketHandlers(hub, secret, maxSessionSec)

	// Create token without required claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": now.Add(time.Hour).Unix(),
		"iat": now.Unix(),
		// Missing user_id and email
	})

	tokenString, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	_, _, err = wsHandlers.validateJWT(tokenString)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing user_id")
}
