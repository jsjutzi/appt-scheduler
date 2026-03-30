package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jsjutzi/appt-scheduler/pkg/api"
	"github.com/jsjutzi/appt-scheduler/pkg/config"
	"github.com/jsjutzi/appt-scheduler/pkg/db"
	"github.com/jsjutzi/appt-scheduler/pkg/service"
)

var seedFilePath = "./appointments.json"

func main() {
	// Set up signal handling for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Load configuration
	cfg := config.Load()

	dbPool, err := db.NewPool(ctx, cfg.PgConnectionString)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer dbPool.Close()

	// Initialize repositories and services
	apptRepo := db.NewApptRepository(dbPool)
	apptService := service.NewApptService(apptRepo)
	apiHandler := api.NewAPI(apptService)

	// Seed data (safe to call N times)
	if err := apptRepo.SeedFromJSON(ctx, seedFilePath); err != nil {
		log.Printf("Warning: Failed to seed data: %v", err)
	} else {
		log.Println("Successfully seeded appointments from appointments.json")
	}

	// Setup router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.Timeout(30 * time.Second))

	// Register all routes
	apiHandler.SetupRoutes(r)

	// Start HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Run server in background
	go func() {
		log.Printf("Server running on http://localhost:%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("Shutting down gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Forced shutdown: %v", err)
	}

	log.Println("Server stopped")
}
