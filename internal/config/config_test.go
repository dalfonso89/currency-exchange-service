package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	for _, env := range os.Environ() {
		key := env[:len(env)-len(os.Getenv(env))-1]
		originalEnv[key] = os.Getenv(key)
	}

	// Clean up after test
	defer func() {
		os.Clearenv()
		for key, value := range originalEnv {
			os.Setenv(key, value)
		}
	}()

	tests := []struct {
		name     string
		envVars  map[string]string
		expected func(*Config) bool
	}{
		{
			name:    "default configuration",
			envVars: map[string]string{},
			expected: func(cfg *Config) bool {
				return cfg.Port == "8081" &&
					cfg.LogLevel == "info" &&
					len(cfg.ExchangeRateProviders) == 4 &&
					cfg.RatesCacheTTL == 60*time.Second &&
					cfg.MaxConcurrentRequests == 4 &&
					cfg.RateLimitEnabled == true &&
					cfg.RateLimitRequests == 100 &&
					cfg.RateLimitWindow == 60*time.Second &&
					cfg.RateLimitBurst == 10
			},
		},
		{
			name: "custom configuration",
			envVars: map[string]string{
				"PORT":                      "9090",
				"LOG_LEVEL":                 "debug",
				"API_TIMEOUT_SECONDS":       "60",
				"API_RETRY_COUNT":           "5",
				"API_RETRY_DELAY_SECONDS":   "2",
				"RATES_CACHE_TTL_SECONDS":   "120",
				"MAX_CONCURRENT_REQUESTS":   "8",
				"RATE_LIMIT_ENABLED":        "false",
				"RATE_LIMIT_REQUESTS":       "200",
				"RATE_LIMIT_WINDOW_SECONDS": "120",
				"RATE_LIMIT_BURST":          "20",
			},
			expected: func(cfg *Config) bool {
				return cfg.Port == "9090" &&
					cfg.LogLevel == "debug" &&
					cfg.RatesCacheTTL == 120*time.Second &&
					cfg.MaxConcurrentRequests == 8 &&
					cfg.RateLimitEnabled == false &&
					cfg.RateLimitRequests == 200 &&
					cfg.RateLimitWindow == 120*time.Second &&
					cfg.RateLimitBurst == 20
			},
		},
		{
			name: "provider configuration",
			envVars: map[string]string{
				"EXCHANGE_RATE_API_ENABLED":   "false",
				"OPEN_EXCHANGE_RATES_ENABLED": "true",
				"FRANKFURTER_ENABLED":         "false",
				"EXCHANGE_RATE_HOST_ENABLED":  "true",
				"PROVIDER_1_NAME":             "custom-api",
				"PROVIDER_1_BASE_URL":         "https://api.custom.com/latest",
				"PROVIDER_1_API_KEY":          "custom-key",
				"PROVIDER_1_ENABLED":          "true",
				"PROVIDER_1_PRIORITY":         "1",
			},
			expected: func(cfg *Config) bool {
				// Check that we have the right number of enabled providers
				enabledCount := 0
				for _, provider := range cfg.ExchangeRateProviders {
					if provider.Enabled {
						enabledCount++
					}
				}
				return enabledCount == 3 // 2 default enabled + 1 custom
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set test environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Load configuration
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			// Check expected values
			if !tt.expected(cfg) {
				t.Errorf("Load() configuration does not match expected values")
			}
		})
	}
}

func TestLoadExchangeRateProviders(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	for _, env := range os.Environ() {
		key := env[:len(env)-len(os.Getenv(env))-1]
		originalEnv[key] = os.Getenv(key)
	}

	// Clean up after test
	defer func() {
		os.Clearenv()
		for key, value := range originalEnv {
			os.Setenv(key, value)
		}
	}()

	tests := []struct {
		name     string
		envVars  map[string]string
		expected int // expected number of enabled providers
	}{
		{
			name:     "all providers enabled",
			envVars:  map[string]string{},
			expected: 4,
		},
		{
			name: "some providers disabled",
			envVars: map[string]string{
				"EXCHANGE_RATE_API_ENABLED":   "false",
				"OPEN_EXCHANGE_RATES_ENABLED": "false",
			},
			expected: 2,
		},
		{
			name: "with additional providers",
			envVars: map[string]string{
				"PROVIDER_1_NAME":     "custom1",
				"PROVIDER_1_BASE_URL": "https://api1.com",
				"PROVIDER_1_ENABLED":  "true",
				"PROVIDER_2_NAME":     "custom2",
				"PROVIDER_2_BASE_URL": "https://api2.com",
				"PROVIDER_2_ENABLED":  "true",
			},
			expected: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set test environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Load providers
			providers := loadExchangeRateProviders()

			// Count enabled providers
			enabledCount := 0
			for _, provider := range providers {
				if provider.Enabled {
					enabledCount++
				}
			}

			if enabledCount != tt.expected {
				t.Errorf("loadExchangeRateProviders() enabled count = %v, want %v", enabledCount, tt.expected)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		fallback string
		envValue string
		expected string
	}{
		{
			name:     "environment variable exists",
			key:      "TEST_VAR",
			fallback: "default",
			envValue: "env_value",
			expected: "env_value",
		},
		{
			name:     "environment variable does not exist",
			key:      "NONEXISTENT_VAR",
			fallback: "default",
			envValue: "",
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable if needed
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			result := getEnv(tt.key, tt.fallback)
			if result != tt.expected {
				t.Errorf("getEnv() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMustAtoi(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "valid integer",
			input:    "123",
			expected: 123,
		},
		{
			name:     "invalid integer",
			input:    "abc",
			expected: 60, // default fallback
		},
		{
			name:     "empty string",
			input:    "",
			expected: 60, // default fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mustAtoi(tt.input)
			if result != tt.expected {
				t.Errorf("mustAtoi() = %v, want %v", result, tt.expected)
			}
		})
	}
}
