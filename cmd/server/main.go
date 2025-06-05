package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	mongo "note-pulse/internal/clients/mongo" // mongo client singleton
	"note-pulse/internal/config"
	"note-pulse/internal/logger"

	"github.com/grafana/pyroscope-go"
	_ "go.uber.org/automaxprocs"
	"golang.org/x/sync/errgroup"
)

// Build-time variables set by ldflags
var (
	version = "dev"
	commit  = "none"
	builtAt = "unknown"
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

	if cfg.PyroscopeEnabled {
		if _, err := pyroscope.Start(pyroscope.Config{
			ApplicationName: cfg.PyroscopeAppName,
			ServerAddress:   cfg.PyroscopeServerAddr,
			Tags:            map[string]string{"commit": commit},
		}); err != nil {
			logg.Error("pyroscope start failed", "err", err)
		} else {
			logg.Info("pyroscope agent started", "server", cfg.PyroscopeServerAddr)
		}
	}

	if cfg.PprofEnabled {
		go func() {
			// simple tiny extra server
			// run on loopback so cAdvisor / Prometheus can scrape if needed
			if err := http.ListenAndServe("0.0.0.0:6060", nil); err != nil {
				logg.Error("pprof server error", "err", err)
			}
		}()
		logg.Info("pprof enabled at :6060/debug/pprof/")
	}

	_, db, err := mongo.Init(ctx, cfg, logg)
	if err != nil {
		logg.Error("mongo init", "err", err)
		os.Exit(1)
	}
	logg.Info("connected to mongo", "db", db.Name())

	logg.Info("starting NotePulse", "port", cfg.AppPort, "version", version, "commit", commit, "built_at", builtAt)

	// Setup router and start server
	app := setupRouter(cfg)
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
