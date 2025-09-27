package testutils

import (
	"context"
	"currency-exchange-api/internal/config"
	"currency-exchange-api/internal/logger"
	"currency-exchange-api/internal/models"
	"time"

	"github.com/sirupsen/logrus"
)

// MockLogger creates a mock logger for testing
func MockLogger() *logrus.Logger {
	return logger.New("debug")
}

// MockConfig creates a mock configuration for testing
func MockConfig() *config.Config {
	return &config.Config{
		Port:     "8081",
		LogLevel: "debug",

		ExchangeRateProviders: []config.ExchangeRateProvider{
			{
				Name:       "test-provider",
				BaseURL:    "https://api.test.com/latest",
				APIKey:     "test-api-key",
				Enabled:    true,
				Priority:   1,
				Timeout:    30 * time.Second,
				RetryCount: 3,
				RetryDelay: 1 * time.Second,
			},
		},
		RatesCacheTTL:         60 * time.Second,
		MaxConcurrentRequests: 4,

		RateLimitEnabled:  true,
		RateLimitRequests: 100,
		RateLimitWindow:   60 * time.Second,
		RateLimitBurst:    10,
	}
}

// MockRatesResponse creates a mock rates response for testing
func MockRatesResponse() models.RatesResponse {
	return models.RatesResponse{
		Base:      "USD",
		Timestamp: time.Now().Unix(),
		Rates: map[string]float64{
			"EUR": 0.85,
			"GBP": 0.73,
			"JPY": 110.0,
		},
		Provider: "test-provider",
	}
}

// MockHealthCheck creates a mock health check response for testing
func MockHealthCheck() models.HealthCheck {
	return models.HealthCheck{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Uptime:    "1m30s",
	}
}

// MockErrorResponse creates a mock error response for testing
func MockErrorResponse() models.ErrorResponse {
	return models.ErrorResponse{
		Error:   "test error",
		Message: "test error message",
		Code:    400,
	}
}

// MockContext creates a mock context for testing
func MockContext() context.Context {
	return context.Background()
}

// MockContextWithTimeout creates a mock context with timeout for testing
func MockContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}
