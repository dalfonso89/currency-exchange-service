package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dalfonso89/currency-exchange-service/api"
	"github.com/dalfonso89/currency-exchange-service/config"
	"github.com/dalfonso89/currency-exchange-service/logger"
	"github.com/dalfonso89/currency-exchange-service/ratelimit"
	"github.com/dalfonso89/currency-exchange-service/service"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	loggerInstance := logger.New(cfg.LogLevel)
	logrusLogger := loggerInstance.(*logger.LogrusLogger)
	logrusLogger.SetOutput(os.Stdout)

	// Initialize services
	ratesService := service.NewRatesService(cfg, loggerInstance)
	rateLimiter := ratelimit.NewLimiter(cfg, loggerInstance)

	// Initialize HTTP handlers
	handlerConfig := api.HandlerConfig{
		Logger:       loggerInstance,
		RatesService: ratesService,
		RateLimiter:  rateLimiter,
	}
	handlers := api.NewHandlers(handlerConfig)

	// Setup Gin router
	router := handlers.SetupRoutes()

	// Setup HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		loggerInstance.Info("Starting microservice on port " + cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			loggerInstance.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	loggerInstance.Info("Shutting down server...")

	// Stop rate limiter cleanup
	rateLimiter.Stop()

	// Give outstanding requests 30 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		loggerInstance.Fatalf("Server forced to shutdown: %v", err)
	}

	loggerInstance.Info("Server stopped")
}
