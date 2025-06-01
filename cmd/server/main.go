package main

import (
	"context"
	"fmt"
	"log"

	"note-pulse/internal/config"
)

func main() {
	cfg, err := config.Load(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("config: %+v\n", cfg)
}
