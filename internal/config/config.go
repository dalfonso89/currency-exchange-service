package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// ExchangeRateProvider represents a single exchange rate API provider
type ExchangeRateProvider struct {
	Name       string
	BaseURL    string
	APIKey     string
	Enabled    bool
	Priority   int // Lower number = higher priority
	Timeout    time.Duration
	RetryCount int
	RetryDelay time.Duration
}

// Config holds all configuration for the application
type Config struct {
	Port     string
	LogLevel string

	// Exchange rate providers (dynamic list)
	ExchangeRateProviders []ExchangeRateProvider
	RatesCacheTTL         time.Duration
	MaxConcurrentRequests int

	// Rate limiting
	RateLimitEnabled  bool
	RateLimitRequests int
	RateLimitWindow   time.Duration
	RateLimitBurst    int
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Load exchange rate providers
	providers := loadExchangeRateProviders()

	return &Config{
		Port:     getEnv("PORT", "8081"),
		LogLevel: getEnv("LOG_LEVEL", "info"),

		ExchangeRateProviders: providers,
		RatesCacheTTL:         time.Duration(mustAtoi(getEnv("RATES_CACHE_TTL_SECONDS", "60"))) * time.Second,
		MaxConcurrentRequests: mustAtoi(getEnv("MAX_CONCURRENT_REQUESTS", "4")),

		RateLimitEnabled:  getEnv("RATE_LIMIT_ENABLED", "true") == "true",
		RateLimitRequests: mustAtoi(getEnv("RATE_LIMIT_REQUESTS", "100")),
		RateLimitWindow:   time.Duration(mustAtoi(getEnv("RATE_LIMIT_WINDOW_SECONDS", "60"))) * time.Second,
		RateLimitBurst:    mustAtoi(getEnv("RATE_LIMIT_BURST", "10")),
	}, nil
}

