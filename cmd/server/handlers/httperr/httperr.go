package httperr

import (
	"errors"

	"github.com/gofiber/fiber/v2"
)

// E represents an HTTP error with status code and message
type E struct {
	Status  int    `json:"-" example:"400"`
	Message string `json:"error" example:"Bad Request"`
}

// Error implements the error interface
func (e E) Error() string {
	return e.Message
}

// JSON returns the error as JSON response
func (e E) JSON(c *fiber.Ctx) error {
	return c.Status(e.Status).JSON(e)
}

// Fail returns the error for Fiber's global error handler to process
func Fail(err E) error {
	return err
}

// InvalidInput wraps a validation error and returns the standard response.
func InvalidInput(err error) error {
	return Fail(E{
		Status:  400,
		Message: "Invalid input: " + err.Error(),
	})
}

// InternalError returns an internal server error with the given message
func InternalError(message string) E {
	return E{Status: 500, Message: message}
}

// Pre-defined HTTP errors
var (
	ErrBadRequest           = E{Status: 400, Message: "Bad Request"}
	ErrInvalidUserID        = E{Status: 400, Message: "Invalid user ID"}
	ErrUnauthorized         = E{Status: 401, Message: "Unauthorized"}
	ErrUserNotAuthenticated = E{Status: 401, Message: "User not authenticated"}
	ErrTooManyRequests      = E{Status: 429, Message: "Too Many Requests"}
	ErrInternal             = InternalError("Internal Server Error")
)

// Handler is the global error handler for Fiber
func Handler(c *fiber.Ctx, err error) error {
	// Check if it's our custom error type
	var e E
	if errors.As(err, &e) {
		return e.JSON(c)
	}

	var fiberError *fiber.Error
	if errors.As(err, &fiberError) {
		return c.Status(fiberError.Code).JSON(E{
			Status:  fiberError.Code,
			Message: fiberError.Message,
		})
	}

	return ErrInternal.JSON(c)
}
