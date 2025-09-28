package testutils

import (
	"context"
	"currency-exchange-api/config"
	"currency-exchange-api/logger"
	"currency-exchange-api/models"
	"time"
)

// MockLogger creates a mock logger for testing
func MockLogger() logger.Logger {
	return logger.New("debug")
}

// MockConfig creates a mock configuration for testing
func MockConfig() *config.Config {
	return &config.Config{
		Port:                  "8081",
		LogLevel:              "debug",
		RatesCacheTTL:         5 * time.Minute,
		MaxConcurrentRequests: 100,
		RateLimitEnabled:      true,
		RateLimitRequests:     100,
		RateLimitWindow:       time.Minute,
		RateLimitBurst:        10,
		ExchangeRateProviders: []config.ExchangeRateProvider{
			{
				Name:     "erapi",
				BaseURL:  "https://api.exchangerate-api.com/v4/latest",
				Enabled:  true,
				Priority: 1,
			},
			{
				Name:     "openexchangerates",
				BaseURL:  "https://openexchangerates.org/api/latest.json",
				Enabled:  true,
				Priority: 2,
			},
		},
	}
}

// MockRatesResponse creates a mock rates response for testing
func MockRatesResponse() models.RatesResponse {
	return models.RatesResponse{
		Base:      "USD",
		Timestamp: time.Now().Unix(),
		Rates: map[string]float64{
			"EUR": 0.85,
			"GBP": 0.75,
			"JPY": 110.0,
		},
		Provider: "mock-provider",
	}
}

// MockContext creates a mock context for testing
func MockContext() context.Context {
	return context.Background()
}
