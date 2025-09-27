package api

import (
	"context"
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
)

// IntegrationTestSuite provides comprehensive integration testing with concurrent load testing
type IntegrationTestSuite struct {
	server                    *httptest.Server
	handlers                  *Handlers
	config                    *config.Config
	logger                    *logger.Logger
	mockExchangeRateServer    *testutils.MockExchangeRateServer
	mockJSONPlaceholderServer *testutils.MockJSONPlaceholderServer
}

// NewIntegrationTestSuite creates a new integration test suite
func NewIntegrationTestSuite() *IntegrationTestSuite {
	// Create mock servers
	mockExchangeRateServer := testutils.NewMockExchangeRateServer()
	mockJSONPlaceholderServer := testutils.NewMockJSONPlaceholderServer()

	// Create test configuration with mock servers
	cfg := testutils.MockConfigWithMocks(mockExchangeRateServer.URL(), mockJSONPlaceholderServer.URL())

	// Create logger
	logger := logger.New("error")

	// Create services
	apiService := service.NewAPIService(cfg, logger)
	ratesService := service.NewRatesService(cfg, logger)

	// Create handlers
	handlers := NewHandlers(apiService, logger).WithRates(ratesService)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := handlers.SetupRoutes()

	// Create test server
	server := httptest.NewServer(router)

	return &IntegrationTestSuite{
		server:                    server,
		handlers:                  handlers,
		config:                    cfg,
		logger:                    logger,
		mockExchangeRateServer:    mockExchangeRateServer,
		mockJSONPlaceholderServer: mockJSONPlaceholderServer,
	}
}

// Close cleans up the test suite
func (its *IntegrationTestSuite) Close() {
	its.server.Close()
	its.mockExchangeRateServer.Close()
	its.mockJSONPlaceholderServer.Close()
}

// TestConcurrentRatesRequests tests the rates endpoint with high concurrent load
func TestConcurrentRatesRequests(t *testing.T) {
	suite := NewIntegrationTestSuite()
	defer suite.Close()

	const (
		numGoroutines        = 50
		requestsPerGoroutine = 10
	)

	var wg sync.WaitGroup
	results := make(chan TestResult, numGoroutines*requestsPerGoroutine)
	errors := make(chan error, numGoroutines*requestsPerGoroutine)

	startTime := time.Now()

	// Launch concurrent goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < requestsPerGoroutine; j++ {
				start := time.Now()

				resp, err := http.Get(suite.server.URL + "/api/v1/rates")
				if err != nil {
					errors <- fmt.Errorf("goroutine %d request %d failed: %w", goroutineID, j, err)
					continue
				}

				duration := time.Since(start)

				// Read response body
				var response map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
					errors <- fmt.Errorf("goroutine %d request %d decode failed: %w", goroutineID, j, err)
					resp.Body.Close()
					continue
				}
				resp.Body.Close()

				results <- TestResult{
					GoroutineID: goroutineID,
					RequestID:   j,
					StatusCode:  resp.StatusCode,
					Duration:    duration,
					Success:     resp.StatusCode == http.StatusOK,
					Response:    response,
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(results)
	close(errors)

	totalDuration := time.Since(startTime)

	// Collect results
	var successfulRequests, failedRequests int
	var totalResponseTime time.Duration
	var maxResponseTime, minResponseTime time.Duration
	firstResponseTime := true

	for result := range results {
		if result.Success {
			successfulRequests++
		} else {
			failedRequests++
		}

		totalResponseTime += result.Duration

		if firstResponseTime {
			maxResponseTime = result.Duration
			minResponseTime = result.Duration
			firstResponseTime = false
		} else {
			if result.Duration > maxResponseTime {
				maxResponseTime = result.Duration
			}
			if result.Duration < minResponseTime {
				minResponseTime = result.Duration
			}
		}
	}

	// Check for errors
	var errorCount int
	for err := range errors {
		t.Logf("Request error: %v", err)
		errorCount++
	}

	// Calculate metrics
	totalRequests := numGoroutines * requestsPerGoroutine
	successRate := float64(successfulRequests) / float64(totalRequests) * 100
	avgResponseTime := totalResponseTime / time.Duration(totalRequests)
	requestsPerSecond := float64(totalRequests) / totalDuration.Seconds()

	// Log test results
	t.Logf("=== Concurrent Load Test Results ===")
	t.Logf("Total Requests: %d", totalRequests)
	t.Logf("Successful Requests: %d (%.2f%%)", successfulRequests, successRate)
	t.Logf("Failed Requests: %d", failedRequests)
	t.Logf("Errors: %d", errorCount)
	t.Logf("Total Duration: %v", totalDuration)
	t.Logf("Average Response Time: %v", avgResponseTime)
	t.Logf("Min Response Time: %v", minResponseTime)
	t.Logf("Max Response Time: %v", maxResponseTime)
	t.Logf("Requests per Second: %.2f", requestsPerSecond)

	// Assertions
	if successRate < 80.0 {
		t.Errorf("Success rate too low: %.2f%% (expected >= 80%%)", successRate)
	}

	if avgResponseTime > 10*time.Second {
		t.Errorf("Average response time too high: %v (expected < 10s)", avgResponseTime)
	}

	if errorCount > totalRequests/10 {
		t.Errorf("Too many errors: %d (expected < %d)", errorCount, totalRequests/10)
	}
}

