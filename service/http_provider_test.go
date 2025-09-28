package service

import (
	"context"
	"currency-exchange-api/config"
	"currency-exchange-api/testutils"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPExchangeRateProvider_GetName(t *testing.T) {
	provider := NewHTTPExchangeRateProvider(
		config.ExchangeRateProvider{Name: "test-provider"},
		testutils.MockLogger(),
	)

	if provider.GetName() != "test-provider" {
		t.Errorf("GetName() = %v, want %v", provider.GetName(), "test-provider")
	}
}

func TestHTTPExchangeRateProvider_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		expected bool
	}{
		{"enabled provider", true, true},
		{"disabled provider", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewHTTPExchangeRateProvider(
				config.ExchangeRateProvider{Enabled: tt.enabled},
				testutils.MockLogger(),
			)

			if provider.IsEnabled() != tt.expected {
				t.Errorf("IsEnabled() = %v, want %v", provider.IsEnabled(), tt.expected)
			}
		})
	}
}

func TestHTTPExchangeRateProvider_GetPriority(t *testing.T) {
	provider := NewHTTPExchangeRateProvider(
		config.ExchangeRateProvider{Priority: 5},
		testutils.MockLogger(),
	)

	if provider.GetPriority() != 5 {
		t.Errorf("GetPriority() = %v, want %v", provider.GetPriority(), 5)
	}
}

