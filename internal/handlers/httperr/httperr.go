package httperr

import (
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

// New creates a new HTTP error with the given status code and message
func New(status int, message string) E {
	return E{Status: status, Message: message}
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

// Pre-defined HTTP errors
var (
	ErrBadRequest      = E{Status: 400, Message: "Bad Request"}
	ErrUnauthorized    = E{Status: 401, Message: "Unauthorized"}
	ErrTooManyRequests = E{Status: 429, Message: "Too Many Requests"}
	ErrInternal        = E{Status: 500, Message: "Internal Server Error"}
)

// Handler is the global error handler for Fiber
func Handler(c *fiber.Ctx, err error) error {
	// Check if it's our custom error type
	if e, ok := err.(E); ok {
		return e.JSON(c)
	}

	if e, ok := err.(*fiber.Error); ok {
		return c.Status(e.Code).JSON(E{
			Status:  e.Code,
			Message: e.Message,
		})
	}

	return ErrInternal.JSON(c)
}
