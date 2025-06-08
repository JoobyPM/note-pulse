package notes

import (
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"note-pulse/cmd/server/testutil"
	"note-pulse/internal/config"
	"note-pulse/internal/logger"
	"note-pulse/internal/services/notes"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	gorillaws "github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	wsMaxIncomingBytes = 1 << 20 // 1 MiB
)

func TestWSUpgradeTableDriven(t *testing.T) {
	cfg := config.Config{
		LogLevel:  "info",
		LogFormat: "text",
	}
	_, err := logger.Init(cfg)
	require.NoError(t, err)

	config := DefaultWebSocketTestConfig()
	testCases := GetStandardWSUpgradeTestCases(t, config.Secret)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			app, _, _ := SetupWebSocketHandlersApp(t, config)

			req := testutil.CreateWebSocketRequest("/ws", tc.Token)
			resp, err := app.Test(req)
			require.NoError(t, err)
			assert.Equal(t, tc.ExpectedStatus, resp.StatusCode)
		})
	}
}

func TestWSUpgradeNonWebSocketRequest(t *testing.T) {
	cfg := config.Config{
		LogLevel:  "info",
		LogFormat: "text",
	}
	_, err := logger.Init(cfg)
	require.NoError(t, err)

	config := DefaultWebSocketTestConfig()
	app, _, _ := SetupWebSocketHandlersApp(t, config)

	req := testutil.CreateJSONRequest("GET", "/ws", nil)
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
	maxSessionSec := 2

	wsHandlers := NewWebSocketHandlers(hub, secret, maxSessionSec)

	// Create a test WebSocket server
	app := fiber.New()
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			userID := bson.NewObjectID().Hex()
			email := "test@example.com"
			c.Locals("userID", userID)
			c.Locals("userEmail", email)
			// Pass the correct context type so WSNotesStream doesn't reject the upgrade.
			c.Locals("parentCtx", c.UserContext())
			return c.Next()
		}
		return c.SendStatus(400)
	})
	app.Get("/ws", websocket.New(wsHandlers.WSNotesStream))

	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	listenerCloseErr := ln.Close() // Close the listener since Fiber will create its own
	require.NoError(t, listenerCloseErr)

	go func() {
		err := app.Listen(":" + fmt.Sprintf("%d", port))
		require.NoError(t, err)
	}()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Connect to WebSocket
	dialer := gorillaws.Dialer{}
	conn, _, err := dialer.Dial(fmt.Sprintf("ws://127.0.0.1:%d/ws", port), nil)
	if err != nil {
		t.Fatalf("Could not establish WebSocket connection for timeout test: %v", err)
	}
	conn.SetReadLimit(wsMaxIncomingBytes)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Errorf("failed to close WebSocket connection: %v", err)
		}
	}()

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
		var closeErr *gorillaws.CloseError
		if errors.As(readMessageErr, &closeErr) {
			assert.Equal(t, WSClosePolicyViolation, closeErr.Code, "Expected policy violation close code")
		}

		// Verify timing - should be close to maxSessionSec
		assert.True(t, elapsed >= 2*time.Second, "Connection should have been closed after session timeout")
		assert.True(t, elapsed < 4*time.Second, "Connection should have been closed promptly")
	}
}

// Integration test that verifies proper cleanup when WebSocket closes
func TestWSConnectionCleanup(t *testing.T) {
	hub := NewMockHub()
	userID := bson.NewObjectID()

	var sub *notes.Subscriber // will be set later

	// -- FIRST cleanup: runs **after** the cancel registered below
	t.Cleanup(func() {
		require.Eventually(t, func() bool {
			return hub.GetSubscriberCount() == 0 // should now be 0
		}, 100*time.Millisecond, 10*time.Millisecond,
			"Hub should have no subscribers after cleanup")

		select {
		case <-sub.Done:
		case <-time.After(50 * time.Millisecond):
			t.Fatal("Done channel should be closed after cleanup")
		}

		assert.Panics(t, func() {
			sub.Ch <- notes.NoteEvent{Type: "test"} // channel is closed
		}, "should panic when sending to closed channel")
	})

	// subscribe **after** the assertion cleanup is registered
	sub = WebSocketConnectionTest(t, hub, userID)
	require.Equal(t, 1, hub.GetSubscriberCount())
}

func TestValidateJWTTabledriven(t *testing.T) {
	hub := NewMockHub()
	secret := "test-secret-key-with-32-characters"
	maxSessionSec := 900
	wsHandlers := NewWebSocketHandlers(hub, secret, maxSessionSec)

	userID := bson.NewObjectID().Hex()
	email := "test@example.com"

	testCases := []struct {
		name        string
		setupToken  func() string
		expectError bool
		errorMsg    string
	}{
		{
			name: "Success",
			setupToken: func() string {
				token, _ := CreateTestJWTForWebSocket(userID, email, secret, time.Hour)
				return token
			},
			expectError: false,
		},
		{
			name: "InvalidFormat",
			setupToken: func() string {
				return "invalid.token.format"
			},
			expectError: true,
		},
		{
			name: "WrongSecret",
			setupToken: func() string {
				wrongSecret := "wrong-secret-key-with-32-characters"
				token, _ := CreateTestJWTForWebSocket(userID, email, wrongSecret, time.Hour)
				return token
			},
			expectError: true,
		},
		{
			name: "MissingClaims",
			setupToken: func() string {
				now := time.Now().UTC()
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"exp": now.Add(time.Hour).Unix(),
					"iat": now.Unix(),
					// Missing user_id and email
				})
				tokenString, _ := token.SignedString([]byte(secret))
				return tokenString
			},
			expectError: true,
			errorMsg:    "missing user_id",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token := tc.setupToken()
			parsedUserID, parsedEmail, err := wsHandlers.validateJWT(token)

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorMsg != "" {
					assert.Contains(t, err.Error(), tc.errorMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, userID, parsedUserID.Hex())
				assert.Equal(t, email, parsedEmail)
			}
		})
	}
}
