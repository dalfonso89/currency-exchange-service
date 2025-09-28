package service

import (
	"context"
	"currency-exchange-api/config"
	"currency-exchange-api/logger"
	"currency-exchange-api/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPExchangeRateProvider implements ExchangeRateProvider for HTTP-based APIs
type HTTPExchangeRateProvider struct {
	configuration config.ExchangeRateProvider
	logger        logger.Logger
	httpClient    *http.Client
}

// NewHTTPExchangeRateProvider creates a new HTTP exchange rate provider
func NewHTTPExchangeRateProvider(configuration config.ExchangeRateProvider, logger logger.Logger) *HTTPExchangeRateProvider {
	return &HTTPExchangeRateProvider{
		configuration: configuration,
		logger:        logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetName returns the provider name
func (provider *HTTPExchangeRateProvider) GetName() string {
	return provider.configuration.Name
}

// IsEnabled returns whether the provider is enabled
func (provider *HTTPExchangeRateProvider) IsEnabled() bool {
	return provider.configuration.Enabled
}

// GetPriority returns the provider priority
func (provider *HTTPExchangeRateProvider) GetPriority() int {
	return provider.configuration.Priority
}

// GetRates fetches exchange rates from the provider
func (provider *HTTPExchangeRateProvider) GetRates(ctx context.Context, baseCurrency string) (models.RatesResponse, error) {
	url := provider.buildURL(baseCurrency)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return models.RatesResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := provider.httpClient.Do(req)
	if err != nil {
		return models.RatesResponse{}, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.RatesResponse{}, fmt.Errorf("provider returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.RatesResponse{}, fmt.Errorf("failed to read response body: %w", err)
	}

	return provider.parseResponse(body, baseCurrency)
}

// buildURL constructs the URL for the provider based on its configuration
func (provider *HTTPExchangeRateProvider) buildURL(baseCurrency string) string {
	baseURL := provider.configuration.BaseURL

	// Handle different provider URL patterns
	switch provider.configuration.Name {
	case "erapi":
		// ExchangeRate-API format: https://api.exchangerate-api.com/v4/latest/USD
		return fmt.Sprintf("%s/%s", baseURL, baseCurrency)
	case "openexchangerates":
		// OpenExchangeRates format: https://openexchangerates.org/api/latest.json?base=USD
		return fmt.Sprintf("%s?base=%s", baseURL, baseCurrency)
	case "frankfurter":
		// Frankfurter format: https://api.frankfurter.app/latest?from=USD
		return fmt.Sprintf("%s?from=%s", baseURL, baseCurrency)
	case "exchangerate.host":
		// ExchangeRate.host format: https://api.exchangerate.host/latest?base=USD
		return fmt.Sprintf("%s?base=%s", baseURL, baseCurrency)
	default:
		// Generic format: append base currency as query parameter
		return fmt.Sprintf("%s?base=%s", baseURL, baseCurrency)
	}
}

// parseResponse parses the JSON response from the provider
func (provider *HTTPExchangeRateProvider) parseResponse(body []byte, baseCurrency string) (models.RatesResponse, error) {
	var response models.RatesResponse

	// Try to parse as generic response first
	if err := json.Unmarshal(body, &response); err == nil && response.Base != "" {
		response.Provider = provider.configuration.Name
		return response, nil
	}

	// Provider-specific parsing
	switch provider.configuration.Name {
	case "erapi":
		return provider.parseERAPIResponse(body, baseCurrency)
	case "openexchangerates":
		return provider.parseOpenExchangeRatesResponse(body, baseCurrency)
	case "frankfurter":
		return provider.parseFrankfurterResponse(body, baseCurrency)
	case "exchangerate.host":
		return provider.parseExchangeRateHostResponse(body, baseCurrency)
	default:
		return provider.parseGenericResponse(body, baseCurrency)
	}
}

// parseERAPIResponse parses ExchangeRate-API response format
func (provider *HTTPExchangeRateProvider) parseERAPIResponse(body []byte, baseCurrency string) (models.RatesResponse, error) {
	var data struct {
		Base      string             `json:"base"`
		Timestamp int64              `json:"timestamp"`
		Rates     map[string]float64 `json:"rates"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return models.RatesResponse{}, fmt.Errorf("failed to parse ERAPI response: %w", err)
	}

	return models.RatesResponse{
		Base:      data.Base,
		Timestamp: data.Timestamp,
		Rates:     data.Rates,
		Provider:  provider.configuration.Name,
	}, nil
}

// parseOpenExchangeRatesResponse parses OpenExchangeRates response format
func (provider *HTTPExchangeRateProvider) parseOpenExchangeRatesResponse(body []byte, baseCurrency string) (models.RatesResponse, error) {
	var data struct {
		Base      string             `json:"base"`
		Timestamp int64              `json:"timestamp"`
		Rates     map[string]float64 `json:"rates"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return models.RatesResponse{}, fmt.Errorf("failed to parse OpenExchangeRates response: %w", err)
	}

	return models.RatesResponse{
		Base:      data.Base,
		Timestamp: data.Timestamp,
		Rates:     data.Rates,
		Provider:  provider.configuration.Name,
	}, nil
}

// parseFrankfurterResponse parses Frankfurter response format
func (provider *HTTPExchangeRateProvider) parseFrankfurterResponse(body []byte, baseCurrency string) (models.RatesResponse, error) {
	var data struct {
		Base      string             `json:"base"`
		Timestamp int64              `json:"timestamp"`
		Rates     map[string]float64 `json:"rates"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return models.RatesResponse{}, fmt.Errorf("failed to parse Frankfurter response: %w", err)
	}

	return models.RatesResponse{
		Base:      data.Base,
		Timestamp: data.Timestamp,
		Rates:     data.Rates,
		Provider:  provider.configuration.Name,
	}, nil
}

// parseExchangeRateHostResponse parses ExchangeRate.host response format
func (provider *HTTPExchangeRateProvider) parseExchangeRateHostResponse(body []byte, baseCurrency string) (models.RatesResponse, error) {
	var data struct {
		Base      string             `json:"base"`
		Timestamp int64              `json:"timestamp"`
		Rates     map[string]float64 `json:"rates"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return models.RatesResponse{}, fmt.Errorf("failed to parse ExchangeRate.host response: %w", err)
	}

	return models.RatesResponse{
		Base:      data.Base,
		Timestamp: data.Timestamp,
		Rates:     data.Rates,
		Provider:  provider.configuration.Name,
	}, nil
}

// parseGenericResponse attempts to parse a generic response format
func (provider *HTTPExchangeRateProvider) parseGenericResponse(body []byte, baseCurrency string) (models.RatesResponse, error) {
	var data struct {
		Base      string             `json:"base"`
		Timestamp int64              `json:"timestamp"`
		Rates     map[string]float64 `json:"rates"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return models.RatesResponse{}, fmt.Errorf("failed to parse generic response: %w", err)
	}

	return models.RatesResponse{
		Base:      data.Base,
		Timestamp: data.Timestamp,
		Rates:     data.Rates,
		Provider:  provider.configuration.Name,
	}, nil
}
