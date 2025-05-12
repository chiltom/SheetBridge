package main

import (
	"context"
	"log"

	"github.com/chiltom/SheetBridge/internal/handlers"
	"github.com/chiltom/SheetBridge/internal/repositories"
	"github.com/chiltom/SheetBridge/internal/services"
	"github.com/chiltom/SheetBridge/internal/utils"
)

func main() {
	ctx := context.Background()

	// Load configuration
	cfg, err := utils.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Initialize repository
	repo, err := repositories.NewRepository(ctx, cfg.DB)
	if err != nil {
		log.Fatalf("failed to initialize repository: %v", err)
	}
	defer repo.Close()

	// Initialize service
	svc := services.NewService(repo)

	// Start server
	srv := handlers.NewServer(cfg, svc)
	if err := srv.Run(ctx); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
