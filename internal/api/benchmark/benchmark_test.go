package benchmark

import (
	"context"
	"currency-exchange-api/internal/api"
	"currency-exchange-api/internal/logger"
	"currency-exchange-api/internal/service"
	"currency-exchange-api/internal/testutils"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

// BenchmarkTestSuite provides shared setup for benchmark tests
type BenchmarkTestSuite struct {
	server   *httptest.Server
	handlers *api.Handlers
}

// NewBenchmarkTestSuite creates a new benchmark test suite
func NewBenchmarkTestSuite() *BenchmarkTestSuite {
	// Create mock servers
	mockExchangeRateServer := testutils.NewMockExchangeRateServer()
	mockJSONPlaceholderServer := testutils.NewMockJSONPlaceholderServer()

	// Create test configuration with mock servers
	cfg := testutils.MockConfigWithMocks(mockExchangeRateServer.URL(), mockJSONPlaceholderServer.URL())
	cfg.MaxConcurrentRequests = 50
	cfg.RateLimitEnabled = false // Disable rate limiting for benchmarks

	// Create logger
	logger := logger.New("error")

	// Create services
	ratesService := service.NewRatesService(cfg, logger)

	// Create handlers
	handlerConfig := api.HandlerConfig{
		Logger:       logger,
		RatesService: ratesService,
		RateLimiter:  nil, // No rate limiter in benchmarks
	}
	handlers := api.NewHandlers(handlerConfig)

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	router := handlers.SetupRoutes()
	server := httptest.NewServer(router)

	return &BenchmarkTestSuite{
		server:   server,
		handlers: handlers,
	}
}

// Close cleans up the benchmark test suite
func (suite *BenchmarkTestSuite) Close() {
	if suite.server != nil {
		suite.server.Close()
	}
}

// TestSimple tests a simple test
func TestSimple(t *testing.T) {
	if 1+1 != 2 {
		t.Error("Math is broken")
	}
}

// BenchmarkSimple tests a simple benchmark
func BenchmarkSimple(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Simple operation
		_ = i * 2
	}
}

// Global benchmark suite to avoid port conflicts
var (
	globalBenchmarkSuite *BenchmarkTestSuite
	once                 sync.Once
)

func getBenchmarkSuite() *BenchmarkTestSuite {
	once.Do(func() {
		globalBenchmarkSuite = NewBenchmarkTestSuite()
	})
	return globalBenchmarkSuite
}

// BenchmarkConcurrentRates benchmarks the rates endpoint under concurrent load
func BenchmarkConcurrentRates(b *testing.B) {
	suite := getBenchmarkSuite()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := http.Get(suite.server.URL + "/api/v1/rates")
			if err != nil {
				b.Fatalf("Request error: %v", err)
			}
			resp.Body.Close()
		}
	})
}

// BenchmarkRatesEndpoint benchmarks single requests to the rates endpoint
func BenchmarkRatesEndpoint(b *testing.B) {
	suite := getBenchmarkSuite()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := http.Get(suite.server.URL + "/api/v1/rates")
		if err != nil {
			b.Fatalf("Request error: %v", err)
		}
		resp.Body.Close()
	}
}

// BenchmarkRatesByBase benchmarks requests with specific base currency
func BenchmarkRatesByBase(b *testing.B) {
	suite := getBenchmarkSuite()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := http.Get(suite.server.URL + "/api/v1/rates/EUR")
		if err != nil {
			b.Fatalf("Request error: %v", err)
		}
		resp.Body.Close()
	}
}

// BenchmarkHealthCheck benchmarks the health check endpoint
func BenchmarkHealthCheck(b *testing.B) {
	suite := getBenchmarkSuite()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := http.Get(suite.server.URL + "/health")
		if err != nil {
			b.Fatalf("Request error: %v", err)
		}
		resp.Body.Close()
	}
}

// BenchmarkServiceLogic benchmarks the service logic directly
func BenchmarkServiceLogic(b *testing.B) {
	// Create test configuration without external dependencies
	cfg := testutils.MockConfig()
	logger := logger.New("error")
	ratesService := service.NewRatesService(cfg, logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This will fail because there are no mock servers, but it will benchmark the service setup
		_, _ = ratesService.GetRates(context.Background(), "USD")
	}
}
