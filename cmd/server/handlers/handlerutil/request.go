package handlerutil

import (
	"errors"
	"note-pulse/cmd/server/handlers/httperr"
	"note-pulse/internal/logger"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func NotFoundError(err error) error {
	return httperr.Fail(httperr.E{
		Status:  404,
		Message: err.Error(),
	})
}

// GetUserID extracts user ID from fiber context
func GetUserID(c *fiber.Ctx) (bson.ObjectID, error) {
	userIDStr, ok := c.Locals("userID").(string)
	if !ok {
		logger.L().Error("user ID not found in context", "handler", "getUserID", "path", c.Path())
		return bson.ObjectID{}, httperr.Fail(httperr.ErrUnauthorized)
	}

	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		logger.L().Error("invalid user ID", "handler", "getUserID", "userIDStr", userIDStr, "path", c.Path(), "error", err)
		return bson.ObjectID{}, httperr.Fail(httperr.ErrUnauthorized)
	}

	return userID, nil
}

// ParseAndValidateBody parses request body and validates it
func ParseAndValidateBody(c *fiber.Ctx, req any, validator *validator.Validate, handlerName string) error {
	userID, _ := GetUserID(c)
	userIDHex := userID.Hex()

	if err := c.BodyParser(req); err != nil {
		logger.L().Warn("failed to parse request body", "handler", handlerName, "userID", userIDHex, "error", err)
		return httperr.Fail(httperr.ErrBadRequest)
	}

	if err := validator.Struct(req); err != nil {
		logger.L().Warn("request validation failed", "handler", handlerName, "userID", userIDHex, "error", err)
		return httperr.InvalidInput(err)
	}

	return nil
}

// ParseAndValidateQuery parses query parameters and validates them
func ParseAndValidateQuery(c *fiber.Ctx, req any, validator *validator.Validate, handlerName string) error {
	userID, _ := GetUserID(c)
	userIDHex := userID.Hex()

	if err := c.QueryParser(req); err != nil {
		logger.L().Warn("failed to parse query params", "handler", handlerName, "userID", userIDHex, "error", err)
		return httperr.Fail(httperr.ErrBadRequest)
	}

	if err := validator.Struct(req); err != nil {
		logger.L().Warn("query validation failed", "handler", handlerName, "userID", userIDHex, "error", err)
		return httperr.InvalidInput(err)
	}

	return nil
}

// ExtractNoteID extracts and validates note ID from URL parameter
func ExtractNoteID(c *fiber.Ctx, userID bson.ObjectID, handlerName string, notFoundErr error) (bson.ObjectID, error) {
	noteIDStr := c.Params("id")
	if noteIDStr == "" {
		logger.L().Warn("missing note ID parameter", "handler", handlerName, "userID", userID.Hex(), "path", c.Path())
		return bson.ObjectID{}, NotFoundError(notFoundErr)
	}

	noteID, err := bson.ObjectIDFromHex(noteIDStr)
	if err != nil {
		logger.L().Warn("invalid note ID parameter", "handler", handlerName, "userID", userID.Hex(), "noteIDStr", noteIDStr, "error", err)
		return bson.ObjectID{}, NotFoundError(notFoundErr)
	}

	return noteID, nil
}

// HandleServiceError handles common service error responses
func HandleServiceError(err error, handlerName string, userID bson.ObjectID, noteID *bson.ObjectID, notFoundErr error) error {
	userIDHex := userID.Hex()
	logFields := []any{"handler", handlerName, "userID", userIDHex, "error", err}

	if noteID != nil {
		logFields = append(logFields, "noteID", noteID.Hex())
	}

	if errors.Is(err, notFoundErr) {
		logger.L().Info("resource not found", logFields...)
		return NotFoundError(notFoundErr)
	}

	logger.L().Error("service operation failed", logFields...)
	return httperr.Fail(httperr.E{
		Status:  500,
		Message: err.Error(),
	})
}