// TestRaceConditionDetection tests for specific race conditions
func TestRaceConditionDetection(t *testing.T) {
	suite := NewIntegrationTestSuite()
	defer suite.Close()

	const (
		numGoroutines        = 20
		requestsPerGoroutine = 5
	)

	var wg sync.WaitGroup
	responses := make(chan map[string]interface{}, numGoroutines*requestsPerGoroutine)
	errors := make(chan error, numGoroutines*requestsPerGoroutine)

	// Test concurrent access to the same endpoint
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < requestsPerGoroutine; j++ {
				resp, err := http.Get(suite.server.URL + "/api/v1/rates")
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

				responses <- response
			}
		}(i)
	}

	wg.Wait()
	close(responses)
	close(errors)

	// Check for errors
	var errorCount int
	for err := range errors {
		t.Logf("Race condition test error: %v", err)
		errorCount++
	}

	// Validate response consistency
	var responsesList []map[string]interface{}
	for response := range responses {
		responsesList = append(responsesList, response)
	}

	// Check that all responses have the expected structure
	for i, response := range responsesList {
		if _, ok := response["base"]; !ok {
			t.Errorf("Response %d missing 'base' field", i)
		}
		if _, ok := response["rates"]; !ok {
			t.Errorf("Response %d missing 'rates' field", i)
		}
		if _, ok := response["timestamp"]; !ok {
			t.Errorf("Response %d missing 'timestamp' field", i)
		}
	}

	if errorCount > 0 {
		t.Errorf("Race condition test had %d errors", errorCount)
	}

	t.Logf("Race condition test completed with %d successful responses", len(responsesList))
}

// TestCacheConsistency tests cache behavior under concurrent load
func TestCacheConsistency(t *testing.T) {
	suite := NewIntegrationTestSuite()
	defer suite.Close()

	const numRequests = 30

	var wg sync.WaitGroup
	responses := make(chan map[string]interface{}, numRequests)
	errors := make(chan error, numRequests)

	// Make concurrent requests to the same endpoint (should hit cache)
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(requestID int) {
			defer wg.Done()

			resp, err := http.Get(suite.server.URL + "/api/v1/rates")
			if err != nil {
				errors <- fmt.Errorf("request %d failed: %w", requestID, err)
				return
			}

			var response map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
				errors <- fmt.Errorf("request %d decode failed: %w", requestID, err)
				resp.Body.Close()
				return
			}
			resp.Body.Close()

			responses <- response
		}(i)
	}

	wg.Wait()
	close(responses)
	close(errors)

	// Check for errors
	var errorCount int
	for err := range errors {
		t.Logf("Cache consistency test error: %v", err)
		errorCount++
	}

	// Validate cache consistency
	var responsesList []map[string]interface{}
	for response := range responses {
		responsesList = append(responsesList, response)
	}

	// All responses should have the same base currency and similar timestamps
	if len(responsesList) > 1 {
		firstResponse := responsesList[0]
		firstBase := firstResponse["base"]
		firstTimestamp := firstResponse["timestamp"]

		for i, response := range responsesList[1:] {
			if response["base"] != firstBase {
				t.Errorf("Response %d has different base currency: %v vs %v", i+1, response["base"], firstBase)
			}

			// Timestamps should be close (within 1 second due to cache)
			if timestamp, ok := response["timestamp"].(float64); ok {
				if firstTimestampFloat, ok := firstTimestamp.(float64); ok {
					timeDiff := timestamp - firstTimestampFloat
					if timeDiff > 1.0 { // More than 1 second difference
						t.Errorf("Response %d has significantly different timestamp: %v vs %v", i+1, timestamp, firstTimestampFloat)
					}
				}
			}
		}
	}

	if errorCount > 0 {
		t.Errorf("Cache consistency test had %d errors", errorCount)
	}

	t.Logf("Cache consistency test completed with %d successful responses", len(responsesList))
}

