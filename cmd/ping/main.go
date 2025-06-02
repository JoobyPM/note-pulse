package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"note-pulse/internal/config"
	"note-pulse/internal/logger"
)

// healthResp matches the JSON shape returned by /api/v1/healthz
type healthResp struct {
	Status string `json:"status"`
}

func main() {
	bootstrap := log.New(os.Stderr, "ping: ", log.LstdFlags)

	// Set ENV LOG_LEVEL to error, as we don't want to see any logs from the ping command
	os.Setenv("LOG_LEVEL", "error")

	cfg, err := config.Load()
	if err != nil {
		bootstrap.Printf("config load failed: %v", err)
		os.Exit(1)
	}

	logg, err := logger.Init(cfg)
	if err != nil {
		bootstrap.Printf("logger init failed: %v", err)
		os.Exit(1)
	}

	url := fmt.Sprintf("http://localhost:%d/api/v1/healthz", cfg.AppPort)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		logg.Error("create request", "err", err)
		os.Exit(1)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logg.Error("request failed", "err", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logg.Error("unexpected status code", "code", resp.StatusCode)
		os.Exit(1)
	}

	var hr healthResp
	if err := json.NewDecoder(resp.Body).Decode(&hr); err != nil {
		logg.Error("decode body", "err", err)
		os.Exit(1)
	}

	if hr.Status != "ok" {
		logg.Error("service reported unhealthy", "status", hr.Status)
		os.Exit(1)
	}

	logg.Info("service healthy")
}
