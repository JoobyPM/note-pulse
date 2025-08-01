package handlers

import (
	"note-pulse/cmd/server/ctxkeys"

	"github.com/gofiber/fiber/v2"
)

// Me returns the current user information. (demo and for future use)
// @Summary Get current user
// @Description Get current user information
// @Tags auth
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} map[string]string
// @Router /me [get]
func Me(c *fiber.Ctx) error {
	userID := c.Locals(ctxkeys.UserIDKey).(string)
	userEmail := c.Locals(ctxkeys.UserEmailKey).(string)
	return c.JSON(fiber.Map{
		"uid":   userID,
		"email": userEmail,
	})
}
