package notes

import (
	"context"
	"errors"

	"note-pulse/cmd/server/handlers/httperr"
	"note-pulse/internal/logger"
	"note-pulse/internal/services/notes"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Service defines the interface for notes service
type Service interface {
	Create(ctx context.Context, userID bson.ObjectID, req notes.CreateNoteRequest) (*notes.NoteResponse, error)
	List(ctx context.Context, userID bson.ObjectID, req notes.ListNotesRequest) (*notes.ListNotesResponse, error)
	Update(ctx context.Context, userID, noteID bson.ObjectID, req notes.UpdateNoteRequest) (*notes.NoteResponse, error)
	Delete(ctx context.Context, userID, noteID bson.ObjectID) error
}

// Handlers contains the notes HTTP handlers
type Handlers struct {
	service   Service
	validator *validator.Validate
}

// NewHandlers creates new notes handlers
func NewHandlers(service Service, validator *validator.Validate) *Handlers {
	return &Handlers{
		service:   service,
		validator: validator,
	}
}

// getUserID extracts user ID from fiber context
func (h *Handlers) getUserID(c *fiber.Ctx) (bson.ObjectID, error) {
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

// Create handles note creation
// @Summary Create a new note
// @Tags notes
// @Accept json
// @Produce json
// @Security Bearer
// @Param request body notes.CreateNoteRequest true "Create note request"
// @Success 201 {object} notes.NoteResponse
// @Failure 400 {object} httperr.E
// @Failure 401 {object} httperr.E
// @Router /notes [post]
func (h *Handlers) Create(c *fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	var req notes.CreateNoteRequest
	if err := c.BodyParser(&req); err != nil {
		logger.L().Warn("failed to parse create note request body", "handler", "Create", "userID", userID.Hex(), "error", err)
		return httperr.Fail(httperr.ErrBadRequest)
	}

	if err := h.validator.Struct(req); err != nil {
		logger.L().Warn("create note request validation failed", "handler", "Create", "userID", userID.Hex(), "error", err)
		return httperr.InvalidInput(err)
	}

	resp, err := h.service.Create(c.Context(), userID, req)
	if err != nil {
		logger.L().Error("create note service failed", "handler", "Create", "userID", userID.Hex(), "error", err)
		return httperr.Fail(httperr.E{
			Status:  500,
			Message: err.Error(),
		})
	}

	return c.Status(201).JSON(resp)
}

// List handles notes listing with pagination
// @Summary List notes with cursor-based pagination
// @Tags notes
// @Accept json
// @Produce json
// @Security Bearer
// @Param limit query int false "Limit (default: 50, max: 100)" minimum(1) maximum(100)
// @Param cursor query string false "Cursor for pagination"
// @Success 200 {object} notes.ListNotesResponse
// @Failure 400 {object} httperr.E
// @Failure 401 {object} httperr.E
// @Router /notes [get]
func (h *Handlers) List(c *fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	var req notes.ListNotesRequest
	if err := c.QueryParser(&req); err != nil {
		logger.L().Warn("failed to parse list notes query params", "handler", "List", "userID", userID.Hex(), "error", err)
		return httperr.Fail(httperr.ErrBadRequest)
	}

	if err := h.validator.Struct(req); err != nil {
		logger.L().Warn("list notes request validation failed", "handler", "List", "userID", userID.Hex(), "error", err)
		return httperr.InvalidInput(err)
	}

	resp, err := h.service.List(c.Context(), userID, req)
	if err != nil {
		logger.L().Error("list notes service failed", "handler", "List", "userID", userID.Hex(), "error", err)
		return httperr.Fail(httperr.E{
			Status:  500,
			Message: err.Error(),
		})
	}

	return c.JSON(resp)
}

// Update handles note updates
// @Summary Update a note
// @Tags notes
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Note ID"
// @Param request body notes.UpdateNoteRequest true "Update note request"
// @Success 200 {object} notes.NoteResponse
// @Failure 400 {object} httperr.E
// @Failure 401 {object} httperr.E
// @Router /notes/{id} [patch]
func (h *Handlers) Update(c *fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	noteIDStr := c.Params("id")
	if noteIDStr == "" {
		logger.L().Warn("missing note ID parameter", "handler", "Update", "userID", userID.Hex(), "path", c.Path())
		return httperr.Fail(httperr.E{
			Status:  400,
			Message: notes.ErrNoteNotFound.Error(),
		})
	}

	noteID, err := bson.ObjectIDFromHex(noteIDStr)
	if err != nil {
		logger.L().Warn("invalid note ID parameter", "handler", "Update", "userID", userID.Hex(), "noteIDStr", noteIDStr, "error", err)
		return httperr.Fail(httperr.E{
			Status:  400,
			Message: notes.ErrNoteNotFound.Error(),
		})
	}

	var req notes.UpdateNoteRequest
	if err := c.BodyParser(&req); err != nil {
		logger.L().Warn("failed to parse update note request body", "handler", "Update", "userID", userID.Hex(), "noteID", noteID.Hex(), "error", err)
		return httperr.Fail(httperr.ErrBadRequest)
	}

	if err := h.validator.Struct(req); err != nil {
		logger.L().Warn("update note request validation failed", "handler", "Update", "userID", userID.Hex(), "noteID", noteID.Hex(), "error", err)
		return httperr.InvalidInput(err)
	}

	resp, err := h.service.Update(c.Context(), userID, noteID, req)
	if err != nil {
		if errors.Is(err, notes.ErrNoteNotFound) {
			logger.L().Info("note not found for update", "handler", "Update", "userID", userID.Hex(), "noteID", noteID.Hex())
			return httperr.Fail(httperr.E{
				Status:  400,
				Message: notes.ErrNoteNotFound.Error(),
			})
		}
		logger.L().Error("update note service failed", "handler", "Update", "userID", userID.Hex(), "noteID", noteID.Hex(), "error", err)
		return httperr.Fail(httperr.E{
			Status:  500,
			Message: err.Error(),
		})
	}

	return c.JSON(resp)
}

// Delete handles note deletion
// @Summary Delete a note
// @Tags notes
// @Accept json
// @Produce json
// @Security Bearer
// @Param id path string true "Note ID"
// @Success 204
// @Failure 400 {object} httperr.E
// @Failure 401 {object} httperr.E
// @Router /notes/{id} [delete]
func (h *Handlers) Delete(c *fiber.Ctx) error {
	userID, err := h.getUserID(c)
	if err != nil {
		return err
	}

	noteIDStr := c.Params("id")
	if noteIDStr == "" {
		logger.L().Warn("missing note ID parameter", "handler", "Delete", "userID", userID.Hex(), "path", c.Path())
		return httperr.Fail(httperr.E{
			Status:  400,
			Message: notes.ErrNoteNotFound.Error(),
		})
	}

	noteID, err := bson.ObjectIDFromHex(noteIDStr)
	if err != nil {
		logger.L().Warn("invalid note ID parameter", "handler", "Delete", "userID", userID.Hex(), "noteIDStr", noteIDStr, "error", err)
		return httperr.Fail(httperr.E{
			Status:  400,
			Message: notes.ErrNoteNotFound.Error(),
		})
	}

	err = h.service.Delete(c.Context(), userID, noteID)
	if err != nil {
		if errors.Is(err, notes.ErrNoteNotFound) {
			logger.L().Info("note not found for delete", "handler", "Delete", "userID", userID.Hex(), "noteID", noteID.Hex())
			return httperr.Fail(httperr.E{
				Status:  400,
				Message: notes.ErrNoteNotFound.Error(),
			})
		}
		logger.L().Error("delete note service failed", "handler", "Delete", "userID", userID.Hex(), "noteID", noteID.Hex(), "error", err)
		return httperr.Fail(httperr.E{
			Status:  500,
			Message: err.Error(),
		})
	}

	return c.SendStatus(204)
}
