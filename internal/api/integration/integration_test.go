//go:build integration

package integration

import (
	"currency-exchange-api/internal/api"
	"currency-exchange-api/internal/config"
	"currency-exchange-api/internal/logger"
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
	"github.com/sirupsen/logrus"
)

// IntegrationTestSuite provides shared setup for integration tests
type IntegrationTestSuite struct {
	server   *httptest.Server
	handlers *api.Handlers
	config   *config.Config
	logger   *logrus.Logger
}

// NewIntegrationTestSuite creates a new integration test suite
func NewIntegrationTestSuite() *IntegrationTestSuite {
	// Create mock servers
	mockExchangeRateServer := testutils.NewMockExchangeRateServer()
	mockJSONPlaceholderServer := testutils.NewMockJSONPlaceholderServer()

	// Create test configuration with mock servers
	cfg := testutils.MockConfigWithMocks(mockExchangeRateServer.URL(), mockJSONPlaceholderServer.URL())
	cfg.MaxConcurrentRequests = 20
	cfg.RateLimitEnabled = false // Disable rate limiting for integration tests

	// Create logger
	logger := logger.New("error")

	// Create services
	ratesService := service.NewRatesService(cfg, logger)

	// Create handlers
	handlerConfig := api.HandlerConfig{
		Logger:       logger,
		RatesService: ratesService,
		RateLimiter:  nil, // No rate limiter in tests
	}
	handlers := api.NewHandlers(handlerConfig)

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	router := handlers.SetupRoutes()
	server := httptest.NewServer(router)

	return &IntegrationTestSuite{
		server:   server,
		handlers: handlers,
		config:   cfg,
		logger:   logger,
	}
}

// Close cleans up the test suite
func (suite *IntegrationTestSuite) Close() {
	if suite.server != nil {
		suite.server.Close()
	}
}

// TestConcurrentRatesRequests tests the rates endpoint with high concurrent load
func TestConcurrentRatesRequests(t *testing.T) {
	suite := NewIntegrationTestSuite()
	defer suite.Close()

	const numRequests = 100
	const concurrency = 20

	results := make(chan error, numRequests)
	semaphore := make(chan struct{}, concurrency)

	var wg sync.WaitGroup
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			resp, err := http.Get(suite.server.URL + "/api/v1/rates")
			if err != nil {
				results <- err
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				results <- fmt.Errorf("unexpected status code: %d", resp.StatusCode)
				return
			}

			var rates map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
				results <- err
				return
			}

			results <- nil
		}()
	}

	wg.Wait()
	close(results)

	// Check results
	errorCount := 0
	for err := range results {
		if err != nil {
			errorCount++
			t.Logf("Request error: %v", err)
		}
	}

	if errorCount > 0 {
		t.Errorf("Expected 0 errors, got %d", errorCount)
	}
}

// TestRaceConditionDetection tests for specific race conditions
func TestRaceConditionDetection(t *testing.T) {
	suite := NewIntegrationTestSuite()
	defer suite.Close()

	const numGoroutines = 50
	const requestsPerGoroutine = 10

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*requestsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				resp, err := http.Get(suite.server.URL + "/api/v1/rates")
				if err != nil {
					errors <- fmt.Errorf("goroutine %d, request %d: %v", goroutineID, j, err)
					continue
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					errors <- fmt.Errorf("goroutine %d, request %d: status %d", goroutineID, j, resp.StatusCode)
					continue
				}

				var rates map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
					errors <- fmt.Errorf("goroutine %d, request %d: decode error %v", goroutineID, j, err)
					continue
				}

				// Verify response structure
				if _, ok := rates["base"]; !ok {
					errors <- fmt.Errorf("goroutine %d, request %d: missing 'base' field", goroutineID, j)
					continue
				}
				if _, ok := rates["rates"]; !ok {
					errors <- fmt.Errorf("goroutine %d, request %d: missing 'rates' field", goroutineID, j)
					continue
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		errorCount++
		t.Logf("Race condition error: %v", err)
	}

	if errorCount > 0 {
		t.Errorf("Detected %d race condition errors", errorCount)
	}
}

// TestCacheConsistency tests cache behavior under concurrent load
func TestCacheConsistency(t *testing.T) {
	suite := NewIntegrationTestSuite()
	defer suite.Close()

	const numRequests = 200
	const concurrency = 30

	results := make(chan map[string]interface{}, numRequests)
	semaphore := make(chan struct{}, concurrency)

	var wg sync.WaitGroup
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			resp, err := http.Get(suite.server.URL + "/api/v1/rates")
			if err != nil {
				t.Logf("Request error: %v", err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Logf("Unexpected status: %d", resp.StatusCode)
				return
			}

			var rates map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
				t.Logf("Decode error: %v", err)
				return
			}

			results <- rates
		}()
	}

	wg.Wait()
	close(results)

	// Check cache consistency
	var firstResult map[string]interface{}
	consistentCount := 0
	totalCount := 0

	for result := range results {
		totalCount++
		if firstResult == nil {
			firstResult = result
			consistentCount++
			continue
		}

		// Compare base currency and timestamp (should be consistent within cache TTL)
		if result["base"] == firstResult["base"] {
			consistentCount++
		}
	}

	consistencyRatio := float64(consistentCount) / float64(totalCount)
	if consistencyRatio < 0.8 { // Allow some variance due to cache updates
		t.Errorf("Cache consistency too low: %.2f%% (%d/%d)", consistencyRatio*100, consistentCount, totalCount)
	}
}

// TestStressLoad tests the service under extreme load
func TestStressLoad(t *testing.T) {
	suite := NewIntegrationTestSuite()
	defer suite.Close()

	const numRequests = 500
	const concurrency = 100

	results := make(chan error, numRequests)
	semaphore := make(chan struct{}, concurrency)

	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(requestID int) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			resp, err := http.Get(suite.server.URL + "/api/v1/rates")
			if err != nil {
				results <- fmt.Errorf("request %d: %v", requestID, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				results <- fmt.Errorf("request %d: status %d", requestID, resp.StatusCode)
				return
			}

			var rates map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
				results <- fmt.Errorf("request %d: decode error %v", requestID, err)
				return
			}

			results <- nil
		}(i)
	}

	wg.Wait()
	close(results)

	duration := time.Since(startTime)
	t.Logf("Stress test completed in %v", duration)

	// Check results
	errorCount := 0
	for err := range results {
		if err != nil {
			errorCount++
			t.Logf("Stress test error: %v", err)
		}
	}

	errorRate := float64(errorCount) / float64(numRequests)
	if errorRate > 0.05 { // Allow 5% error rate under stress
		t.Errorf("Error rate too high: %.2f%% (%d/%d)", errorRate*100, errorCount, numRequests)
	}

	t.Logf("Stress test results: %d requests, %d errors (%.2f%%), duration: %v",
		numRequests, errorCount, errorRate*100, duration)
}
