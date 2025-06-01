package main

import (
	"context"
	"log"

	"note-pulse/internal/config"
	"note-pulse/internal/logger"
)

func main() {
	cfg, err := config.Load(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	
	logger, err := logger.Init(cfg)
	if err != nil {
		log.Fatal(err)
	}
	
	logger.Info("starting NotePulse", "port", cfg.AppPort)
}
