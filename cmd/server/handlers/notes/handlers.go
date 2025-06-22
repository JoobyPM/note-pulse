package notes

import (
	"context"
	"errors"
	"note-pulse/cmd/server/handlers/handlerutil"
	"note-pulse/cmd/server/handlers/httperr"
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
	userID, err := handlerutil.GetUserID(c)
	if err != nil {
		return err
	}

	var req notes.CreateNoteRequest
	if err := handlerutil.ParseAndValidateBody(c, &req, h.validator, "Create"); err != nil {
		return err
	}

	resp, err := h.service.Create(c.Context(), userID, req)
	if err != nil {
		return handlerutil.HandleServiceError(err, "Create", userID, nil, notes.ErrNoteNotFound)
	}

	return c.Status(201).JSON(resp)
}

// List handles notes listing with pagination
// @Summary List notes with cursor-based pagination, search, filtering and sorting
// @Tags notes
// @Accept json
// @Produce json
// @Security Bearer
// @Param limit query int false "Limit (default: 50, max: 100)" minimum(1) maximum(100)
// @Param cursor query string false "Cursor for pagination. Cannot be used with offset or anchor."
// @Param anchor query string false "Centre the window on this note id. Cannot be used with offset or cursor."
// @Param span query int false "How many notes to return (default:limit)" minimum(1) maximum(100)
// @Param q query string false "Full-text search in title or body"
// @Param color query string false "Hex color filter (#RRGGBB)"
// @Param sort query string false "Sort field: created_at|updated_at|title"
// @Param order query string false "asc|desc (default desc)"
// @Param offset query int false "Offset for absolute positioning (0-50,000). Cannot be used with cursor or anchor." minimum(0) maximum(50000)
// @Success 200 {object} notes.ListNotesResponse
// @Failure 400 {object} httperr.E
// @Failure 401 {object} httperr.E
// @Failure 416 {object} httperr.E
// @Router /notes [get]
func (h *Handlers) List(c *fiber.Ctx) error {
	userID, err := handlerutil.GetUserID(c)
	if err != nil {
		return err
	}

	var req notes.ListNotesRequest
	if err := handlerutil.ParseAndValidateQuery(c, &req, h.validator, "List"); err != nil {
		return err
	}

	resp, err := h.service.List(c.Context(), userID, req)
	if err != nil {
		if errors.Is(err, notes.ErrBadRequest) {
			c.Locals("log_level", "info")
			return httperr.Fail(httperr.ErrBadRequest)
		}
		if errors.Is(err, notes.ErrOffsetBeyondTotal) {
			c.Locals("log_level", "info")
			return httperr.Fail(httperr.ErrRequestedRangeNotSatisfiable)
		}
		return handlerutil.HandleServiceError(err, "List", userID, nil, notes.ErrNoteNotFound)
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
	userID, err := handlerutil.GetUserID(c)
	if err != nil {
		return err
	}

	noteID, err := handlerutil.ExtractNoteID(c, userID, "Update")
	if err != nil {
		return err
	}

	var req notes.UpdateNoteRequest
	if err := handlerutil.ParseAndValidateBody(c, &req, h.validator, "Update"); err != nil {
		return err
	}

	resp, err := h.service.Update(c.Context(), userID, noteID, req)
	if err != nil {
		return handlerutil.HandleServiceError(err, "Update", userID, &noteID, notes.ErrNoteNotFound)
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
	userID, err := handlerutil.GetUserID(c)
	if err != nil {
		return err
	}

	noteID, err := handlerutil.ExtractNoteID(c, userID, "Delete")
	if err != nil {
		return err
	}

	err = h.service.Delete(c.Context(), userID, noteID)
	if err != nil {
		return handlerutil.HandleServiceError(err, "Delete", userID, &noteID, notes.ErrNoteNotFound)
	}

	return c.SendStatus(204)
}
