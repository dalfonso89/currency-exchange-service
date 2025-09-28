package testutils

import (
	"currency-exchange-api/config"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"
)

// MockExchangeRateServer creates a mock HTTP server for exchange rate APIs
type MockExchangeRateServer struct {
	server    *httptest.Server
	responses map[string]ExchangeRateResponse
}

// ExchangeRateResponse represents a mock exchange rate API response
type ExchangeRateResponse struct {
	Base      string             `json:"base"`
	Timestamp int64              `json:"timestamp"`
	Rates     map[string]float64 `json:"rates"`
	Provider  string             `json:"provider,omitempty"`
}

// NewMockExchangeRateServer creates a new mock exchange rate server
func NewMockExchangeRateServer() *MockExchangeRateServer {
	mock := &MockExchangeRateServer{
		responses: make(map[string]ExchangeRateResponse),
	}

	// Set up default responses for different providers
	mock.SetupDefaultResponses()

	mock.server = httptest.NewServer(http.HandlerFunc(mock.handler))
	return mock
}

// SetupDefaultResponses sets up default mock responses for different providers
func (m *MockExchangeRateServer) SetupDefaultResponses() {
	// Exchange Rate API response format
	m.responses["/USD"] = ExchangeRateResponse{
		Base:      "USD",
		Timestamp: time.Now().Unix(),
		Rates: map[string]float64{
			"EUR": 0.85,
			"GBP": 0.73,
			"JPY": 110.0,
			"CAD": 1.25,
			"AUD": 1.35,
		},
		Provider: "EXCHANGE_RATE_API",
	}

	// Open Exchange Rates response format
	m.responses["/openexchangerates"] = ExchangeRateResponse{
		Base:      "USD",
		Timestamp: time.Now().Unix(),
		Rates: map[string]float64{
			"EUR": 0.85,
			"GBP": 0.73,
			"JPY": 110.0,
			"CAD": 1.25,
			"AUD": 1.35,
		},
		Provider: "OPEN_EXCHANGE_RATES",
	}

	// Frankfurter API response format
	m.responses["/frankfurter"] = ExchangeRateResponse{
		Base:      "USD",
		Timestamp: time.Now().Unix(),
		Rates: map[string]float64{
			"EUR": 0.85,
			"GBP": 0.73,
			"JPY": 110.0,
			"CAD": 1.25,
			"AUD": 1.35,
		},
		Provider: "FRANKFURTER_API",
	}

	// Exchange Rate Host response format
	m.responses["/exchangeratehost"] = ExchangeRateResponse{
		Base:      "USD",
		Timestamp: time.Now().Unix(),
		Rates: map[string]float64{
			"EUR": 0.85,
			"GBP": 0.73,
			"JPY": 110.0,
			"CAD": 1.25,
			"AUD": 1.35,
		},
		Provider: "EXCHANGE_RATE_HOST",
	}
}

