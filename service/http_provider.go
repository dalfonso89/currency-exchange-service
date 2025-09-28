package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"currency-exchange-api/config"
	"currency-exchange-api/models"

	"github.com/sirupsen/logrus"
)

// HTTPExchangeRateProvider implements ExchangeRateProvider for HTTP-based APIs
type HTTPExchangeRateProvider struct {
	config     config.ExchangeRateProvider
	logger     *logrus.Logger
	httpClient *http.Client
}

// NewHTTPExchangeRateProvider creates a new HTTP-based exchange rate provider
func NewHTTPExchangeRateProvider(config config.ExchangeRateProvider, logger *logrus.Logger) *HTTPExchangeRateProvider {
	httpTransport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
	}

	return &HTTPExchangeRateProvider{
		config: config,
		logger: logger,
		httpClient: &http.Client{
			Timeout:   config.Timeout,
			Transport: httpTransport,
		},
	}
}

// GetName returns the provider name
func (p *HTTPExchangeRateProvider) GetName() string {
	return p.config.Name
}

// IsEnabled returns whether the provider is enabled
func (p *HTTPExchangeRateProvider) IsEnabled() bool {
	return p.config.Enabled
}

// GetPriority returns the provider priority
func (p *HTTPExchangeRateProvider) GetPriority() int {
	return p.config.Priority
}

// GetRates fetches exchange rates from the provider
func (p *HTTPExchangeRateProvider) GetRates(ctx context.Context, baseCurrency string) (models.RatesResponse, error) {
	url := p.buildURL(baseCurrency)

	request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return models.RatesResponse{}, err
	}

	// Add API key if available
	if p.config.APIKey != "" {
		request.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}
	request.Header.Set("Accept", "application/json")

	response, err := p.httpClient.Do(request)
	if err != nil {
		return models.RatesResponse{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return models.RatesResponse{}, fmt.Errorf("%s returned %d: %s", p.config.Name, response.StatusCode, string(body))
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return models.RatesResponse{}, err
	}

	return p.parseResponse(body)
}

// buildURL constructs the API URL for the given base currency
func (p *HTTPExchangeRateProvider) buildURL(baseCurrency string) string {
	baseURL := strings.TrimRight(p.config.BaseURL, "/")
	upperBase := strings.ToUpper(baseCurrency)

	switch p.config.Name {
	case "erapi":
		return fmt.Sprintf("%s/%s", baseURL, upperBase)
	case "openexchangerates":
		return fmt.Sprintf("%s?app_id=%s&base=%s", baseURL, p.config.APIKey, upperBase)
	case "frankfurter":
		return fmt.Sprintf("%s?base=%s", baseURL, upperBase)
	case "exchangerate.host":
		return fmt.Sprintf("%s?base=%s", baseURL, upperBase)
	default:
		// For custom providers, try to append base currency as path parameter
		if strings.Contains(baseURL, "?") {
			return fmt.Sprintf("%s&base=%s", baseURL, upperBase)
		}
		return fmt.Sprintf("%s?base=%s", baseURL, upperBase)
	}
}

// parseResponse parses the API response based on provider type
func (p *HTTPExchangeRateProvider) parseResponse(body []byte) (models.RatesResponse, error) {
	switch p.config.Name {
	case "erapi":
		return p.parseERAPIResponse(body)
	case "openexchangerates":
		return p.parseOpenExchangeRatesResponse(body)
	case "frankfurter":
		return p.parseFrankfurterResponse(body)
	case "exchangerate.host":
		return p.parseExchangeRateHostResponse(body)
	default:
		return p.parseGenericResponse(body)
	}
}

// parseERAPIResponse parses Exchange Rate API response
func (p *HTTPExchangeRateProvider) parseERAPIResponse(body []byte) (models.RatesResponse, error) {
	var payload struct {
		BaseCode           string             `json:"base_code"`
		TimeLastUpdateUnix int64              `json:"time_last_update_unix"`
		Rates              map[string]float64 `json:"rates"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return models.RatesResponse{}, err
	}

	if payload.BaseCode == "" || len(payload.Rates) == 0 {
		return models.RatesResponse{}, fmt.Errorf("invalid response from %s", p.config.Name)
	}

	return models.RatesResponse{
		Base:      payload.BaseCode,
		Timestamp: payload.TimeLastUpdateUnix,
		Rates:     payload.Rates,
		Provider:  p.config.Name,
	}, nil
}

// parseOpenExchangeRatesResponse parses Open Exchange Rates response
func (p *HTTPExchangeRateProvider) parseOpenExchangeRatesResponse(body []byte) (models.RatesResponse, error) {
	var payload struct {
		Base      string             `json:"base"`
		Timestamp int64              `json:"timestamp"`
		Rates     map[string]float64 `json:"rates"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return models.RatesResponse{}, err
	}

	if payload.Base == "" || len(payload.Rates) == 0 {
		return models.RatesResponse{}, fmt.Errorf("invalid response from %s", p.config.Name)
	}

	return models.RatesResponse{
		Base:      payload.Base,
		Timestamp: payload.Timestamp,
		Rates:     payload.Rates,
		Provider:  p.config.Name,
	}, nil
}

// parseFrankfurterResponse parses Frankfurter API response
func (p *HTTPExchangeRateProvider) parseFrankfurterResponse(body []byte) (models.RatesResponse, error) {
	var payload struct {
		Base  string             `json:"base"`
		Date  string             `json:"date"`
		Rates map[string]float64 `json:"rates"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return models.RatesResponse{}, err
	}

	if payload.Base == "" || len(payload.Rates) == 0 {
		return models.RatesResponse{}, fmt.Errorf("invalid response from %s", p.config.Name)
	}

	// Frankfurter doesn't provide unix timestamp, use current time
	return models.RatesResponse{
		Base:      payload.Base,
		Timestamp: time.Now().Unix(),
		Rates:     payload.Rates,
		Provider:  p.config.Name,
	}, nil
}

// parseExchangeRateHostResponse parses Exchange Rate Host response
func (p *HTTPExchangeRateProvider) parseExchangeRateHostResponse(body []byte) (models.RatesResponse, error) {
	var payload struct {
		Base      string             `json:"base"`
		Timestamp int64              `json:"timestamp"`
		Rates     map[string]float64 `json:"rates"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return models.RatesResponse{}, err
	}

	if payload.Base == "" || len(payload.Rates) == 0 {
		return models.RatesResponse{}, fmt.Errorf("invalid response from %s", p.config.Name)
	}

	return models.RatesResponse{
		Base:      payload.Base,
		Timestamp: payload.Timestamp,
		Rates:     payload.Rates,
		Provider:  p.config.Name,
	}, nil
}

// parseGenericResponse attempts to parse a generic JSON response
func (p *HTTPExchangeRateProvider) parseGenericResponse(body []byte) (models.RatesResponse, error) {
	var payload struct {
		Base      string             `json:"base"`
		Timestamp int64              `json:"timestamp"`
		Rates     map[string]float64 `json:"rates"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return models.RatesResponse{}, err
	}

	if payload.Base == "" || len(payload.Rates) == 0 {
		return models.RatesResponse{}, fmt.Errorf("invalid response from %s", p.config.Name)
	}

	return models.RatesResponse{
		Base:      payload.Base,
		Timestamp: payload.Timestamp,
		Rates:     payload.Rates,
		Provider:  p.config.Name,
	}, nil
}
