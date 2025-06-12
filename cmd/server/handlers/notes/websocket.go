package notes

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"note-pulse/cmd/server/ctxkeys"
	"note-pulse/cmd/server/handlers/httperr"
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

	// WebSocket timeout constants
	wsWriteTimeout     = 10 * time.Second // Timeout for writing messages to WebSocket
	wsPingInterval     = 25 * time.Second // Interval for sending ping messages
	wsPingWriteTimeout = 5 * time.Second  // Timeout for writing ping messages

	msgFailedToCloseWebSocketConnection = "failed to close WebSocket connection"
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
		c.Locals(ctxkeys.UserIDKey, userID.Hex())
		c.Locals(ctxkeys.UserEmailKey, userEmail)
		// Use Fiber's request‑bound context so WSNotesStream gets a *real* context.Context.
		c.Locals(ctxkeys.ParentCtxKey, c.UserContext())

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
	conn, parentCtx, err := h.initializeConnection(c)
	if err != nil {
		h.closeConnection(c)
		return
	}

	ctx, cancelCtx := context.WithCancel(parentCtx)
	defer cancelCtx()

	subscriber, cancel := h.hub.Subscribe(ctx, conn.connULID, conn.userID)
	defer cancel()

	logger.L().Info("WebSocket connection established", "user_id", conn.userID.Hex(), "conn_id", conn.connID)

	sessionTimer := h.startSessionTimer(c, conn, cancelCtx)
	defer h.stopSessionTimer(sessionTimer)

	ping := h.startKeepAlive(c, conn)
	defer ping.Stop()

	go h.handleOutgoingMessages(c, conn, subscriber, ctx)

	h.handleIncomingMessages(c, conn)

	logger.L().Info("WebSocket connection closed", "user_id", conn.userID.Hex(), "conn_id", conn.connID)
	cancelCtx()
}

// wsConnection holds connection-specific data
type wsConnection struct {
	userID   bson.ObjectID
	connULID ulid.ULID
	connID   string
}

// initializeConnection validates and sets up the WebSocket connection
func (h *WebSocketHandlers) initializeConnection(c *websocket.Conn) (*wsConnection, context.Context, error) {
	userIDStr, ok := c.Locals(ctxkeys.UserIDKey).(string)
	if !ok {
		logger.L().Error(ctxkeys.UserIDKey + " not found in WebSocket context")
		return nil, nil, fmt.Errorf(ctxkeys.UserIDKey + " not found")
	}

	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		logger.L().Error("invalid "+ctxkeys.UserIDKey+" in WebSocket context", ctxkeys.UserIDKey, userIDStr, "error", err)
		return nil, nil, fmt.Errorf("invalid %s: %w", ctxkeys.UserIDKey, err)
	}

	parentCtx, ok := c.Locals(ctxkeys.ParentCtxKey).(context.Context)
	if !ok {
		logger.L().Error(ctxkeys.ParentCtxKey + " not found in WebSocket context")
		return nil, nil, fmt.Errorf(ctxkeys.ParentCtxKey + " not found")
	}

	connULID := ulid.MustNew(ulid.Timestamp(time.Now().UTC()), rand.Reader)
	connID := connULID.String()

	conn := &wsConnection{
		userID:   userID,
		connULID: connULID,
		connID:   connID,
	}

	return conn, parentCtx, nil
}

// closeConnection safely closes the WebSocket connection
func (h *WebSocketHandlers) closeConnection(c *websocket.Conn) {
	if err := c.Close(); err != nil {
		logger.L().Error(msgFailedToCloseWebSocketConnection, "error", err)
	}
}

// startSessionTimer creates and starts the session timeout timer
func (h *WebSocketHandlers) startSessionTimer(c *websocket.Conn, conn *wsConnection, cancelCtx context.CancelFunc) *time.Timer {
	return time.AfterFunc(time.Duration(h.maxSessionSec)*time.Second, func() {
		logger.L().Info("WebSocket session timeout", "user_id", conn.userID.Hex(), "conn_id", conn.connID)
		h.sendCloseMessage(c, conn)
		h.closeConnection(c)
		cancelCtx()
	})
}

// stopSessionTimer safely stops the session timer
func (h *WebSocketHandlers) stopSessionTimer(timer *time.Timer) {
	if timer != nil {
		timer.Stop()
	}
}

// sendCloseMessage sends a close frame to the client
func (h *WebSocketHandlers) sendCloseMessage(c *websocket.Conn, conn *wsConnection) {
	err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(WSClosePolicyViolation, "session timeout"))
	if err != nil {
		logger.L().Error("failed to send close message", "error", err, "user_id", conn.userID.Hex(), "conn_id", conn.connID)
	}
}

// startKeepAlive starts the keep-alive ping mechanism
func (h *WebSocketHandlers) startKeepAlive(c *websocket.Conn, conn *wsConnection) *time.Ticker {
	ping := time.NewTicker(wsPingInterval)
	go func() {
		for range ping.C {
			if h.sendPing(c, conn) != nil {
				return
			}
		}
	}()
	return ping
}

