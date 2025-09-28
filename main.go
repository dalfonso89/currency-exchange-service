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
	serverErr := make(chan error, 1)
	go func() {
		loggerInstance.Info("Starting microservice on port " + cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-quit:
		loggerInstance.Infof("Received signal: %v", sig)
	case err := <-serverErr:
		loggerInstance.Errorf("Server error: %v", err)
		os.Exit(1)
	}

	loggerInstance.Info("Shutting down server...")

	// Stop rate limiter cleanup
	rateLimiter.Stop()

	// Create shutdown context with timeout for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Graceful server shutdown
	if err := server.Shutdown(shutdownCtx); err != nil {
		loggerInstance.Errorf("Server shutdown error: %v", err)
		// Force close if graceful shutdown fails
		if closeErr := server.Close(); closeErr != nil {
			loggerInstance.Errorf("Force close error: %v", closeErr)
		}
		os.Exit(1)
	}

	loggerInstance.Info("Server stopped gracefully")
}
