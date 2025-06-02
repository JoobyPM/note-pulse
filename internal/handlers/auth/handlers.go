package auth

import (
	"context"

	"note-pulse/internal/handlers/httperr"
	"note-pulse/internal/services/auth"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

// AuthService defines the interface for auth service
type AuthService interface {
	SignUp(ctx context.Context, req auth.SignUpRequest) (*auth.AuthResponse, error)
	SignIn(ctx context.Context, req auth.SignInRequest) (*auth.AuthResponse, error)
}

// Handlers contains the auth HTTP handlers
type Handlers struct {
	authService AuthService
	validator   *validator.Validate
}

// NewHandlers creates new auth handlers
func NewHandlers(authService AuthService, validator *validator.Validate) *Handlers {
	return &Handlers{
		authService: authService,
		validator:   validator,
	}
}

// SignUp handles user registration
// @Summary Register a new user
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.SignUpRequest true "Sign up request"
// @Success 201 {object} auth.SignUpResponse
// @Failure 400 {object} httperr.E
// @Router /auth/sign-up [post]
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
// @Summary Authenticate a user
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.SignInRequest true "Sign in request"
// @Success 200 {object} auth.SignInResponse
// @Failure 400 {object} httperr.E
// @Failure 401 {object} httperr.E
// @Failure 429 {object} httperr.E
// @Router /auth/sign-in [post]
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
