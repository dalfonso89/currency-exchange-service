package api

import (
	"context"
	"currency-exchange-api/internal/models"
	"currency-exchange-api/internal/service"
	"currency-exchange-api/internal/testutils"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// MockAPIService is a mock implementation of APIService for testing
type MockAPIService struct {
	healthError   error
	posts         []map[string]interface{}
	post          map[string]interface{}
	users         []map[string]interface{}
	comments      []map[string]interface{}
	postError     error
	usersError    error
	commentsError error
}

func (m *MockAPIService) HealthCheck(ctx context.Context) error {
	return m.healthError
}

func (m *MockAPIService) FetchPosts(ctx context.Context) ([]map[string]interface{}, error) {
	return m.posts, nil
}

func (m *MockAPIService) FetchPostByID(ctx context.Context, id int) (map[string]interface{}, error) {
	if m.postError != nil {
		return nil, m.postError
	}
	return m.post, nil
}

func (m *MockAPIService) FetchUsers(ctx context.Context) ([]map[string]interface{}, error) {
	return m.users, m.usersError
}

func (m *MockAPIService) FetchComments(ctx context.Context) ([]map[string]interface{}, error) {
	return m.comments, m.commentsError
}

func TestNewHandlers(t *testing.T) {
	logger := testutils.MockLogger()
	handlerConfig := HandlerConfig{
		Logger:       logger,
		RatesService: nil,
		RateLimiter:  nil,
	}
	handlers := NewHandlers(handlerConfig)

	if handlers == nil {
		t.Fatal("NewHandlers() returned nil")
	}

	// APIService removed - no longer needed

	if handlers.logger != logger {
		t.Error("NewHandlers() did not set logger correctly")
	}
}

func TestHandlers_HealthCheck(t *testing.T) {
	logger := testutils.MockLogger()
	handlerConfig := HandlerConfig{
		Logger:       logger,
		RatesService: nil,
		RateLimiter:  nil,
	}
	handlers := NewHandlers(handlerConfig)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handlers.HealthCheck(c)

	if w.Code != http.StatusOK {
		t.Errorf("HealthCheck() status = %v, want %v", w.Code, http.StatusOK)
	}

	var response models.HealthCheck
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("HealthCheck() response unmarshal error = %v", err)
	}

	if response.Status == "" {
		t.Error("HealthCheck() response missing status")
	}
	if response.Version == "" {
		t.Error("HealthCheck() response missing version")
	}
	if response.Uptime == "" {
		t.Error("HealthCheck() response missing uptime")
	}
}

func TestHandlers_GetRates(t *testing.T) {
	// Create mock servers
	mockExchangeRateServer := testutils.NewMockExchangeRateServer()
	defer mockExchangeRateServer.Close()
	mockJSONPlaceholderServer := testutils.NewMockJSONPlaceholderServer()
	defer mockJSONPlaceholderServer.Close()

	// Create test configuration with mock servers
	cfg := testutils.MockConfigWithMocks(mockExchangeRateServer.URL(), mockJSONPlaceholderServer.URL())
	logger := testutils.MockLogger()
	handlerConfig := HandlerConfig{
		Logger:       logger,
		RatesService: nil,
		RateLimiter:  nil,
	}
	handlers := NewHandlers(handlerConfig)

	// Use rates service with mock servers
	ratesService := service.NewRatesService(cfg, logger)
	handlers.ratesService = ratesService

	req := httptest.NewRequest("GET", "/api/v1/rates", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handlers.GetRates(c)

	// Should succeed with mock servers
	if w.Code != http.StatusOK {
		t.Errorf("GetRates() status code = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestHandlers_GetRatesByBase(t *testing.T) {
	// Create mock servers
	mockExchangeRateServer := testutils.NewMockExchangeRateServer()
	defer mockExchangeRateServer.Close()
	mockJSONPlaceholderServer := testutils.NewMockJSONPlaceholderServer()
	defer mockJSONPlaceholderServer.Close()

	// Create test configuration with mock servers
	cfg := testutils.MockConfigWithMocks(mockExchangeRateServer.URL(), mockJSONPlaceholderServer.URL())
	logger := testutils.MockLogger()
	handlerConfig := HandlerConfig{
		Logger:       logger,
		RatesService: nil,
		RateLimiter:  nil,
	}
	handlers := NewHandlers(handlerConfig)

	// Use rates service with mock servers
	ratesService := service.NewRatesService(cfg, logger)
	handlers.ratesService = ratesService

	req := httptest.NewRequest("GET", "/api/v1/rates/EUR", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{{Key: "base", Value: "EUR"}}

	handlers.GetRatesByBase(c)

	// Should succeed with mock servers
	if w.Code != http.StatusOK {
		t.Errorf("GetRatesByBase() status code = %v, want %v", w.Code, http.StatusOK)
	}
}
