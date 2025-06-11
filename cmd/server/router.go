package main

import (
	"context"
	"strings"
	"time"

	"note-pulse/cmd/server/handlers"
	"note-pulse/cmd/server/handlers/auth"
	"note-pulse/cmd/server/handlers/httperr"
	notesHandlers "note-pulse/cmd/server/handlers/notes"
	"note-pulse/cmd/server/middlewares"
	"note-pulse/internal/clients/mongo"
	"note-pulse/internal/config"
	"note-pulse/internal/logger"
	authServices "note-pulse/internal/services/auth"
	notesServices "note-pulse/internal/services/notes"
	"note-pulse/internal/utils/crypto"

	_ "note-pulse/docs" // Load swagger docs

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/swagger"
)

const (
	RateLimitExpiration = 1 * time.Minute
)

// setupRouter configures and returns a Fiber app with all routes
func setupRouter(ctx context.Context, cfg config.Config) *fiber.App {

	// Initialize validator and register password validation
	v := validator.New()
	if err := crypto.RegisterPasswordValidator(v); err != nil {
		logger.L().Error("failed to register password validator", "err", err)
		panic(err)
	}

	// Validate JWT algorithm at boot
	alg := strings.ToUpper(cfg.JWTAlgorithm)
	switch alg {
	case "HS256":
		// Valid algorithm
	default:
		logger.L().Error(authServices.ErrUnsupportedJWTAlg.Error(), "algorithm", cfg.JWTAlgorithm)
		panic(authServices.ErrUnsupportedJWTAlg.Error() + ": " + cfg.JWTAlgorithm)
	}

	app := fiber.New(fiber.Config{
		ErrorHandler: httperr.Handler,
		Immutable:    true, // make Fiber copy all request-derived strings
	})

	// Global middlewares
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Content-Type, Authorization",
	}))

	if cfg.RouteMetricsEnabled {
		middlewares.AttachMetrics(app)
	}

	// Health check endpoint, outside versioned API to appease scanners and to avoid logging
	app.Get("/healthz", handlers.Healthz)

	app.Get("/docs/*", swagger.HandlerDefault)

	app.Static("/", "./web-ui", fiber.Static{
		Browse: false,
		Index:  "index.html",
	})

	var v1 fiber.Router
	if cfg.RequestLoggingEnabled {
		v1 = app.Group("/api/v1", fiberlogger.New())
		logger.L().Info("request logging enabled")
	} else {
		v1 = app.Group("/api/v1")
		logger.L().Info("request logging disabled")
	}

	jwtMiddleware := middlewares.JWT(cfg)

	limiterMW := limiter.New(limiter.Config{
		Max:        cfg.SignInRatePerMin,
		Expiration: RateLimitExpiration,
		LimitReached: func(c *fiber.Ctx) error {
			return httperr.Fail(httperr.ErrTooManyRequests)
		},
	})

	authGrp := v1.Group("/auth", limiterMW)

	usersRepo, newUsersRepoErr := mongo.NewUsersRepo(ctx, mongo.DB())
	if newUsersRepoErr != nil {
		logger.L().Error("failed to create users repository", "error", newUsersRepoErr)
		panic(newUsersRepoErr)
	}
	refreshTokensRepo, newRefreshTokensRepoErr := mongo.NewRefreshTokensRepo(ctx, mongo.DB())
	if newRefreshTokensRepoErr != nil {
		logger.L().Error("failed to create refresh tokens repository", "error", newRefreshTokensRepoErr)
		panic(newRefreshTokensRepoErr)
	}
	authSvc := authServices.NewService(usersRepo, refreshTokensRepo, cfg, logger.L())
	authHandlers := auth.NewHandlers(authSvc, v)

	authGrp.Post("/sign-up", authHandlers.SignUp)
	authGrp.Post("/sign-in", authHandlers.SignIn)
	authGrp.Post("/refresh", authHandlers.Refresh)
	authGrp.Post("/sign-out", jwtMiddleware, authHandlers.SignOut)
	authGrp.Post("/sign-out-all", jwtMiddleware, authHandlers.SignOutAll)

	// Notes routes
	notesRepo, err := mongo.NewNotesRepo(ctx, mongo.DB())
	if err != nil {
		logger.L().Error(notesServices.ErrCreateNotesRepo.Error(), "error", err)
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
	v1.Get("/me", jwtMiddleware, handlers.Me)

	return app
}
