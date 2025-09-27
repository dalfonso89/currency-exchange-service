package api

import (
	"currency-exchange-api/internal/logger"
	"currency-exchange-api/internal/ratelimit"
	"currency-exchange-api/internal/service"
	"currency-exchange-api/internal/testutils"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// TestRaceConditionCacheAccess tests for race conditions in cache access
func TestRaceConditionCacheAccess(t *testing.T) {
	// Create mock servers
	mockExchangeRateServer := testutils.NewMockExchangeRateServer()
	mockJSONPlaceholderServer := testutils.NewMockJSONPlaceholderServer()
	defer mockExchangeRateServer.Close()
	defer mockJSONPlaceholderServer.Close()

	// Create test configuration with mock servers
	cfg := testutils.MockConfigWithMocks(mockExchangeRateServer.URL(), mockJSONPlaceholderServer.URL())
	cfg.RatesCacheTTL = 100 * time.Millisecond // Very short TTL
	cfg.MaxConcurrentRequests = 20
	cfg.RateLimitEnabled = false // Disable rate limiting for this test

	logger := logger.New("error")
	apiService := service.NewAPIService(cfg, logger)
	ratesService := service.NewRatesService(cfg, logger)
	handlers := NewHandlers(apiService, logger).WithRates(ratesService)

	gin.SetMode(gin.TestMode)
	router := handlers.SetupRoutes()
	server := httptest.NewServer(router)
	defer server.Close()

	const numGoroutines = 50
	const requestsPerGoroutine = 10

	var wg sync.WaitGroup
	responses := make(chan CacheTestResponse, numGoroutines*requestsPerGoroutine)
	errors := make(chan error, numGoroutines*requestsPerGoroutine)

	// Launch concurrent requests that will trigger cache updates
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < requestsPerGoroutine; j++ {
				start := time.Now()

				resp, err := http.Get(server.URL + "/api/v1/rates")
				if err != nil {
					errors <- fmt.Errorf("goroutine %d request %d failed: %w", goroutineID, j, err)
					continue
				}

				var response map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
					errors <- fmt.Errorf("goroutine %d request %d decode failed: %w", goroutineID, j, err)
					resp.Body.Close()
					continue
				}
				resp.Body.Close()

				responses <- CacheTestResponse{
					GoroutineID: goroutineID,
					RequestID:   j,
					StatusCode:  resp.StatusCode,
					Duration:    time.Since(start),
					Response:    response,
					Timestamp:   time.Now(),
				}

				// Small delay to allow cache to expire and be refreshed
				time.Sleep(50 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	close(responses)
	close(errors)

	// Check for errors
	var errorCount int
	for err := range errors {
		t.Logf("Cache race condition test error: %v", err)
		errorCount++
	}

	// Validate responses
	var responsesList []CacheTestResponse
	for response := range responses {
		responsesList = append(responsesList, response)
	}

	// Check for data consistency
	if len(responsesList) > 1 {
		validateCacheConsistency(t, responsesList)
	}

	if errorCount > 0 {
		t.Errorf("Cache race condition test had %d errors", errorCount)
	}

	t.Logf("Cache race condition test completed with %d successful responses", len(responsesList))
}

// TestRaceConditionRateLimiter tests for race conditions in rate limiter
func TestRaceConditionRateLimiter(t *testing.T) {
	// Create mock servers
	mockExchangeRateServer := testutils.NewMockExchangeRateServer()
	mockJSONPlaceholderServer := testutils.NewMockJSONPlaceholderServer()
	defer mockExchangeRateServer.Close()
	defer mockJSONPlaceholderServer.Close()

	// Create test configuration with mock servers
	cfg := testutils.MockConfigWithMocks(mockExchangeRateServer.URL(), mockJSONPlaceholderServer.URL())
	cfg.RatesCacheTTL = 60 * time.Second
	cfg.MaxConcurrentRequests = 20
	cfg.RateLimitEnabled = true
	cfg.RateLimitRequests = 50 // Allow many requests
	cfg.RateLimitWindow = 60 * time.Second
	cfg.RateLimitBurst = 20

	logger := logger.New("error")
	apiService := service.NewAPIService(cfg, logger)
	ratesService := service.NewRatesService(cfg, logger)
	rateLimiter := ratelimit.NewLimiter(cfg, logger)
	handlers := NewHandlers(apiService, logger).WithRates(ratesService).WithRateLimit(rateLimiter)

	gin.SetMode(gin.TestMode)
	router := handlers.SetupRoutes()
	server := httptest.NewServer(router)
	defer server.Close()

	const numGoroutines = 30
	const requestsPerGoroutine = 5

	var wg sync.WaitGroup
	responses := make(chan RateLimitTestResponse, numGoroutines*requestsPerGoroutine)
	errors := make(chan error, numGoroutines*requestsPerGoroutine)

	// Launch concurrent requests to test rate limiter
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < requestsPerGoroutine; j++ {
				start := time.Now()

				resp, err := http.Get(server.URL + "/api/v1/rates")
				if err != nil {
					errors <- fmt.Errorf("goroutine %d request %d failed: %w", goroutineID, j, err)
					continue
				}

				var response map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
					errors <- fmt.Errorf("goroutine %d request %d decode failed: %w", goroutineID, j, err)
					resp.Body.Close()
					continue
				}
				resp.Body.Close()

				responses <- RateLimitTestResponse{
					GoroutineID: goroutineID,
					RequestID:   j,
					StatusCode:  resp.StatusCode,
					Duration:    time.Since(start),
					Response:    response,
					Timestamp:   time.Now(),
				}
			}
		}(i)
	}

	wg.Wait()
	close(responses)
	close(errors)

	// Check for errors
	var errorCount int
	for err := range errors {
		t.Logf("Rate limiter race condition test error: %v", err)
		errorCount++
	}

	// Validate responses
	var responsesList []RateLimitTestResponse
	for response := range responses {
		responsesList = append(responsesList, response)
	}

	// Check rate limiting behavior
	validateRateLimiting(t, responsesList)

	if errorCount > 0 {
		t.Errorf("Rate limiter race condition test had %d errors", errorCount)
	}

	t.Logf("Rate limiter race condition test completed with %d responses", len(responsesList))
}

