package main

import (
	"context"
	"log"
	"os"

	mongo "note-pulse/internal/clients/mongo" // mongo client singleton
	"note-pulse/internal/config"
	"note-pulse/internal/logger"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.Load(ctx)
	if err != nil {
		log.Fatal(err)
	}

	logg, err := logger.Init(cfg)
	if err != nil {
		log.Fatal(err)
	}

	_, db, err := mongo.Init(ctx, cfg, logg)
	if err != nil {
		logg.Error("mongo init", "err", err)
		os.Exit(1)
	}
	logg.Info("connected to mongo", "db", db.Name())

	logg.Info("starting NotePulse", "port", cfg.AppPort)

	// continue bootstrapping...
}
