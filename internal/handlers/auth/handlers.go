package auth

import (
	"note-pulse/internal/handlers/httperr"
	"note-pulse/internal/services/auth"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

// Handlers contains the auth HTTP handlers
type Handlers struct {
	authService *auth.Service
	validator   *validator.Validate
}

// NewHandlers creates new auth handlers
func NewHandlers(authService *auth.Service, validator *validator.Validate) *Handlers {
	return &Handlers{
		authService: authService,
		validator:   validator,
	}
}

// SignUp handles user registration
func (h *Handlers) SignUp(c *fiber.Ctx) error {
	var req auth.SignUpRequest
	if err := c.BodyParser(&req); err != nil {
		return httperr.Fail(httperr.ErrBadRequest)
	}

	if err := h.validator.Struct(req); err != nil {
		return httperr.Fail(httperr.E{
			Status:  400,
			Message: "Invalid input: " + err.Error(),
		})
	}

	resp, err := h.authService.SignUp(c.Context(), req)
	if err != nil {
		return httperr.Fail(httperr.E{
			Status:  400,
			Message: err.Error(),
		})
	}

	return c.Status(201).JSON(resp)
}

// SignIn handles user authentication
func (h *Handlers) SignIn(c *fiber.Ctx) error {
	var req auth.SignInRequest
	if err := c.BodyParser(&req); err != nil {
		return httperr.Fail(httperr.ErrBadRequest)
	}

	if err := h.validator.Struct(req); err != nil {
		return httperr.Fail(httperr.E{
			Status:  400,
			Message: "Invalid input: " + err.Error(),
		})
	}

	resp, err := h.authService.SignIn(c.Context(), req)
	if err != nil {
		return httperr.Fail(httperr.E{
			Status:  401,
			Message: err.Error(),
		})
	}

	return c.JSON(resp)
}
