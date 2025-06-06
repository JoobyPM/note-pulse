package notes

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"note-pulse/internal/handlers/httperr"
	"note-pulse/internal/logger"
	"note-pulse/internal/services/notes"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/oklog/ulid/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	// WSClosePolicyViolation represents WebSocket close code for policy violation
	WSClosePolicyViolation = 1008
)

// Hub interface for WebSocket management
type Hub interface {
	Subscribe(ctx context.Context, connULID ulid.ULID, userID bson.ObjectID) (*notes.Subscriber, func())
	Unsubscribe(ctx context.Context, connULID ulid.ULID)
}

// WebSocketHandlers contains WebSocket-related handlers
type WebSocketHandlers struct {
	hub           Hub
	jwtSecret     string
	maxSessionSec int
}

// NewWebSocketHandlers creates new WebSocket handlers
func NewWebSocketHandlers(hub Hub, jwtSecret string, maxSessionSec int) *WebSocketHandlers {
	return &WebSocketHandlers{
		hub:           hub,
		jwtSecret:     jwtSecret,
		maxSessionSec: maxSessionSec,
	}
}

// WSUpgrade upgrades HTTP connection to WebSocket for notes streaming
func (h *WebSocketHandlers) WSUpgrade(c *fiber.Ctx) error {
	if websocket.IsWebSocketUpgrade(c) {
		// Validate JWT token from query parameter
		token := c.Query("token")
		if token == "" {
			logger.L().Warn("missing token in websocket upgrade", "handler", "WSUpgrade", "path", c.Path())
			return httperr.Fail(httperr.E{
				Status:  401,
				Message: "Missing token",
			})
		}

		userID, userEmail, err := h.validateJWT(token)
		if err != nil {
			logger.L().Error("invalid token in websocket upgrade", "handler", "WSUpgrade", "path", c.Path(), "error", err)
			return httperr.Fail(httperr.E{
				Status:  401,
				Message: "Invalid token",
			})
		}

		// Store user info and context in locals for the WebSocket handler
		c.Locals("userID", userID.Hex())
		c.Locals("userEmail", userEmail)
		c.Locals("parentCtx", c.Context())

		return c.Next()
	}

	logger.L().Warn("websocket upgrade required", "handler", "WSUpgrade", "path", c.Path())
	return httperr.Fail(httperr.E{
		Status:  400,
		Message: "WebSocket upgrade required",
	})
}