// handler handles HTTP requests to the mock server
func (m *MockExchangeRateServer) handler(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	// Handle HTTP method using type switch
	switch r.Method {
	case "OPTIONS":
		w.WriteHeader(http.StatusOK)
		return
	case "GET":
		// Continue processing
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Determine response based on URL path
	var response ExchangeRateResponse
	var found bool

	path := r.URL.Path
	query := r.URL.Query()

	// Handle different API formats
	if path == "/USD" || path == "/latest" {
		response, found = m.responses["/USD"]
	} else if query.Get("app_id") != "" {
		// Handle openexchangerates with dynamic base currency
		baseCurrency := query.Get("base")
		if baseCurrency == "" {
			baseCurrency = "USD" // Default to USD
		}
		response = ExchangeRateResponse{
			Base:      baseCurrency,
			Timestamp: time.Now().Unix(),
			Rates: map[string]float64{
				"USD": 1.0,
				"EUR": 0.85,
				"GBP": 0.73,
				"JPY": 110.0,
				"CAD": 1.25,
				"AUD": 1.35,
			},
			Provider: "openexchangerates",
		}
		found = true
	} else if path == "/latest" && query.Get("base") != "" {
		response, found = m.responses["/frankfurter"]
	} else if query.Get("base") != "" {
		response, found = m.responses["/exchangeratehost"]
	} else if len(path) > 1 && path[0] == '/' {
		// Handle dynamic base currency (e.g., /EUR, /GBP, etc.)
		baseCurrency := path[1:] // Remove leading slash

		// For erapi provider, return the correct format
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		erapiResponse := map[string]interface{}{
			"base_code":             baseCurrency,
			"time_last_update_unix": time.Now().Unix(),
			"rates": map[string]float64{
				"USD": 1.0,
				"EUR": 0.85,
				"GBP": 0.73,
				"JPY": 110.0,
				"CAD": 1.25,
				"AUD": 1.35,
			},
		}
		json.NewEncoder(w).Encode(erapiResponse)
		return
	} else {
		// Default response - always return USD response
		response, found = m.responses["/USD"]
	}

	// If still not found, create a default response
	if !found {
		response = ExchangeRateResponse{
			Base:      "USD",
			Timestamp: time.Now().Unix(),
			Rates: map[string]float64{
				"EUR": 0.85,
				"GBP": 0.73,
				"JPY": 110.0,
				"CAD": 1.25,
				"AUD": 1.35,
			},
			Provider: "MOCK",
		}
		found = true
	}

	// Set content type
	w.Header().Set("Content-Type", "application/json")

	// Return appropriate response format based on the request path using type switch
	switch path {
	case "/USD":
		// ERAPI format
		apiResponse := struct {
			BaseCode           string             `json:"base_code"`
			TimeLastUpdateUnix int64              `json:"time_last_update_unix"`
			Rates              map[string]float64 `json:"rates"`
		}{
			BaseCode:           response.Base,
			TimeLastUpdateUnix: response.Timestamp,
			Rates:              response.Rates,
		}
		json.NewEncoder(w).Encode(apiResponse)
	case "/latest":
		// Open Exchange Rates format
		apiResponse := struct {
			Base      string             `json:"base"`
			Timestamp int64              `json:"timestamp"`
			Rates     map[string]float64 `json:"rates"`
		}{
			Base:      response.Base,
			Timestamp: response.Timestamp,
			Rates:     response.Rates,
		}
		json.NewEncoder(w).Encode(apiResponse)
	default:
		// Default format (generic)
		apiResponse := struct {
			Base      string             `json:"base"`
			Timestamp int64              `json:"timestamp"`
			Rates     map[string]float64 `json:"rates"`
		}{
			Base:      response.Base,
			Timestamp: response.Timestamp,
			Rates:     response.Rates,
		}
		json.NewEncoder(w).Encode(apiResponse)
	}
}

// URL returns the mock server URL
func (m *MockExchangeRateServer) URL() string {
	return m.server.URL
}

// Close closes the mock server
func (m *MockExchangeRateServer) Close() {
	m.server.Close()
}

// SetResponse sets a custom response for a specific path
func (m *MockExchangeRateServer) SetResponse(path string, response ExchangeRateResponse) {
	m.responses[path] = response
}

// MockJSONPlaceholderServer creates a mock server for JSONPlaceholder API
type MockJSONPlaceholderServer struct {
	server *httptest.Server
}

// NewMockJSONPlaceholderServer creates a new mock JSONPlaceholder server
func NewMockJSONPlaceholderServer() *MockJSONPlaceholderServer {
	mock := &MockJSONPlaceholderServer{}
	mock.server = httptest.NewServer(http.HandlerFunc(mock.handler))
	return mock
}

// handler handles HTTP requests to the mock JSONPlaceholder server
func (m *MockJSONPlaceholderServer) handler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := r.URL.Path

	switch path {
	case "/posts":
		posts := []map[string]interface{}{
			{"id": 1, "title": "Test Post 1", "body": "Test body 1", "userId": 1},
			{"id": 2, "title": "Test Post 2", "body": "Test body 2", "userId": 2},
		}
		json.NewEncoder(w).Encode(posts)
	case "/posts/1":
		post := map[string]interface{}{
			"id": 1, "title": "Test Post 1", "body": "Test body 1", "userId": 1,
		}
		json.NewEncoder(w).Encode(post)
	case "/users":
		users := []map[string]interface{}{
			{"id": 1, "name": "Test User 1", "email": "user1@test.com"},
			{"id": 2, "name": "Test User 2", "email": "user2@test.com"},
		}
		json.NewEncoder(w).Encode(users)
	case "/comments":
		comments := []map[string]interface{}{
			{"id": 1, "postId": 1, "name": "Test Comment 1", "email": "comment1@test.com", "body": "Test comment body 1"},
			{"id": 2, "postId": 2, "name": "Test Comment 2", "email": "comment2@test.com", "body": "Test comment body 2"},
		}
		json.NewEncoder(w).Encode(comments)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

// URL returns the mock server URL
func (m *MockJSONPlaceholderServer) URL() string {
	return m.server.URL
}

// Close closes the mock server
func (m *MockJSONPlaceholderServer) Close() {
	m.server.Close()
}

// MockConfigWithMocks returns a test configuration with mock server URLs
func MockConfigWithMocks(exchangeRateServerURL, jsonPlaceholderServerURL string) *config.Config {
	return &config.Config{
		Port:                  "0", // Use random port
		LogLevel:              "error",
		RatesCacheTTL:         60 * time.Second,
		MaxConcurrentRequests: 4,
		RateLimitEnabled:      true,
		RateLimitRequests:     100,
		RateLimitWindow:       60 * time.Second,
		RateLimitBurst:        20,
		ExchangeRateProviders: []config.ExchangeRateProvider{
			{
				Name:       "erapi",
				BaseURL:    exchangeRateServerURL,
				APIKey:     "",
				Enabled:    true,
				Priority:   1,
				Timeout:    5 * time.Second,
				RetryCount: 3,
				RetryDelay: 1 * time.Second,
			},
			{
				Name:       "openexchangerates",
				BaseURL:    exchangeRateServerURL + "/latest",
				APIKey:     "test-key",
				Enabled:    true,
				Priority:   2,
				Timeout:    5 * time.Second,
				RetryCount: 3,
				RetryDelay: 1 * time.Second,
			},
		},
	}
}
