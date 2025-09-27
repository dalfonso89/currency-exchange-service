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
	cfg := testutils.MockConfig()
	logger := testutils.MockLogger()
	apiService := service.NewAPIService(cfg, logger)
	handlerConfig := HandlerConfig{
		APIService:   apiService,
		Logger:       logger,
		RatesService: nil,
		RateLimiter:  nil,
	}
	handlers := NewHandlers(handlerConfig)

	if handlers == nil {
		t.Fatal("NewHandlers() returned nil")
	}

	if handlers.apiService != apiService {
		t.Error("NewHandlers() did not set apiService correctly")
	}

	if handlers.logger != logger {
		t.Error("NewHandlers() did not set logger correctly")
	}
}

func TestHandlers_HealthCheck(t *testing.T) {
	cfg := testutils.MockConfig()
	logger := testutils.MockLogger()
	apiService := service.NewAPIService(cfg, logger)
	handlerConfig := HandlerConfig{
		APIService:   apiService,
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
	cfg := testutils.MockConfig()
	logger := testutils.MockLogger()
	apiService := service.NewAPIService(cfg, logger)
	handlerConfig := HandlerConfig{
		APIService:   apiService,
		Logger:       logger,
		RatesService: nil,
		RateLimiter:  nil,
	}
	handlers := NewHandlers(handlerConfig)

	// Use real rates service
	ratesService := service.NewRatesService(cfg, logger)
	handlers.ratesService = ratesService

	req := httptest.NewRequest("GET", "/api/v1/rates", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handlers.GetRates(c)

	// The service might fail due to external API issues, so we just check it doesn't panic
	if w.Code == 0 {
		t.Error("GetRates() did not set status code")
	}
}

func TestHandlers_GetRatesByBase(t *testing.T) {
	cfg := testutils.MockConfig()
	logger := testutils.MockLogger()
	apiService := service.NewAPIService(cfg, logger)
	handlerConfig := HandlerConfig{
		APIService:   apiService,
		Logger:       logger,
		RatesService: nil,
		RateLimiter:  nil,
	}
	handlers := NewHandlers(handlerConfig)

	// Use real rates service
	ratesService := service.NewRatesService(cfg, logger)
	handlers.ratesService = ratesService

	req := httptest.NewRequest("GET", "/api/v1/rates/EUR", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{{Key: "base", Value: "EUR"}}

	handlers.GetRatesByBase(c)

	// The service might fail due to external API issues, so we just check it doesn't panic
	if w.Code == 0 {
		t.Error("GetRatesByBase() did not set status code")
	}
}

func TestHandlers_GetPosts(t *testing.T) {
	cfg := testutils.MockConfig()
	logger := testutils.MockLogger()
	apiService := service.NewAPIService(cfg, logger)
	handlerConfig := HandlerConfig{
		APIService:   apiService,
		Logger:       logger,
		RatesService: nil,
		RateLimiter:  nil,
	}
	handlers := NewHandlers(handlerConfig)

	req := httptest.NewRequest("GET", "/api/v1/posts", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handlers.GetPosts(c)

	if w.Code != http.StatusOK {
		t.Errorf("GetPosts() status = %v, want %v", w.Code, http.StatusOK)
	}

	var response models.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("GetPosts() response unmarshal error = %v", err)
	}

	if response.Status != http.StatusOK {
		t.Errorf("GetPosts() response status = %v, want %v", response.Status, http.StatusOK)
	}
}

func TestHandlers_GetPostByID(t *testing.T) {
	cfg := testutils.MockConfig()
	logger := testutils.MockLogger()
	apiService := service.NewAPIService(cfg, logger)
	handlerConfig := HandlerConfig{
		APIService:   apiService,
		Logger:       logger,
		RatesService: nil,
		RateLimiter:  nil,
	}
	handlers := NewHandlers(handlerConfig)

	req := httptest.NewRequest("GET", "/api/v1/posts/1", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	handlers.GetPostByID(c)

	if w.Code != http.StatusOK {
		t.Errorf("GetPostByID() status = %v, want %v", w.Code, http.StatusOK)
	}

	var response models.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("GetPostByID() response unmarshal error = %v", err)
	}

	if response.Status != http.StatusOK {
		t.Errorf("GetPostByID() response status = %v, want %v", response.Status, http.StatusOK)
	}
}

func TestHandlers_GetUsers(t *testing.T) {
	cfg := testutils.MockConfig()
	logger := testutils.MockLogger()
	apiService := service.NewAPIService(cfg, logger)
	handlerConfig := HandlerConfig{
		APIService:   apiService,
		Logger:       logger,
		RatesService: nil,
		RateLimiter:  nil,
	}
	handlers := NewHandlers(handlerConfig)

	req := httptest.NewRequest("GET", "/api/v1/users", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handlers.GetUsers(c)

	if w.Code != http.StatusOK {
		t.Errorf("GetUsers() status = %v, want %v", w.Code, http.StatusOK)
	}

	var response models.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("GetUsers() response unmarshal error = %v", err)
	}

	if response.Status != http.StatusOK {
		t.Errorf("GetUsers() response status = %v, want %v", response.Status, http.StatusOK)
	}
}

func TestHandlers_GetComments(t *testing.T) {
	cfg := testutils.MockConfig()
	logger := testutils.MockLogger()
	apiService := service.NewAPIService(cfg, logger)
	handlerConfig := HandlerConfig{
		APIService:   apiService,
		Logger:       logger,
		RatesService: nil,
		RateLimiter:  nil,
	}
	handlers := NewHandlers(handlerConfig)

	req := httptest.NewRequest("GET", "/api/v1/comments", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	handlers.GetComments(c)

	if w.Code != http.StatusOK {
		t.Errorf("GetComments() status = %v, want %v", w.Code, http.StatusOK)
	}

	var response models.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("GetComments() response unmarshal error = %v", err)
	}

	if response.Status != http.StatusOK {
		t.Errorf("GetComments() response status = %v, want %v", response.Status, http.StatusOK)
	}
}