// TestRaceConditionProviderAccess tests for race conditions in provider access
func TestRaceConditionProviderAccess(t *testing.T) {
	// Create mock servers
	mockExchangeRateServer := testutils.NewMockExchangeRateServer()
	mockJSONPlaceholderServer := testutils.NewMockJSONPlaceholderServer()
	defer mockExchangeRateServer.Close()
	defer mockJSONPlaceholderServer.Close()

	// Create test configuration with mock servers
	cfg := testutils.MockConfigWithMocks(mockExchangeRateServer.URL(), mockJSONPlaceholderServer.URL())
	cfg.RatesCacheTTL = 1 * time.Second // Short TTL to force provider calls
	cfg.MaxConcurrentRequests = 5       // Limit concurrent requests to test semaphore
	cfg.RateLimitEnabled = false

	logger := logger.New("error")
	apiService := service.NewAPIService(cfg, logger)
	ratesService := service.NewRatesService(cfg, logger)
	handlers := NewHandlers(apiService, logger).WithRates(ratesService)

	gin.SetMode(gin.TestMode)
	router := handlers.SetupRoutes()
	server := httptest.NewServer(router)
	defer server.Close()

	const numGoroutines = 20
	const requestsPerGoroutine = 3

	var wg sync.WaitGroup
	responses := make(chan ProviderTestResponse, numGoroutines*requestsPerGoroutine)
	errors := make(chan error, numGoroutines*requestsPerGoroutine)

	// Launch concurrent requests to test provider access
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < requestsPerGoroutine; j++ {
				start := time.Now()

				resp, err := http.Get(server.URL + "/api/v1/rates")
				if err != nil {
					errors <- fmt.Errorf("goroutine %d request %d failed: %w", goroutineID, j, err)
					continue
				}

				var response map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
					errors <- fmt.Errorf("goroutine %d request %d decode failed: %w", goroutineID, j, err)
					resp.Body.Close()
					continue
				}
				resp.Body.Close()

				responses <- ProviderTestResponse{
					GoroutineID: goroutineID,
					RequestID:   j,
					StatusCode:  resp.StatusCode,
					Duration:    time.Since(start),
					Response:    response,
					Timestamp:   time.Now(),
				}

				// Wait for cache to expire
				time.Sleep(1 * time.Second)
			}
		}(i)
	}

	wg.Wait()
	close(responses)
	close(errors)

	// Check for errors
	var errorCount int
	for err := range errors {
		t.Logf("Provider race condition test error: %v", err)
		errorCount++
	}

	// Validate responses
	var responsesList []ProviderTestResponse
	for response := range responses {
		responsesList = append(responsesList, response)
	}

	// Check provider access consistency
	validateProviderAccess(t, responsesList)

	if errorCount > 0 {
		t.Errorf("Provider race condition test had %d errors", errorCount)
	}

	t.Logf("Provider race condition test completed with %d responses", len(responsesList))
}