// loadExchangeRateProviders loads exchange rate providers from environment variables
func loadExchangeRateProviders() []ExchangeRateProvider {
	providers := []ExchangeRateProvider{}

	// Default providers (keeping the original four)
	defaultProviders := []ExchangeRateProvider{
		{
			Name:       "erapi",
			BaseURL:    getEnv("EXCHANGE_RATE_API_BASE_URL", "https://open.er-api.com/v6/latest"),
			APIKey:     getEnv("EXCHANGE_RATE_API_KEY", ""),
			Enabled:    getEnv("EXCHANGE_RATE_API_ENABLED", "true") == "true",
			Priority:   1,
			Timeout:    time.Duration(mustAtoi(getEnv("EXCHANGE_RATE_API_TIMEOUT", "30"))) * time.Second,
			RetryCount: mustAtoi(getEnv("EXCHANGE_RATE_API_RETRY_COUNT", "3")),
			RetryDelay: time.Duration(mustAtoi(getEnv("EXCHANGE_RATE_API_RETRY_DELAY", "1"))) * time.Second,
		},
		{
			Name:       "openexchangerates",
			BaseURL:    getEnv("OPEN_EXCHANGE_RATES_BASE_URL", "https://openexchangerates.org/api/latest.json"),
			APIKey:     getEnv("OPEN_EXCHANGE_RATES_API_KEY", ""),
			Enabled:    getEnv("OPEN_EXCHANGE_RATES_ENABLED", "true") == "true",
			Priority:   2,
			Timeout:    time.Duration(mustAtoi(getEnv("OPEN_EXCHANGE_RATES_TIMEOUT", "30"))) * time.Second,
			RetryCount: mustAtoi(getEnv("OPEN_EXCHANGE_RATES_RETRY_COUNT", "3")),
			RetryDelay: time.Duration(mustAtoi(getEnv("OPEN_EXCHANGE_RATES_RETRY_DELAY", "1"))) * time.Second,
		},
		{
			Name:       "frankfurter",
			BaseURL:    getEnv("FRANKFURTER_API_BASE_URL", "https://api.frankfurter.app/latest"),
			APIKey:     getEnv("FRANKFURTER_API_KEY", ""),
			Enabled:    getEnv("FRANKFURTER_ENABLED", "true") == "true",
			Priority:   3,
			Timeout:    time.Duration(mustAtoi(getEnv("FRANKFURTER_TIMEOUT", "30"))) * time.Second,
			RetryCount: mustAtoi(getEnv("FRANKFURTER_RETRY_COUNT", "3")),
			RetryDelay: time.Duration(mustAtoi(getEnv("FRANKFURTER_RETRY_DELAY", "1"))) * time.Second,
		},
		{
			Name:       "exchangerate.host",
			BaseURL:    getEnv("EXCHANGE_RATE_HOST_BASE_URL", "https://api.exchangerate.host/latest"),
			APIKey:     getEnv("EXCHANGE_RATE_HOST_API_KEY", ""),
			Enabled:    getEnv("EXCHANGE_RATE_HOST_ENABLED", "true") == "true",
			Priority:   4,
			Timeout:    time.Duration(mustAtoi(getEnv("EXCHANGE_RATE_HOST_TIMEOUT", "30"))) * time.Second,
			RetryCount: mustAtoi(getEnv("EXCHANGE_RATE_HOST_RETRY_COUNT", "3")),
			RetryDelay: time.Duration(mustAtoi(getEnv("EXCHANGE_RATE_HOST_RETRY_DELAY", "1"))) * time.Second,
		},
	}

	// Add default providers
	providers = append(providers, defaultProviders...)

	// Load additional providers from environment
	additionalProviders := loadAdditionalProviders()
	providers = append(providers, additionalProviders...)

	// Filter out disabled providers and sort by priority
	enabledProviders := []ExchangeRateProvider{}
	for _, provider := range providers {
		if provider.Enabled {
			enabledProviders = append(enabledProviders, provider)
		}
	}

	// Sort by priority (lower number = higher priority)
	for i := 0; i < len(enabledProviders); i++ {
		for j := i + 1; j < len(enabledProviders); j++ {
			if enabledProviders[i].Priority > enabledProviders[j].Priority {
				enabledProviders[i], enabledProviders[j] = enabledProviders[j], enabledProviders[i]
			}
		}
	}

	return enabledProviders
}

// loadAdditionalProviders loads additional providers from environment variables
func loadAdditionalProviders() []ExchangeRateProvider {
	providers := []ExchangeRateProvider{}

	// Check for additional providers (PROVIDER_1_NAME, PROVIDER_2_NAME, etc.)
	for i := 1; i <= 10; i++ { // Support up to 10 additional providers
		name := getEnv(fmt.Sprintf("PROVIDER_%d_NAME", i), "")
		if name == "" {
			break
		}

		provider := ExchangeRateProvider{
			Name:       name,
			BaseURL:    getEnv(fmt.Sprintf("PROVIDER_%d_BASE_URL", i), ""),
			APIKey:     getEnv(fmt.Sprintf("PROVIDER_%d_API_KEY", i), ""),
			Enabled:    getEnv(fmt.Sprintf("PROVIDER_%d_ENABLED", i), "true") == "true",
			Priority:   mustAtoi(getEnv(fmt.Sprintf("PROVIDER_%d_PRIORITY", i), "10")),
			Timeout:    time.Duration(mustAtoi(getEnv(fmt.Sprintf("PROVIDER_%d_TIMEOUT", i), "30"))) * time.Second,
			RetryCount: mustAtoi(getEnv(fmt.Sprintf("PROVIDER_%d_RETRY_COUNT", i), "3")),
			RetryDelay: time.Duration(mustAtoi(getEnv(fmt.Sprintf("PROVIDER_%d_RETRY_DELAY", i), "1"))) * time.Second,
		}

		if provider.BaseURL != "" {
			providers = append(providers, provider)
		}
	}

	return providers
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
