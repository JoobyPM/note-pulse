package auth

import (
	"context"
	"errors"

	"note-pulse/cmd/server/handlers/httperr"
	"note-pulse/internal/logger"
	"note-pulse/internal/services/auth"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// AuthService defines the interface for auth service
type AuthService interface {
	SignUp(ctx context.Context, req auth.SignUpRequest) (*auth.AuthResponse, error)
	SignIn(ctx context.Context, req auth.SignInRequest) (*auth.AuthResponse, error)
	Refresh(ctx context.Context, rawRefreshToken string) (*auth.AuthResponse, error)
	SignOut(ctx context.Context, userID bson.ObjectID, rawRefreshToken string) error
	SignOutAll(ctx context.Context, userID bson.ObjectID) error
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
		logger.L().Warn("failed to parse signup request body", "handler", "SignUp", "error", err)
		return httperr.Fail(httperr.ErrBadRequest)
	}

	if err := h.validator.Struct(req); err != nil {
		logger.L().Warn("signup request validation failed", "handler", "SignUp", "error", err)
		return httperr.InvalidInput(err)
	}

	resp, err := h.authService.SignUp(c.Context(), req)
	if err != nil {
		logger.L().Error("signup service failed", "handler", "SignUp", "email", req.Email, "error", err)
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
		logger.L().Warn("failed to parse signin request body", "handler", "SignIn", "error", err)
		return httperr.Fail(httperr.ErrBadRequest)
	}

	if err := h.validator.Struct(req); err != nil {
		logger.L().Warn("signin request validation failed", "handler", "SignIn", "error", err)
		return httperr.InvalidInput(err)
	}

	resp, err := h.authService.SignIn(c.Context(), req)
	if err != nil {
		logger.L().Error("signin service failed", "handler", "SignIn", "email", req.Email, "error", err)
		return httperr.Fail(httperr.E{
			Status:  401,
			Message: err.Error(),
		})
	}

	return c.JSON(resp)
}

// Refresh handles token refresh requests
// @Summary Refresh access token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.RefreshRequest true "Refresh token request"
// @Success 200 {object} auth.AuthResponse
// @Failure 400 {object} httperr.E
// @Failure 401 {object} httperr.E
// @Router /auth/refresh [post]
func (h *Handlers) Refresh(c *fiber.Ctx) error {
	var req auth.RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		logger.L().Warn("failed to parse refresh request body", "handler", "Refresh", "error", err)
		return httperr.Fail(httperr.ErrBadRequest)
	}

	if err := h.validator.Struct(&req); err != nil {
		logger.L().Warn("refresh request validation failed", "handler", "Refresh", "error", err)
		return httperr.InvalidInput(err)
	}

	resp, err := h.authService.Refresh(c.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidRefreshToken) {
			logger.L().Info("invalid refresh token reuse detected", "remote", c.IP(), "error", err)
			return httperr.Fail(httperr.ErrUnauthorized)
		}
		logger.L().Error("refresh service failed", "handler", "Refresh", "error", err)
		return httperr.Fail(httperr.E{
			Status:  401,
			Message: err.Error(),
		})
	}

	return c.JSON(resp)
}

// SignOut handles user sign out requests
// @Summary Sign out a user
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.SignOutRequest true "Sign out request"
// @Success 200 {object} map[string]string
// @Failure 400 {object} httperr.E
// @Failure 401 {object} httperr.E
// @Router /auth/sign-out [post]
func (h *Handlers) SignOut(c *fiber.Ctx) error {
	// Extract user ID from JWT token context
	userIDStr := c.Locals("userID")
	if userIDStr == nil {
		logger.L().Warn("missing user ID in token context", "handler", "SignOut")
		return httperr.Fail(httperr.ErrUserNotAuthenticated)
	}

	userID, err := bson.ObjectIDFromHex(userIDStr.(string))
	if err != nil {
		logger.L().Warn("invalid user ID format", "handler", "SignOut", "userID", userIDStr, "error", err)
		return httperr.Fail(httperr.ErrInvalidUserID)
	}

	var req auth.SignOutRequest
	if err := c.BodyParser(&req); err != nil {
		logger.L().Warn("failed to parse signout request body", "handler", "SignOut", "error", err)
		return httperr.Fail(httperr.ErrBadRequest)
	}

	if err := h.validator.Struct(&req); err != nil {
		logger.L().Warn("signout request validation failed", "handler", "SignOut", "error", err)
		return httperr.InvalidInput(err)
	}

	if err := h.authService.SignOut(c.Context(), userID, req.RefreshToken); err != nil {
		if errors.Is(err, auth.ErrInvalidRefreshToken) {
			return httperr.Fail(httperr.ErrUnauthorized)
		}
		logger.L().Error("signout service failed", "handler", "SignOut", "userID", userID.Hex(), "error", err)
		return httperr.Fail(httperr.ErrInternal)
	}

	return c.JSON(map[string]string{"message": "Successfully signed out"})
}

// SignOutAll handles user sign out from all devices
// @Summary Sign out from all devices
// @Tags auth
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 401 {object} httperr.E
// @Failure 500 {object} httperr.E
// @Router /auth/sign-out-all [post]
func (h *Handlers) SignOutAll(c *fiber.Ctx) error {
	// Extract user ID from JWT token context
	userIDStr := c.Locals("userID")
	if userIDStr == nil {
		logger.L().Warn("missing user ID in token context", "handler", "SignOutAll")
		return httperr.Fail(httperr.ErrUserNotAuthenticated)
	}

	userID, err := bson.ObjectIDFromHex(userIDStr.(string))
	if err != nil {
		logger.L().Warn("invalid user ID format", "handler", "SignOutAll", "userID", userIDStr, "error", err)
		return httperr.Fail(httperr.ErrInvalidUserID)
	}

	if err := h.authService.SignOutAll(c.Context(), userID); err != nil {
		logger.L().Error("signout all service failed", "handler", "SignOutAll", "userID", userID.Hex(), "error", err)
		return httperr.Fail(httperr.InternalError(err.Error()))
	}

	return c.JSON(map[string]string{"message": "Signed out everywhere"})
}