// sendPing sends a ping message to the client
func (h *WebSocketHandlers) sendPing(c *websocket.Conn, conn *wsConnection) error {
	if err := c.SetWriteDeadline(time.Now().Add(wsPingWriteTimeout)); err != nil {
		logger.L().Error("failed to set write deadline", "error", err, "user_id", conn.userID.Hex(), "conn_id", conn.connID)
		return err
	}
	if err := c.WriteMessage(websocket.PingMessage, nil); err != nil {
		logger.L().Warn("failed to write ping message", "error", err, "user_id", conn.userID.Hex(), "conn_id", conn.connID)
		return err
	}
	return nil
}

// handleOutgoingMessages handles messages sent to the client
func (h *WebSocketHandlers) handleOutgoingMessages(c *websocket.Conn, conn *wsConnection, subscriber *notes.Subscriber, ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logger.L().Error("panic in WebSocket sender", "error", r, "user_id", conn.userID.Hex())
		}
	}()

	for {
		select {
		case event, ok := <-subscriber.Ch:
			if !ok {
				return
			}
			if h.sendEvent(c, conn, event) != nil {
				return
			}
		case <-subscriber.Done:
			return
		case <-ctx.Done():
			return
		}
	}
}

// sendEvent sends an event to the client
func (h *WebSocketHandlers) sendEvent(c *websocket.Conn, conn *wsConnection, event notes.NoteEvent) error {
	message := h.buildEventMessage(event)

	if err := c.SetWriteDeadline(time.Now().Add(wsWriteTimeout)); err != nil {
		logger.L().Error("failed to set write deadline", "error", err, "user_id", conn.userID.Hex(), "conn_id", conn.connID)
		return err
	}
	if err := c.WriteJSON(message); err != nil {
		logger.L().Error("failed to write WebSocket message", "error", err, "user_id", conn.userID.Hex(), "conn_id", conn.connID)
		return err
	}
	return nil
}

// buildEventMessage builds the message payload for an event
func (h *WebSocketHandlers) buildEventMessage(event notes.NoteEvent) map[string]any {
	if event.Type == "deleted" {
		return map[string]any{
			"type": event.Type,
			"note": map[string]any{
				"id": event.Note.ID.Hex(),
			},
		}
	}
	return map[string]any{
		"type": event.Type,
		"note": event.Note,
	}
}

// handleIncomingMessages handles messages received from the client
func (h *WebSocketHandlers) handleIncomingMessages(c *websocket.Conn, conn *wsConnection) {
	for {
		messageType, _, err := c.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.L().Error("WebSocket error", "error", err, "user_id", conn.userID.Hex(), "conn_id", conn.connID)
			}
			break
		}

		if messageType == websocket.PingMessage {
			if h.sendPong(c, conn) != nil {
				break
			}
		}
	}
}

// sendPong sends a pong message in response to a ping
func (h *WebSocketHandlers) sendPong(c *websocket.Conn, conn *wsConnection) error {
	if err := c.WriteMessage(websocket.PongMessage, nil); err != nil {
		logger.L().Error("failed to send pong", "error", err, "user_id", conn.userID.Hex())
		return err
	}
	return nil
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
		return bson.ObjectID{}, "", fmt.Errorf("invalid user_id: %w", err)
	}

	return userID, userEmail, nil
}

// LogWSConnections logs every WebSocket upgrade attempt.
// It verifies the token with jwtSecret so the logged user_id can't be spoofed.
func LogWSConnections(jwtSecret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			token := c.Query("token")
			userInfo := extractUserIDFromToken(token, jwtSecret)
			logger.L().Info("WebSocket upgrade attempt", "ip", c.IP(), "user", userInfo)
		}
		return c.Next()
	}
}

// extractUserIDFromToken extracts and validates user ID from JWT token
func extractUserIDFromToken(token, jwtSecret string) string {
	if token == "" {
		return ""
	}

	parsed, err := parseAndValidateToken(token, jwtSecret)
	if err != nil || !parsed.Valid {
		return ""
	}

	return getUserIDFromClaims(parsed.Claims)
}

// parseAndValidateToken parses JWT token and validates signature
func parseAndValidateToken(token, jwtSecret string) (*jwt.Token, error) {
	return jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if !isValidSigningMethod(t) {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})
}

// isValidSigningMethod checks if the JWT uses HMAC signing method
func isValidSigningMethod(token *jwt.Token) bool {
	_, ok := token.Method.(*jwt.SigningMethodHMAC)
	return ok
}

// getUserIDFromClaims extracts user_id from JWT claims
func getUserIDFromClaims(claims jwt.Claims) string {
	mapClaims, ok := claims.(jwt.MapClaims)
	if !ok {
		return ""
	}

	userID, exists := mapClaims["user_id"].(string)
	if !exists {
		return ""
	}

	return userID
}
