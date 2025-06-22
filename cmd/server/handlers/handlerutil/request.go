package handlerutil

import (
	"errors"
	"note-pulse/cmd/server/ctxkeys"
	"note-pulse/cmd/server/handlers/httperr"
	"note-pulse/internal/logger"
	util "note-pulse/internal/utils"

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

func unauthorizedError() (bson.ObjectID, error) {
	return bson.ObjectID{}, httperr.Fail(httperr.ErrUnauthorized)
}

// GetUserID extracts user ID from fiber context
func GetUserID(c *fiber.Ctx) (bson.ObjectID, error) {
	userIDStr, ok := c.Locals(ctxkeys.UserIDKey).(string)
	if !ok {
		logger.L().Error("user ID not found in context", "handler", "getUserID", "path", c.Path())
		return unauthorizedError()
	}

	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		logger.L().Error("invalid user ID", "handler", "getUserID", "userIDStr", userIDStr, "path", c.Path(), "error", err)
		return unauthorizedError()
	}

	return userID, nil
}

// ParseAndValidateBody parses request body and validates it
func ParseAndValidateBody(c *fiber.Ctx, req any, v *validator.Validate, handlerName string) error {
	uidHex := "unknown"
	if uid, err := GetUserID(c); err == nil {
		uidHex = uid.Hex()
	}

	if err := c.BodyParser(req); err != nil {
		logger.L().Info("failed to parse request body", "handler", handlerName, ctxkeys.UserIDKey, uidHex, "error", err)
		return httperr.Fail(httperr.ErrBadRequest)
	}

	if err := util.ValidateCtx(c.Context(), v, req); err != nil {
		logger.L().Info("request validation failed", "handler", handlerName, ctxkeys.UserIDKey, uidHex, "error", err)
		return httperr.InvalidInput(err)
	}

	return nil
}

// ParseAndValidateQuery parses query parameters and validates them
func ParseAndValidateQuery(c *fiber.Ctx, req any, v *validator.Validate, handlerName string) error {
	uidHex := "unknown"
	if uid, err := GetUserID(c); err == nil {
		uidHex = uid.Hex()
	}

	if err := c.QueryParser(req); err != nil {
		logger.L().Info("failed to parse query params", "handler", handlerName, ctxkeys.UserIDKey, uidHex, "error", err)
		return httperr.Fail(httperr.ErrBadRequest)
	}

	if err := util.ValidateCtx(c.Context(), v, req); err != nil {
		logger.L().Info("query validation failed", "handler", handlerName, ctxkeys.UserIDKey, uidHex, "error", err)
		return httperr.InvalidInput(err)
	}

	return nil
}

// ExtractNoteID extracts and validates note ID from URL parameter
func ExtractNoteID(c *fiber.Ctx, userID bson.ObjectID, handlerName string) (bson.ObjectID, error) {
	noteIDStr := c.Params("id")
	if noteIDStr == "" {
		logger.L().Info("missing note ID parameter", "handler", handlerName, ctxkeys.UserIDKey, userID.Hex(), "path", c.Path())
		return bson.ObjectID{}, httperr.Fail(httperr.ErrBadRequest)
	}

	noteID, err := bson.ObjectIDFromHex(noteIDStr)
	if err != nil {
		logger.L().Info("invalid note ID parameter", "handler", handlerName, ctxkeys.UserIDKey, userID.Hex(), "noteIDStr", noteIDStr, "error", err)
		return bson.ObjectID{}, httperr.Fail(httperr.ErrBadRequest)
	}

	return noteID, nil
}

// HandleServiceError handles common service error responses
func HandleServiceError(err error, handlerName string, userID bson.ObjectID, noteID *bson.ObjectID, notFoundErr error) error {
	userIDHex := userID.Hex()
	logFields := []any{"handler", handlerName, ctxkeys.UserIDKey, userIDHex, "error", err}

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
