package notes

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	"note-pulse/cmd/server/ctxkeys"
	"note-pulse/cmd/server/testutil"
	"note-pulse/internal/services/notes"

	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"
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

func (m *MockHub) Unsubscribe(_ context.Context, connULID ulid.ULID) {
	if sub, exists := m.subscribers[connULID]; exists {
		close(sub.Ch)
		close(sub.Done)
		delete(m.subscribers, connULID)
	}
}

func (m *MockHub) GetSubscriberCount() int {
	return len(m.subscribers)
}

// WebSocketTestConfig holds configuration for WebSocket tests
type WebSocketTestConfig struct {
	Secret        string
	MaxSessionSec int
}

// DefaultWebSocketTestConfig returns a default test configuration
func DefaultWebSocketTestConfig() WebSocketTestConfig {
	return WebSocketTestConfig{
		Secret:        "test-secret-key-with-32-characters",
		MaxSessionSec: 900,
	}
}

// SetupWebSocketHandlersApp creates a test app with WebSocket handlers
func SetupWebSocketHandlersApp(t *testing.T, config WebSocketTestConfig) (*fiber.App, *MockHub, *WebSocketHandlers) {
	t.Helper()

	app := testutil.CreateTestApp(t)
	hub := NewMockHub()
	wsHandlers := NewWebSocketHandlers(hub, config.Secret, config.MaxSessionSec)

	app.Get("/ws", wsHandlers.WSUpgrade, func(c *fiber.Ctx) error {
		userID := c.Locals(ctxkeys.UserIDKey).(string)
		userEmail := c.Locals(ctxkeys.UserEmailKey).(string)
		return c.JSON(fiber.Map{
			"user_id": userID,
			"email":   userEmail,
		})
	})

	return app, hub, wsHandlers
}

// CreateTestJWTForWebSocket creates a JWT token for WebSocket testing
func CreateTestJWTForWebSocket(userID, email, secret string, expiry time.Duration) (string, error) {
	return testutil.CreateTestJWT(userID, email, []byte(secret), expiry)
}

// WSUpgradeTestCase represents a WebSocket upgrade test case
type WSUpgradeTestCase struct {
	Name           string
	Token          *string // nil means no token
	ExpectedStatus int
}

// GetStandardWSUpgradeTestCases returns common WebSocket upgrade test cases
func GetStandardWSUpgradeTestCases(t *testing.T, secret string) []WSUpgradeTestCase {
	t.Helper()

	userID := bson.NewObjectID().Hex()
	email := "test@example.com"

	validToken, err := CreateTestJWTForWebSocket(userID, email, secret, time.Hour)
	require.NoError(t, err)

	expiredToken, err := CreateTestJWTForWebSocket(userID, email, secret, -time.Hour)
	require.NoError(t, err)

	invalidToken := "invalid-token"

	return []WSUpgradeTestCase{
		{
			Name:           "ValidToken",
			Token:          &validToken,
			ExpectedStatus: 200,
		},
		{
			Name:           "MissingToken",
			Token:          nil,
			ExpectedStatus: 401,
		},
		{
			Name:           "InvalidToken",
			Token:          &invalidToken,
			ExpectedStatus: 401,
		},
		{
			Name:           "ExpiredToken",
			Token:          &expiredToken,
			ExpectedStatus: 401,
		},
	}
}

// WebSocketConnectionTest performs a WebSocket connection test with cleanup
func WebSocketConnectionTest(t *testing.T, hub *MockHub, userID bson.ObjectID) *notes.Subscriber {
	t.Helper()

	now := time.Now().UTC()
	connULID := ulid.MustNew(ulid.Timestamp(now), rand.Reader)

	sub, cancel := hub.Subscribe(context.Background(), connULID, userID)

	t.Cleanup(cancel)

	return sub
}
