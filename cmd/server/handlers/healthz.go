package handlers

import (
	"context"
	"note-pulse/internal/clients/mongo"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

const HealthzTimeout = 5 * time.Second

// Healthz returns the health of the server.
// @Summary Health check
// @Description Check if the server is healthy
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string
func Healthz(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.UserContext(), HealthzTimeout)
	defer cancel()

	db := mongo.DB()
	if db == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status": "down",
			"error":  "database not initialized",
		})
	}

	if err := db.Client().Ping(ctx, readpref.Primary()); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status": "down",
			"error":  err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"status": "ok",
	})
}
