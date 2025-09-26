package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	Port        string
	LogLevel    string
	APIBaseURL  string
	APIKey      string
	Timeout     time.Duration
	RetryCount  int
	RetryDelay  time.Duration

	// Rates providers
	ExchangeRateAPIBaseURL   string
	ExchangeRateAPIKey       string
	OpenExchangeRatesBaseURL string
	OpenExchangeRatesAPIKey  string
	FrankfurterAPIBaseURL    string
	ExchangeRateHostBaseURL  string
	RatesCacheTTL            time.Duration

	// Rate limiting
	RateLimitEnabled      bool
	RateLimitRequests     int
	RateLimitWindow       time.Duration
	RateLimitBurst        int
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	timeout, err := strconv.Atoi(getEnv("API_TIMEOUT_SECONDS", "30"))
	if err != nil {
		timeout = 30
	}

	retryCount, err := strconv.Atoi(getEnv("API_RETRY_COUNT", "3"))
	if err != nil {
		retryCount = 3
	}

	retryDelay, err := strconv.Atoi(getEnv("API_RETRY_DELAY_SECONDS", "1"))
	if err != nil {
		retryDelay = 1
	}

	return &Config{
		Port:        getEnv("PORT", "8080"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		APIBaseURL:  getEnv("API_BASE_URL", "https://jsonplaceholder.typicode.com"),
		APIKey:      getEnv("API_KEY", ""),
		Timeout:     time.Duration(timeout) * time.Second,
		RetryCount:  retryCount,
		RetryDelay:  time.Duration(retryDelay) * time.Second,

		ExchangeRateAPIBaseURL:   getEnv("EXCHANGE_RATE_API_BASE_URL", "https://open.er-api.com/v6/latest"),
		ExchangeRateAPIKey:       getEnv("EXCHANGE_RATE_API_KEY", ""),
		OpenExchangeRatesBaseURL: getEnv("OPEN_EXCHANGE_RATES_BASE_URL", "https://openexchangerates.org/api/latest.json"),
		OpenExchangeRatesAPIKey:  getEnv("OPEN_EXCHANGE_RATES_API_KEY", ""),
		FrankfurterAPIBaseURL:    getEnv("FRANKFURTER_API_BASE_URL", "https://api.frankfurter.app/latest"),
		ExchangeRateHostBaseURL:  getEnv("EXCHANGE_RATE_HOST_BASE_URL", "https://api.exchangerate.host/latest"),
		RatesCacheTTL:            time.Duration(mustAtoi(getEnv("RATES_CACHE_TTL_SECONDS", "60"))) * time.Second,

		RateLimitEnabled:      getEnv("RATE_LIMIT_ENABLED", "true") == "true",
		RateLimitRequests:     mustAtoi(getEnv("RATE_LIMIT_REQUESTS", "100")),
		RateLimitWindow:       time.Duration(mustAtoi(getEnv("RATE_LIMIT_WINDOW_SECONDS", "60"))) * time.Second,
		RateLimitBurst:        mustAtoi(getEnv("RATE_LIMIT_BURST", "10")),
	}, nil
}

// getEnv gets an environment variable with a fallback value
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func mustAtoi(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 60
	}
	return i
}

