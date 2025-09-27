package race

import (
	"currency-exchange-api/internal/api"
	"currency-exchange-api/internal/config"
	"currency-exchange-api/internal/logger"
	"currency-exchange-api/internal/service"
	"currency-exchange-api/internal/testutils"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// RaceTestSuite provides shared setup for race condition tests
type RaceTestSuite struct {
	server   *httptest.Server
	handlers *api.Handlers
	config   *config.Config
	logger   *logrus.Logger
}

// NewRaceTestSuite creates a new race condition test suite
func NewRaceTestSuite() *RaceTestSuite {
	// Create mock servers
	mockExchangeRateServer := testutils.NewMockExchangeRateServer()
	mockJSONPlaceholderServer := testutils.NewMockJSONPlaceholderServer()

	// Create test configuration with mock servers
	cfg := testutils.MockConfigWithMocks(mockExchangeRateServer.URL(), mockJSONPlaceholderServer.URL())
	cfg.MaxConcurrentRequests = 100 // Increase concurrent request limit
	cfg.RateLimitEnabled = false    // Disable rate limiting for race tests

	// Create logger
	logger := logger.New("error")

	// Create services
	ratesService := service.NewRatesService(cfg, logger)

	// Create handlers
	handlerConfig := api.HandlerConfig{
		Logger:       logger,
		RatesService: ratesService,
		RateLimiter:  nil, // No rate limiter in race tests
	}
	handlers := api.NewHandlers(handlerConfig)

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	router := handlers.SetupRoutes()
	server := httptest.NewServer(router)

	return &RaceTestSuite{
		server:   server,
		handlers: handlers,
		config:   cfg,
		logger:   logger,
	}
}

// Close cleans up the race test suite
func (suite *RaceTestSuite) Close() {
	if suite.server != nil {
		suite.server.Close()
	}
}

// TestConcurrentRatesAccess tests concurrent access to rates endpoint
func TestConcurrentRatesAccess(t *testing.T) {
	suite := NewRaceTestSuite()
	defer suite.Close()

	const numGoroutines = 100
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

// TestConcurrentHealthChecks tests concurrent access to health endpoint
func TestConcurrentHealthChecks(t *testing.T) {
	suite := NewRaceTestSuite()
	defer suite.Close()

	const numGoroutines = 50
	const requestsPerGoroutine = 20

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*requestsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				resp, err := http.Get(suite.server.URL + "/health")
				if err != nil {
					errors <- fmt.Errorf("goroutine %d, request %d: %v", goroutineID, j, err)
					continue
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					errors <- fmt.Errorf("goroutine %d, request %d: status %d", goroutineID, j, resp.StatusCode)
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
		t.Logf("Health check race condition error: %v", err)
	}

	if errorCount > 0 {
		t.Errorf("Detected %d health check race condition errors", errorCount)
	}
}

// TestConcurrentDifferentEndpoints tests concurrent access to different endpoints
func TestConcurrentDifferentEndpoints(t *testing.T) {
	suite := NewRaceTestSuite()
	defer suite.Close()

	const numGoroutines = 30
	const requestsPerGoroutine = 15

	endpoints := []string{
		"/api/v1/rates",
		"/api/v1/rates/USD",
		"/api/v1/rates/EUR",
		"/health",
	}

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*requestsPerGoroutine*len(endpoints))

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				for _, endpoint := range endpoints {
					resp, err := http.Get(suite.server.URL + endpoint)
					if err != nil {
						errors <- fmt.Errorf("goroutine %d, request %d, endpoint %s: %v", goroutineID, j, endpoint, err)
						continue
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						errors <- fmt.Errorf("goroutine %d, request %d, endpoint %s: status %d", goroutineID, j, endpoint, resp.StatusCode)
						continue
					}
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
		t.Logf("Multi-endpoint race condition error: %v", err)
	}

	if errorCount > 0 {
		t.Errorf("Detected %d multi-endpoint race condition errors", errorCount)
	}
}

// TestRapidSequentialRequests tests rapid sequential requests to detect race conditions
func TestRapidSequentialRequests(t *testing.T) {
	suite := NewRaceTestSuite()
	defer suite.Close()

	const numRequests = 100
	const delayBetweenRequests = time.Millisecond

	errors := make(chan error, numRequests)

	// Create HTTP client with proper configuration
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	for i := 0; i < numRequests; i++ {
		go func(requestID int) {
			resp, err := client.Get(suite.server.URL + "/api/v1/rates")
			if err != nil {
				errors <- fmt.Errorf("request %d: %v", requestID, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				errors <- fmt.Errorf("request %d: status %d", requestID, resp.StatusCode)
				return
			}

			errors <- nil
		}(i)

		// Small delay to create rapid but not simultaneous requests
		time.Sleep(delayBetweenRequests)
	}

	// Wait for all requests to complete
	time.Sleep(5 * time.Second)
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		if err != nil {
			errorCount++
			t.Logf("Rapid sequential request error: %v", err)
		}
	}

	if errorCount > 0 {
		t.Errorf("Detected %d rapid sequential request errors", errorCount)
	}
}
