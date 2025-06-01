package main

import (
	"context"
	"time"

	"note-pulse/internal/clients/mongo"
	"note-pulse/internal/config"
	authHandlers "note-pulse/internal/handlers/auth"
	"note-pulse/internal/handlers/httperr"
	"note-pulse/internal/logger"
	authServices "note-pulse/internal/services/auth"
	"note-pulse/internal/utils/crypto"

	_ "note-pulse/docs" // Load swagger docs

	"github.com/go-playground/validator/v10"
	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/swagger"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// setupRouter configures and returns a Fiber app with all routes
func setupRouter() *fiber.App {
	cfg, err := config.Load()
	if err != nil {
		logger.L().Error("config load failed in router", "err", err)
		panic(err)
	}

	// Initialize validator and register password validation
	v := validator.New()
	if err := crypto.RegisterPasswordValidator(v); err != nil {
		logger.L().Error("failed to register password validator", "err", err)
		panic(err)
	}

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			logger.L().Error("fiber error", "err", err, "path", c.Path(), "method", c.Method())
			return httperr.Handler(c, err)
		},
	})

	// Global middlewares
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Content-Type, Authorization",
	}))

	// Health check endpoint, separate from the group v1, to avoid logging in the health check
	app.Get("/api/v1/healthz", func(c *fiber.Ctx) error {
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

	app.Get("/docs/*", swagger.HandlerDefault)

	// API v1 group
	v1 := app.Group("/api/v1", fiberlogger.New())

	jwtMiddleware := jwtware.New(jwtware.Config{
		SigningKey: jwtware.SigningKey{Key: []byte(cfg.JWTSecret)},
		SuccessHandler: func(c *fiber.Ctx) error {
			// Extract claims from token
			token := c.Locals("user").(*jwt.Token)
			claims := token.Claims.(jwt.MapClaims)

			userID, ok := claims["user_id"].(string)
			if !ok {
				return httperr.Fail(httperr.E{
					Status:  401,
					Message: "Invalid token: missing user_id",
				})
			}

			userEmail, ok := claims["email"].(string)
			if !ok {
				return httperr.Fail(httperr.E{
					Status:  401,
					Message: "Invalid token: missing email",
				})
			}

			c.Locals("userID", userID)
			c.Locals("userEmail", userEmail)
			return c.Next()
		},
	})

	limiterMW := limiter.New(limiter.Config{
		Max:        cfg.SignInRatePerMin,
		Expiration: 1 * time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return httperr.Fail(httperr.ErrTooManyRequests)
		},
	})

	authGrp := v1.Group("/auth")

	usersRepo := mongo.NewUsersRepo(mongo.DB())
	authSvc := authServices.NewService(usersRepo, cfg, logger.L())
	h := authHandlers.NewHandlers(authSvc, v)

	authGrp.Post("/sign-up", h.SignUp)
	authGrp.Post("/sign-in", limiterMW, h.SignIn)

	// Example protected route (for testing JWT middleware)
	protected := v1.Group("/protected", jwtMiddleware)
	protected.Get("/profile", profile)

	return app
}

// @Summary Get user profile
// @Description Get user profile
// @Tags protected
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} map[string]string
// @Router /protected/profile [get]
func profile(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	userEmail := c.Locals("userEmail").(string)
	return c.JSON(fiber.Map{
		"user_id": userID,
		"email":   userEmail,
		"message": "This is a protected route",
	})
}
