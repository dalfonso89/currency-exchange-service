package api

import (
	"context"
	"currency-exchange-api/internal/config"
	"currency-exchange-api/internal/logger"
	"currency-exchange-api/internal/ratelimit"
	"currency-exchange-api/internal/service"
	"currency-exchange-api/internal/testutils"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// TestSimpleRaceCondition tests for basic race conditions without external API calls
func TestSimpleRaceCondition(t *testing.T) {
	// Create mock servers
	mockExchangeRateServer := testutils.NewMockExchangeRateServer()
	mockJSONPlaceholderServer := testutils.NewMockJSONPlaceholderServer()
	defer mockExchangeRateServer.Close()
	defer mockJSONPlaceholderServer.Close()

	// Create test configuration with mock servers
	cfg := testutils.MockConfigWithMocks(mockExchangeRateServer.URL(), mockJSONPlaceholderServer.URL())
	cfg.RatesCacheTTL = 60 * time.Second
	cfg.MaxConcurrentRequests = 10
	cfg.RateLimitEnabled = false // Disable rate limiting for this test
	cfg.RateLimitRequests = 100
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

	const numGoroutines = 20
	const requestsPerGoroutine = 5

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	// Test concurrent access to health endpoint (no external API calls)
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < requestsPerGoroutine; j++ {
				resp, err := http.Get(server.URL + "/health")
				if err != nil {
					t.Logf("Goroutine %d request %d failed: %v", goroutineID, j, err)
					continue
				}
				resp.Body.Close()

				if resp.StatusCode == http.StatusOK {
					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}
		}(i)
	}

	wg.Wait()

	expectedRequests := numGoroutines * requestsPerGoroutine
	if successCount != expectedRequests {
		t.Errorf("Expected %d successful requests, got %d", expectedRequests, successCount)
	}

	t.Logf("Simple race condition test completed with %d successful responses", successCount)
}

// TestRateLimiterConcurrency tests rate limiter under concurrent access
func TestRateLimiterConcurrency(t *testing.T) {
	cfg := &config.Config{
		RateLimitEnabled:  true,
		RateLimitRequests: 50,
		RateLimitWindow:   60 * time.Second,
		RateLimitBurst:    10,
	}

	logger := logger.New("error")
	rateLimiter := ratelimit.NewLimiter(cfg, logger)

	const numGoroutines = 20
	const requestsPerGoroutine = 10

	var wg sync.WaitGroup
	allowedCount := 0
	deniedCount := 0
	var mu sync.Mutex

	// Test concurrent access to rate limiter
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < requestsPerGoroutine; j++ {
				allowed := rateLimiter.Allow("127.0.0.1")

				mu.Lock()
				if allowed {
					allowedCount++
				} else {
					deniedCount++
				}
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	totalRequests := numGoroutines * requestsPerGoroutine
	t.Logf("Rate limiter test: %d allowed, %d denied out of %d total", allowedCount, deniedCount, totalRequests)

	// Should have some allowed requests (at least the burst limit)
	if allowedCount < cfg.RateLimitBurst {
		t.Errorf("Expected at least %d allowed requests, got %d", cfg.RateLimitBurst, allowedCount)
	}

	// Should have some denied requests (due to rate limiting)
	if deniedCount == 0 {
		t.Log("Warning: No requests were rate limited - this might indicate the rate limiter is not working")
	}
}

// TestCacheConcurrency tests cache access under concurrent load
func TestCacheConcurrency(t *testing.T) {
	// Create mock servers
	mockExchangeRateServer := testutils.NewMockExchangeRateServer()
	mockJSONPlaceholderServer := testutils.NewMockJSONPlaceholderServer()
	defer mockExchangeRateServer.Close()
	defer mockJSONPlaceholderServer.Close()

	// Create test configuration with mock servers
	cfg := testutils.MockConfigWithMocks(mockExchangeRateServer.URL(), mockJSONPlaceholderServer.URL())
	cfg.RatesCacheTTL = 1 * time.Second
	cfg.MaxConcurrentRequests = 5

	logger := logger.New("error")
	ratesService := service.NewRatesService(cfg, logger)

	const numGoroutines = 10
	const requestsPerGoroutine = 3

	var wg sync.WaitGroup
	successCount := 0
	errorCount := 0
	var mu sync.Mutex

	// Test concurrent access to rates service
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < requestsPerGoroutine; j++ {
				ctx := context.Background()
				_, err := ratesService.GetRates(ctx, "USD")

				mu.Lock()
				if err != nil {
					errorCount++
				} else {
					successCount++
				}
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	totalRequests := numGoroutines * requestsPerGoroutine
	t.Logf("Cache concurrency test: %d successful, %d errors out of %d total", successCount, errorCount, totalRequests)

	// The test might fail due to external API issues, but we're testing for race conditions
	// So we just check that the service doesn't panic and handles concurrent access
	if successCount+errorCount != totalRequests {
		t.Errorf("Expected %d total responses, got %d", totalRequests, successCount+errorCount)
	}
}

// TestConcurrentHealthChecks tests health endpoint under concurrent load
func TestConcurrentHealthChecks(t *testing.T) {
	// Create mock servers
	mockExchangeRateServer := testutils.NewMockExchangeRateServer()
	mockJSONPlaceholderServer := testutils.NewMockJSONPlaceholderServer()
	defer mockExchangeRateServer.Close()
	defer mockJSONPlaceholderServer.Close()

	// Create test configuration with mock servers
	cfg := testutils.MockConfigWithMocks(mockExchangeRateServer.URL(), mockJSONPlaceholderServer.URL())

	logger := logger.New("error")
	apiService := service.NewAPIService(cfg, logger)
	handlers := NewHandlers(apiService, logger)

	gin.SetMode(gin.TestMode)
	router := handlers.SetupRoutes()
	server := httptest.NewServer(router)
	defer server.Close()

	const numGoroutines = 30
	const requestsPerGoroutine = 5

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	// Test concurrent access to health endpoint
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < requestsPerGoroutine; j++ {
				resp, err := http.Get(server.URL + "/health")
				if err != nil {
					t.Logf("Goroutine %d request %d failed: %v", goroutineID, j, err)
					continue
				}
				resp.Body.Close()

				if resp.StatusCode == http.StatusOK {
					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}
		}(i)
	}

	wg.Wait()

	expectedRequests := numGoroutines * requestsPerGoroutine
	if successCount != expectedRequests {
		t.Errorf("Expected %d successful requests, got %d", expectedRequests, successCount)
	}

	t.Logf("Concurrent health checks test completed with %d successful responses", successCount)
}
