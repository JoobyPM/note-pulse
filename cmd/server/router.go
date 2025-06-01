package main

import (
	"context"
	"time"

	"note-pulse/internal/clients/mongo"
	"note-pulse/internal/logger"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// setupRouter configures and returns a Fiber app with all routes
func setupRouter() *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			logger.L().Error("fiber error", "err", err, "path", c.Path(), "method", c.Method())
			return fiber.DefaultErrorHandler(c, err)
		},
	})

	// API v1 group
	v1 := app.Group("/api/v1")

	// Health check endpoint
	v1.Get("/healthz", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
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
	})

	return app
}