func TestHTTPExchangeRateProvider_buildURL(t *testing.T) {
	tests := []struct {
		name         string
		providerName string
		baseURL      string
		apiKey       string
		baseCurrency string
		expected     string
	}{
		{
			name:         "erapi provider",
			providerName: "erapi",
			baseURL:      "https://api.erapi.com/v6/latest",
			apiKey:       "",
			baseCurrency: "USD",
			expected:     "https://api.erapi.com/v6/latest/USD",
		},
		{
			name:         "openexchangerates provider",
			providerName: "openexchangerates",
			baseURL:      "https://openexchangerates.org/api/latest.json",
			apiKey:       "test-key",
			baseCurrency: "USD",
			expected:     "https://openexchangerates.org/api/latest.json?app_id=test-key&base=USD",
		},
		{
			name:         "frankfurter provider",
			providerName: "frankfurter",
			baseURL:      "https://api.frankfurter.app/latest",
			apiKey:       "",
			baseCurrency: "USD",
			expected:     "https://api.frankfurter.app/latest?base=USD",
		},
		{
			name:         "exchangerate.host provider",
			providerName: "exchangerate.host",
			baseURL:      "https://api.exchangerate.host/latest",
			apiKey:       "",
			baseCurrency: "USD",
			expected:     "https://api.exchangerate.host/latest?base=USD",
		},
		{
			name:         "custom provider",
			providerName: "custom",
			baseURL:      "https://api.custom.com/latest",
			apiKey:       "",
			baseCurrency: "USD",
			expected:     "https://api.custom.com/latest?base=USD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewHTTPExchangeRateProvider(
				config.ExchangeRateProvider{
					Name:    tt.providerName,
					BaseURL: tt.baseURL,
					APIKey:  tt.apiKey,
				},
				testutils.MockLogger(),
			)

			result := provider.buildURL(tt.baseCurrency)
			if result != tt.expected {
				t.Errorf("buildURL() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHTTPExchangeRateProvider_parseERAPIResponse(t *testing.T) {
	provider := NewHTTPExchangeRateProvider(
		config.ExchangeRateProvider{Name: "erapi"},
		testutils.MockLogger(),
	)

	jsonResponse := `{
		"base_code": "USD",
		"time_last_update_unix": 1640995200,
		"rates": {
			"EUR": 0.85,
			"GBP": 0.73,
			"JPY": 110.0
		}
	}`

	result, err := provider.parseERAPIResponse([]byte(jsonResponse))
	if err != nil {
		t.Fatalf("parseERAPIResponse() error = %v", err)
	}

	if result.Base != "USD" {
		t.Errorf("parseERAPIResponse() Base = %v, want %v", result.Base, "USD")
	}
	if result.Timestamp != 1640995200 {
		t.Errorf("parseERAPIResponse() Timestamp = %v, want %v", result.Timestamp, 1640995200)
	}
	if len(result.Rates) != 3 {
		t.Errorf("parseERAPIResponse() Rates length = %v, want %v", len(result.Rates), 3)
	}
	if result.Provider != "erapi" {
		t.Errorf("parseERAPIResponse() Provider = %v, want %v", result.Provider, "erapi")
	}
}

func TestHTTPExchangeRateProvider_parseOpenExchangeRatesResponse(t *testing.T) {
	provider := NewHTTPExchangeRateProvider(
		config.ExchangeRateProvider{Name: "openexchangerates"},
		testutils.MockLogger(),
	)

	jsonResponse := `{
		"base": "USD",
		"timestamp": 1640995200,
		"rates": {
			"EUR": 0.85,
			"GBP": 0.73,
			"JPY": 110.0
		}
	}`

	result, err := provider.parseOpenExchangeRatesResponse([]byte(jsonResponse))
	if err != nil {
		t.Fatalf("parseOpenExchangeRatesResponse() error = %v", err)
	}

	if result.Base != "USD" {
		t.Errorf("parseOpenExchangeRatesResponse() Base = %v, want %v", result.Base, "USD")
	}
	if result.Timestamp != 1640995200 {
		t.Errorf("parseOpenExchangeRatesResponse() Timestamp = %v, want %v", result.Timestamp, 1640995200)
	}
	if len(result.Rates) != 3 {
		t.Errorf("parseOpenExchangeRatesResponse() Rates length = %v, want %v", len(result.Rates), 3)
	}
	if result.Provider != "openexchangerates" {
		t.Errorf("parseOpenExchangeRatesResponse() Provider = %v, want %v", result.Provider, "openexchangerates")
	}
}

func TestHTTPExchangeRateProvider_parseFrankfurterResponse(t *testing.T) {
	provider := NewHTTPExchangeRateProvider(
		config.ExchangeRateProvider{Name: "frankfurter"},
		testutils.MockLogger(),
	)

	jsonResponse := `{
		"base": "USD",
		"date": "2022-01-01",
		"rates": {
			"EUR": 0.85,
			"GBP": 0.73,
			"JPY": 110.0
		}
	}`

	result, err := provider.parseFrankfurterResponse([]byte(jsonResponse))
	if err != nil {
		t.Fatalf("parseFrankfurterResponse() error = %v", err)
	}

	if result.Base != "USD" {
		t.Errorf("parseFrankfurterResponse() Base = %v, want %v", result.Base, "USD")
	}
	if result.Timestamp == 0 {
		t.Errorf("parseFrankfurterResponse() Timestamp should not be zero")
	}
	if len(result.Rates) != 3 {
		t.Errorf("parseFrankfurterResponse() Rates length = %v, want %v", len(result.Rates), 3)
	}
	if result.Provider != "frankfurter" {
		t.Errorf("parseFrankfurterResponse() Provider = %v, want %v", result.Provider, "frankfurter")
	}
}

func TestHTTPExchangeRateProvider_parseExchangeRateHostResponse(t *testing.T) {
	provider := NewHTTPExchangeRateProvider(
		config.ExchangeRateProvider{Name: "exchangerate.host"},
		testutils.MockLogger(),
	)

	jsonResponse := `{
		"base": "USD",
		"timestamp": 1640995200,
		"rates": {
			"EUR": 0.85,
			"GBP": 0.73,
			"JPY": 110.0
		}
	}`

	result, err := provider.parseExchangeRateHostResponse([]byte(jsonResponse))
	if err != nil {
		t.Fatalf("parseExchangeRateHostResponse() error = %v", err)
	}

	if result.Base != "USD" {
		t.Errorf("parseExchangeRateHostResponse() Base = %v, want %v", result.Base, "USD")
	}
	if result.Timestamp != 1640995200 {
		t.Errorf("parseExchangeRateHostResponse() Timestamp = %v, want %v", result.Timestamp, 1640995200)
	}
	if len(result.Rates) != 3 {
		t.Errorf("parseExchangeRateHostResponse() Rates length = %v, want %v", len(result.Rates), 3)
	}
	if result.Provider != "exchangerate.host" {
		t.Errorf("parseExchangeRateHostResponse() Provider = %v, want %v", result.Provider, "exchangerate.host")
	}
}

func TestHTTPExchangeRateProvider_parseGenericResponse(t *testing.T) {
	provider := NewHTTPExchangeRateProvider(
		config.ExchangeRateProvider{Name: "custom"},
		testutils.MockLogger(),
	)

	jsonResponse := `{
		"base": "USD",
		"timestamp": 1640995200,
		"rates": {
			"EUR": 0.85,
			"GBP": 0.73,
			"JPY": 110.0
		}
	}`

	result, err := provider.parseGenericResponse([]byte(jsonResponse))
	if err != nil {
		t.Fatalf("parseGenericResponse() error = %v", err)
	}

	if result.Base != "USD" {
		t.Errorf("parseGenericResponse() Base = %v, want %v", result.Base, "USD")
	}
	if result.Timestamp != 1640995200 {
		t.Errorf("parseGenericResponse() Timestamp = %v, want %v", result.Timestamp, 1640995200)
	}
	if len(result.Rates) != 3 {
		t.Errorf("parseGenericResponse() Rates length = %v, want %v", len(result.Rates), 3)
	}
	if result.Provider != "custom" {
		t.Errorf("parseGenericResponse() Provider = %v, want %v", result.Provider, "custom")
	}
}

func TestHTTPExchangeRateProvider_GetRates(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"base": "USD",
			"timestamp": 1640995200,
			"rates": {
				"EUR": 0.85,
				"GBP": 0.73,
				"JPY": 110.0
			}
		}`))
	}))
	defer server.Close()

	provider := NewHTTPExchangeRateProvider(
		config.ExchangeRateProvider{
			Name:    "test-provider",
			BaseURL: server.URL,
			APIKey:  "",
			Enabled: true,
			Timeout: 30 * time.Second,
		},
		testutils.MockLogger(),
	)

	ctx := context.Background()
	result, err := provider.GetRates(ctx, "USD")
	if err != nil {
		t.Fatalf("GetRates() error = %v", err)
	}

	if result.Base != "USD" {
		t.Errorf("GetRates() Base = %v, want %v", result.Base, "USD")
	}
	if result.Timestamp != 1640995200 {
		t.Errorf("GetRates() Timestamp = %v, want %v", result.Timestamp, 1640995200)
	}
	if len(result.Rates) != 3 {
		t.Errorf("GetRates() Rates length = %v, want %v", len(result.Rates), 3)
	}
	if result.Provider != "test-provider" {
		t.Errorf("GetRates() Provider = %v, want %v", result.Provider, "test-provider")
	}
}

func TestHTTPExchangeRateProvider_GetRates_Error(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	provider := NewHTTPExchangeRateProvider(
		config.ExchangeRateProvider{
			Name:    "test-provider",
			BaseURL: server.URL,
			APIKey:  "",
			Enabled: true,
			Timeout: 30 * time.Second,
		},
		testutils.MockLogger(),
	)

	ctx := context.Background()
	_, err := provider.GetRates(ctx, "USD")
	if err == nil {
		t.Errorf("GetRates() expected error, got nil")
	}
}

func TestHTTPExchangeRateProvider_GetRates_InvalidJSON(t *testing.T) {
	// Create a test server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	provider := NewHTTPExchangeRateProvider(
		config.ExchangeRateProvider{
			Name:    "test-provider",
			BaseURL: server.URL,
			APIKey:  "",
			Enabled: true,
			Timeout: 30 * time.Second,
		},
		testutils.MockLogger(),
	)

	ctx := context.Background()
	_, err := provider.GetRates(ctx, "USD")
	if err == nil {
		t.Errorf("GetRates() expected error, got nil")
	}
}
