package main

import (
	"context"
	"strings"
	"time"

	"note-pulse/internal/clients/mongo"
	"note-pulse/internal/config"
	authHandlers "note-pulse/internal/handlers/auth"
	"note-pulse/internal/handlers/httperr"
	notesHandlers "note-pulse/internal/handlers/notes"
	"note-pulse/internal/logger"
	authServices "note-pulse/internal/services/auth"
	notesServices "note-pulse/internal/services/notes"
	"note-pulse/internal/utils/crypto"

	_ "note-pulse/docs" // Load swagger docs

	"github.com/go-playground/validator/v10"
	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/contrib/websocket"
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
func setupRouter(cfg config.Config) *fiber.App {

	// Initialize validator and register password validation
	v := validator.New()
	if err := crypto.RegisterPasswordValidator(v); err != nil {
		logger.L().Error("failed to register password validator", "err", err)
		panic(err)
	}

	// Validate JWT algorithm at boot
	alg := strings.ToUpper(cfg.JWTAlgorithm)
	switch alg {
	case "HS256", "RS256":
		// Valid algorithms
	default:
		logger.L().Error("unsupported JWT algorithm", "algorithm", cfg.JWTAlgorithm)
		panic("unsupported JWT algorithm: " + cfg.JWTAlgorithm)
	}

	app := fiber.New(fiber.Config{
		ErrorHandler: httperr.Handler,
	})

	// Global middlewares
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Content-Type, Authorization",
	}))

	// Health check endpoint, outside versioned API to appease scanners and to avoid logging
	app.Get("/healthz", func(c *fiber.Ctx) error {
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
	refreshTokensRepo := mongo.NewRefreshTokensRepo(mongo.DB(), cfg.BcryptCost)
	authSvc := authServices.NewService(usersRepo, refreshTokensRepo, cfg, logger.L())
	authHandlers := authHandlers.NewHandlers(authSvc, v)

	authGrp.Post("/sign-up", authHandlers.SignUp)
	authGrp.Post("/sign-in", limiterMW, authHandlers.SignIn)
	authGrp.Post("/refresh", limiterMW, authHandlers.Refresh)
	authGrp.Post("/sign-out", jwtMiddleware, authHandlers.SignOut)
	authGrp.Post("/sign-out-all", jwtMiddleware, authHandlers.SignOutAll)

	// Notes routes
	notesRepo, err := mongo.NewNotesRepo(mongo.DB())
	if err != nil {
		logger.L().Error("failed to create notes repository", "error", err)
		panic(err)
	}
	hub := notesServices.NewHub(cfg.WSOutboxBuffer)
	notesSvc := notesServices.NewService(notesRepo, hub, logger.L())
	notesH := notesHandlers.NewHandlers(notesSvc, v)

	notesGrp := v1.Group("/notes", jwtMiddleware)
	notesGrp.Post("/", notesH.Create)
	notesGrp.Get("/", notesH.List)
	notesGrp.Patch("/:id", notesH.Update)
	notesGrp.Delete("/:id", notesH.Delete)

	// WebSocket routes
	wsHandlers := notesHandlers.NewWebSocketHandlers(hub, cfg.JWTSecret, cfg.WSMaxSessionSec)
	app.Use("/ws", notesHandlers.LogWSConnections(cfg.JWTSecret))
	app.Get("/ws/notes/stream", wsHandlers.WSUpgrade, websocket.New(wsHandlers.WSNotesStream))

	// User profile endpoint (for testing JWT middleware and for future use)
	v1.Get("/me", jwtMiddleware, me)

	return app
}

// @Summary Get current user
// @Description Get current user information
// @Tags auth
// @Accept json
// @Produce json
// @Security Bearer
// @Success 200 {object} map[string]string
// @Router /me [get]
func me(c *fiber.Ctx) error {
	userID := c.Locals("userID").(string)
	userEmail := c.Locals("userEmail").(string)
	return c.JSON(fiber.Map{
		"uid":   userID,
		"email": userEmail,
	})
}
