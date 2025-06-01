package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mongo "note-pulse/internal/clients/mongo" // mongo client singleton
	"note-pulse/internal/config"
	"note-pulse/internal/logger"

	"golang.org/x/sync/errgroup"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	g, ctx := errgroup.WithContext(ctx)

	// Create bootstrap logger for early errors
	bootstrapLog := log.New(os.Stderr, "bootstrap: ", log.LstdFlags)

	cfg, err := config.Load()
	if err != nil {
		bootstrapLog.Printf("config load failed: %v", err)
		os.Exit(1)
	}

	logg, err := logger.Init(cfg)
	if err != nil {
		bootstrapLog.Printf("logger init failed: %v", err)
		os.Exit(1)
	}

	_, db, err := mongo.Init(ctx, cfg, logg)
	if err != nil {
		logg.Error("mongo init", "err", err)
		os.Exit(1)
	}
	logg.Info("connected to mongo", "db", db.Name())

	logg.Info("starting NotePulse", "port", cfg.AppPort)

	// Setup router and start server
	app := setupRouter()
	portStr := fmt.Sprintf(":%d", cfg.AppPort)

	g.Go(func() error {
		err := app.Listen(portStr)
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	})

	// Graceful shutdown
	g.Go(func() error {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
		defer cancel()

		if err := app.Shutdown(); err != nil {
			return err
		}
		return mongo.Shutdown(shutdownCtx)
	})

	// Wait and exit
	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		logg.Error("fatal", "err", err)
		os.Exit(1)
	}
	logg.Info("graceful shutdown complete")
}