// WSNotesStream handles WebSocket connections for real-time notes updates
func (h *WebSocketHandlers) WSNotesStream(c *websocket.Conn) {
	userIDStr, ok := c.Locals("userID").(string)
	if !ok {
		logger.L().Error("userID not found in WebSocket context")
		c.Close()
		return
	}

	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		logger.L().Error("invalid userID in WebSocket context", "userID", userIDStr, "error", err)
		c.Close()
		return
	}

	// Generate unique connection ID using ULID
	connULID := ulid.MustNew(ulid.Timestamp(time.Now().UTC()), rand.Reader)
	connID := connULID.String()

	// Retrieve parent context from Fiber handler
	parentCtx, ok := c.Locals("parentCtx").(context.Context)
	if !ok {
		logger.L().Error("parentCtx not found in WebSocket context")
		c.Close()
		return
	}

	// Handle incoming messages and outgoing events
	ctx, cancelCtx := context.WithCancel(parentCtx)
	defer cancelCtx()

	// Subscribe to events
	subscriber, cancel := h.hub.Subscribe(ctx, connULID, userID)
	defer cancel()

	logger.L().Info("WebSocket connection established", "user_id", userID.Hex(), "conn_id", connID)

	sessionTimer := time.AfterFunc(time.Duration(h.maxSessionSec)*time.Second, func() {
		logger.L().Info("WebSocket session timeout", "user_id", userID.Hex(), "conn_id", connID)

		// Send close frame with policy violation code
		err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(WSClosePolicyViolation, "session timeout"))
		if err != nil {
			logger.L().Error("failed to send close message", "error", err, "user_id", userID.Hex(), "conn_id", connID)
		}
		err = c.Close()
		if err != nil {
			logger.L().Error("failed to close WebSocket connection", "error", err, "user_id", userID.Hex(), "conn_id", connID)
		}
		cancelCtx()
	})
	defer func() {
		if sessionTimer != nil {
			sessionTimer.Stop()
		}
	}()

	// Goroutine to handle outgoing messages
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.L().Error("panic in WebSocket sender", "error", r, "user_id", userID.Hex())
			}
		}()

		for {
			select {
			case event, ok := <-subscriber.Ch:
				if !ok {
					// Channel closed, connection should be terminated
					return
				}

				var message map[string]any

				// For deleted events, only send minimal data
				if event.Type == "deleted" {
					message = map[string]any{
						"type": event.Type,
						"note": map[string]any{
							"id": event.Note.ID.Hex(),
						},
					}
				} else {
					// For created/updated events, send full note data
					message = map[string]any{
						"type": event.Type,
						"note": event.Note,
					}
				}

				if err := c.WriteJSON(message); err != nil {
					logger.L().Error("failed to write WebSocket message",
						"error", err,
						"user_id", userID.Hex(),
						"conn_id", connID)
					return
				}

			case <-subscriber.Done:
				// Graceful shutdown signal received
				return

			case <-ctx.Done():
				return
			}
		}
	}()

	// Main loop to handle incoming messages and connection lifecycle
	// Read message from client (we don't expect any, but need to handle ping/pong)
	for {
		messageType, _, err := c.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.L().Error("WebSocket error", "error", err, "user_id", userID.Hex(), "conn_id", connID)
			}
			break
		}

		// Handle ping messages
		if messageType == websocket.PingMessage {
			if err := c.WriteMessage(websocket.PongMessage, nil); err != nil {
				logger.L().Error("failed to send pong", "error", err, "user_id", userID.Hex())
				break
			}
		}
	}

	logger.L().Info("WebSocket connection closed", "user_id", userID.Hex(), "conn_id", connID)
	cancelCtx()
}

// validateJWT validates the JWT token and extracts user information
func (h *WebSocketHandlers) validateJWT(tokenString string) (bson.ObjectID, string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(h.jwtSecret), nil
	})

	if err != nil {
		return bson.ObjectID{}, "", err
	}

	if !token.Valid {
		return bson.ObjectID{}, "", fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return bson.ObjectID{}, "", fmt.Errorf("invalid claims")
	}

	userIDStr, ok := claims["user_id"].(string)
	if !ok {
		return bson.ObjectID{}, "", fmt.Errorf("missing user_id")
	}

	userEmail, ok := claims["email"].(string)
	if !ok {
		return bson.ObjectID{}, "", fmt.Errorf("missing email")
	}

	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		return bson.ObjectID{}, "", fmt.Errorf("invalid user_id: %v", err)
	}

	return userID, userEmail, nil
}

// LogWSConnections logs every WebSocket upgrade attempt.
// It verifies the token with jwtSecret so the logged user_id can't be spoofed.
func LogWSConnections(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {

			// Extract token for logging (without validating)
			token := c.Query("token")
			var userInfo string
			if token != "" {
				// Parse *and* verify the signature so the log can't be spoofed
				parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
					// Accept only the configured algorithm, then hand over the secret
					if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
					}
					return []byte(jwtSecret), nil
				})
				if err == nil && parsed.Valid {
					if claims, ok := parsed.Claims.(jwt.MapClaims); ok {
						if userID, exists := claims["user_id"].(string); exists {
							userInfo = userID
						}
					}
				}
			}

			logger.L().Info("WebSocket upgrade attempt", "ip", c.IP(), "user", userInfo)
		}
		return c.Next()
	}
}
