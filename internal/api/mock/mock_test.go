//go:build mock

package mock

import (
	"currency-exchange-api/internal/api"
	"currency-exchange-api/internal/config"
	"currency-exchange-api/internal/logger"
	"currency-exchange-api/internal/service"
	"currency-exchange-api/internal/testutils"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// MockTestSuite provides shared setup for mock tests
type MockTestSuite struct {
	server                    *httptest.Server
	handlers                  *api.Handlers
	config                    *config.Config
	logger                    *logrus.Logger
	mockExchangeRateServer    *testutils.MockExchangeRateServer
	mockJSONPlaceholderServer *testutils.MockJSONPlaceholderServer
}

// NewMockTestSuite creates a new mock test suite
func NewMockTestSuite() *MockTestSuite {
	// Create mock servers
	mockExchangeRateServer := testutils.NewMockExchangeRateServer()
	mockJSONPlaceholderServer := testutils.NewMockJSONPlaceholderServer()

	// Create test configuration with mock servers
	cfg := testutils.MockConfigWithMocks(mockExchangeRateServer.URL(), mockJSONPlaceholderServer.URL())

	// Create logger
	logger := logger.New("error")

	// Create services
	ratesService := service.NewRatesService(cfg, logger)

	// Create handlers
	handlerConfig := api.HandlerConfig{
		Logger:       logger,
		RatesService: ratesService,
		RateLimiter:  nil, // No rate limiter in mock tests
	}
	handlers := api.NewHandlers(handlerConfig)

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	router := handlers.SetupRoutes()
	server := httptest.NewServer(router)

	return &MockTestSuite{
		server:                    server,
		handlers:                  handlers,
		config:                    cfg,
		logger:                    logger,
		mockExchangeRateServer:    mockExchangeRateServer,
		mockJSONPlaceholderServer: mockJSONPlaceholderServer,
	}
}

// Close cleans up the mock test suite
func (suite *MockTestSuite) Close() {
	if suite.server != nil {
		suite.server.Close()
	}
	if suite.mockExchangeRateServer != nil {
		suite.mockExchangeRateServer.Close()
	}
	if suite.mockJSONPlaceholderServer != nil {
		suite.mockJSONPlaceholderServer.Close()
	}
}

// TestMockRatesEndpoint tests the rates endpoint with mock data
func TestMockRatesEndpoint(t *testing.T) {
	suite := NewMockTestSuite()
	defer suite.Close()

	resp, err := http.Get(suite.server.URL + "/api/v1/rates")
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var rates map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	// Verify response structure
	if _, ok := rates["base"]; !ok {
		t.Error("Response missing 'base' field")
	}
	if _, ok := rates["rates"]; !ok {
		t.Error("Response missing 'rates' field")
	}
	if _, ok := rates["timestamp"]; !ok {
		t.Error("Response missing 'timestamp' field")
	}
}

// TestMockRatesByBaseEndpoint tests the rates by base endpoint with mock data
func TestMockRatesByBaseEndpoint(t *testing.T) {
	suite := NewMockTestSuite()
	defer suite.Close()

	resp, err := http.Get(suite.server.URL + "/api/v1/rates/EUR")
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var rates map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	// Verify response structure
	if base, ok := rates["base"].(string); !ok || base != "EUR" {
		t.Errorf("Expected base 'EUR', got %v", rates["base"])
	}
	if _, ok := rates["rates"]; !ok {
		t.Error("Response missing 'rates' field")
	}
}

// TestMockHealthEndpoint tests the health endpoint with mock data
func TestMockHealthEndpoint(t *testing.T) {
	suite := NewMockTestSuite()
	defer suite.Close()

	resp, err := http.Get(suite.server.URL + "/health")
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	// Verify response structure
	if status, ok := health["status"].(string); !ok || status != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", health["status"])
	}
	if _, ok := health["timestamp"]; !ok {
		t.Error("Response missing 'timestamp' field")
	}
	if _, ok := health["version"]; !ok {
		t.Error("Response missing 'version' field")
	}
	if _, ok := health["uptime"]; !ok {
		t.Error("Response missing 'uptime' field")
	}
}

// TestMockInvalidBaseCurrency tests invalid base currency handling
func TestMockInvalidBaseCurrency(t *testing.T) {
	suite := NewMockTestSuite()
	defer suite.Close()

	resp, err := http.Get(suite.server.URL + "/api/v1/rates/INVALID")
	if err != nil {
		t.Fatalf("Request error: %v", err)
	}
	defer resp.Body.Close()

	// Should still return 200 but with error in response or empty rates
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var rates map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	// Verify response structure
	if _, ok := rates["base"]; !ok {
		t.Error("Response missing 'base' field")
	}
	if _, ok := rates["rates"]; !ok {
		t.Error("Response missing 'rates' field")
	}
}

// TestMockConcurrentRequests tests concurrent requests with mock data
func TestMockConcurrentRequests(t *testing.T) {
	suite := NewMockTestSuite()
	defer suite.Close()

	const numRequests = 50
	const concurrency = 10

	results := make(chan error, numRequests)
	semaphore := make(chan struct{}, concurrency)

	for i := 0; i < numRequests; i++ {
		go func(requestID int) {
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			resp, err := http.Get(suite.server.URL + "/api/v1/rates")
			if err != nil {
				results <- err
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				results <- err
				return
			}

			var rates map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&rates); err != nil {
				results <- err
				return
			}

			results <- nil
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		if err := <-results; err != nil {
			t.Errorf("Concurrent request error: %v", err)
		}
	}
}