// TestStressLoad tests the service under extreme load
func TestStressLoad(t *testing.T) {
	suite := NewIntegrationTestSuite()
	defer suite.Close()

	const (
		numGoroutines        = 100
		requestsPerGoroutine = 20
		timeout              = 30 * time.Second
	)

	var wg sync.WaitGroup
	results := make(chan TestResult, numGoroutines*requestsPerGoroutine)
	errors := make(chan error, numGoroutines*requestsPerGoroutine)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	startTime := time.Now()

	// Launch stress test goroutines
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < requestsPerGoroutine; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				start := time.Now()

				req, err := http.NewRequestWithContext(ctx, "GET", suite.server.URL+"/api/v1/rates", nil)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d request %d create failed: %w", goroutineID, j, err)
					continue
				}

				client := &http.Client{Timeout: 10 * time.Second}
				resp, err := client.Do(req)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d request %d failed: %w", goroutineID, j, err)
					continue
				}

				duration := time.Since(start)

				var response map[string]interface{}
				if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
					errors <- fmt.Errorf("goroutine %d request %d decode failed: %w", goroutineID, j, err)
					resp.Body.Close()
					continue
				}
				resp.Body.Close()

				results <- TestResult{
					GoroutineID: goroutineID,
					RequestID:   j,
					StatusCode:  resp.StatusCode,
					Duration:    duration,
					Success:     resp.StatusCode == http.StatusOK,
					Response:    response,
				}
			}
		}(i)
	}

	// Wait for completion or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Log("Stress test completed within timeout")
	case <-ctx.Done():
		t.Log("Stress test timed out")
	}

	close(results)
	close(errors)

	totalDuration := time.Since(startTime)

	// Collect results
	var successfulRequests, failedRequests int
	var totalResponseTime time.Duration
	var maxResponseTime, minResponseTime time.Duration
	firstResponseTime := true

	for result := range results {
		if result.Success {
			successfulRequests++
		} else {
			failedRequests++
		}

		totalResponseTime += result.Duration

		if firstResponseTime {
			maxResponseTime = result.Duration
			minResponseTime = result.Duration
			firstResponseTime = false
		} else {
			if result.Duration > maxResponseTime {
				maxResponseTime = result.Duration
			}
			if result.Duration < minResponseTime {
				minResponseTime = result.Duration
			}
		}
	}

	// Check for errors
	var errorCount int
	for range errors {
		errorCount++
	}

	// Calculate metrics
	totalRequests := successfulRequests + failedRequests
	if totalRequests > 0 {
		successRate := float64(successfulRequests) / float64(totalRequests) * 100
		avgResponseTime := totalResponseTime / time.Duration(totalRequests)
		requestsPerSecond := float64(totalRequests) / totalDuration.Seconds()

		// Log stress test results
		t.Logf("=== Stress Test Results ===")
		t.Logf("Total Requests: %d", totalRequests)
		t.Logf("Successful Requests: %d (%.2f%%)", successfulRequests, successRate)
		t.Logf("Failed Requests: %d", failedRequests)
		t.Logf("Errors: %d", errorCount)
		t.Logf("Total Duration: %v", totalDuration)
		t.Logf("Average Response Time: %v", avgResponseTime)
		t.Logf("Min Response Time: %v", minResponseTime)
		t.Logf("Max Response Time: %v", maxResponseTime)
		t.Logf("Requests per Second: %.2f", requestsPerSecond)

		// Stress test assertions (more lenient than load test)
		if successRate < 60.0 {
			t.Errorf("Stress test success rate too low: %.2f%% (expected >= 60%%)", successRate)
		}

		if avgResponseTime > 15*time.Second {
			t.Errorf("Stress test average response time too high: %v (expected < 15s)", avgResponseTime)
		}
	}
}

// TestResult represents the result of a single test request
type TestResult struct {
	GoroutineID int
	RequestID   int
	StatusCode  int
	Duration    time.Duration
	Success     bool
	Response    map[string]interface{}
}

// BenchmarkConcurrentRates benchmarks the rates endpoint under concurrent load
func BenchmarkConcurrentRates(b *testing.B) {
	suite := NewIntegrationTestSuite()
	defer suite.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := http.Get(suite.server.URL + "/api/v1/rates")
			if err != nil {
				b.Errorf("Request failed: %v", err)
				continue
			}
			resp.Body.Close()
		}
	})
}

// BenchmarkRatesEndpoint benchmarks single requests to the rates endpoint
func BenchmarkRatesEndpoint(b *testing.B) {
	suite := NewIntegrationTestSuite()
	defer suite.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := http.Get(suite.server.URL + "/api/v1/rates")
		if err != nil {
			b.Errorf("Request failed: %v", err)
			continue
		}
		resp.Body.Close()
	}
}