// Helper functions and types

type CacheTestResponse struct {
	GoroutineID int
	RequestID   int
	StatusCode  int
	Duration    time.Duration
	Response    map[string]interface{}
	Timestamp   time.Time
}

type RateLimitTestResponse struct {
	GoroutineID int
	RequestID   int
	StatusCode  int
	Duration    time.Duration
	Response    map[string]interface{}
	Timestamp   time.Time
}

type ProviderTestResponse struct {
	GoroutineID int
	RequestID   int
	StatusCode  int
	Duration    time.Duration
	Response    map[string]interface{}
	Timestamp   time.Time
}

func validateCacheConsistency(t *testing.T, responses []CacheTestResponse) {
	// All responses should have consistent structure
	for i, response := range responses {
		if response.StatusCode != http.StatusOK {
			t.Errorf("Response %d has non-OK status: %d", i, response.StatusCode)
		}

		if _, ok := response.Response["base"]; !ok {
			t.Errorf("Response %d missing 'base' field", i)
		}
		if _, ok := response.Response["rates"]; !ok {
			t.Errorf("Response %d missing 'rates' field", i)
		}
		if _, ok := response.Response["timestamp"]; !ok {
			t.Errorf("Response %d missing 'timestamp' field", i)
		}
	}
}

func validateRateLimiting(t *testing.T, responses []RateLimitTestResponse) {
	successCount := 0
	rateLimitedCount := 0

	for _, response := range responses {
		if response.StatusCode == http.StatusOK {
			successCount++
		} else if response.StatusCode == http.StatusTooManyRequests {
			rateLimitedCount++
		}
	}

	t.Logf("Rate limiting test: %d successful, %d rate limited", successCount, rateLimitedCount)

	// Should have some successful requests
	if successCount == 0 {
		t.Error("No successful requests in rate limiting test")
	}
}

func validateProviderAccess(t *testing.T, responses []ProviderTestResponse) {
	successCount := 0

	for _, response := range responses {
		if response.StatusCode == http.StatusOK {
			successCount++

			// Check response structure
			if _, ok := response.Response["base"]; !ok {
				t.Errorf("Response missing 'base' field")
			}
			if _, ok := response.Response["rates"]; !ok {
				t.Errorf("Response missing 'rates' field")
			}
		}
	}

	t.Logf("Provider access test: %d successful responses", successCount)

	// Should have some successful requests
	if successCount == 0 {
		t.Error("No successful requests in provider access test")
	}
}
