package service

import (
	"context"
	"testing"
	"time"

	"github.com/dalfonso89/currency-exchange-service/models"
	"github.com/dalfonso89/currency-exchange-service/testutils"
)

// MockProvider is a mock implementation of ExchangeRateProvider for testing
type MockProvider struct {
	name     string
	enabled  bool
	priority int
	rates    map[string]float64
	error    error
}

func (m *MockProvider) GetName() string {
	return m.name
}

func (m *MockProvider) IsEnabled() bool {
	return m.enabled
}

func (m *MockProvider) GetPriority() int {
	return m.priority
}

func (m *MockProvider) GetRates(ctx context.Context, baseCurrency string) (models.RatesResponse, error) {
	if m.error != nil {
		return models.RatesResponse{}, m.error
	}
	return models.RatesResponse{
		Base:      baseCurrency,
		Timestamp: time.Now().Unix(),
		Rates:     m.rates,
		Provider:  m.name,
	}, nil
}

func TestNewRatesService(t *testing.T) {
	cfg := testutils.MockConfig()
	logger := testutils.MockLogger()

	service := NewRatesService(cfg, logger)

	if service == nil {
		t.Fatal("NewRatesService() returned nil")
	}
	if service.configuration != cfg {
		t.Errorf("NewRatesService() configuration = %v, want %v", service.configuration, cfg)
	}
	if service.logger != logger {
		t.Errorf("NewRatesService() logger = %v, want %v", service.logger, logger)
	}
	if len(service.providers) == 0 {
		t.Errorf("NewRatesService() providers length = %v, want > 0", len(service.providers))
	}
}

func TestRatesService_GetRates_Success(t *testing.T) {
	cfg := testutils.MockConfig()
	logger := testutils.MockLogger()

	// Create a mock provider
	mockProvider := &MockProvider{
		name:     "test-provider",
		enabled:  true,
		priority: 1,
		rates: map[string]float64{
			"EUR": 0.85,
			"GBP": 0.73,
			"JPY": 110.0,
		},
		error: nil,
	}

	service := &RatesService{
		configuration: cfg,
		logger:        logger,
		providers:     []ExchangeRateProvider{mockProvider},
	}

	ctx := context.Background()
	result, err := service.GetRates(ctx, "USD")
	if err != nil {
		t.Fatalf("GetRates() error = %v", err)
	}

	if result.Base != "USD" {
		t.Errorf("GetRates() Base = %v, want %v", result.Base, "USD")
	}
	if len(result.Rates) != 3 {
		t.Errorf("GetRates() Rates length = %v, want %v", len(result.Rates), 3)
	}
	if result.Provider != "test-provider" {
		t.Errorf("GetRates() Provider = %v, want %v", result.Provider, "test-provider")
	}
}

func TestRatesService_GetRates_NoProviders(t *testing.T) {
	cfg := testutils.MockConfig()
	logger := testutils.MockLogger()

	service := &RatesService{
		configuration: cfg,
		logger:        logger,
		providers:     []ExchangeRateProvider{},
	}

	ctx := context.Background()
	_, err := service.GetRates(ctx, "USD")
	if err == nil {
		t.Errorf("GetRates() expected error, got nil")
	}
}

func TestRatesService_GetRates_AllProvidersFail(t *testing.T) {
	cfg := testutils.MockConfig()
	logger := testutils.MockLogger()

	// Create mock providers that all fail
	mockProvider1 := &MockProvider{
		name:     "provider1",
		enabled:  true,
		priority: 1,
		rates:    nil,
		error:    context.DeadlineExceeded,
	}
	mockProvider2 := &MockProvider{
		name:     "provider2",
		enabled:  true,
		priority: 2,
		rates:    nil,
		error:    context.DeadlineExceeded,
	}

	service := &RatesService{
		configuration: cfg,
		logger:        logger,
		providers:     []ExchangeRateProvider{mockProvider1, mockProvider2},
	}

	ctx := context.Background()
	_, err := service.GetRates(ctx, "USD")
	if err == nil {
		t.Errorf("GetRates() expected error, got nil")
	}
}

func TestRatesService_GetRates_Cache(t *testing.T) {
	cfg := testutils.MockConfig()
	logger := testutils.MockLogger()

	// Create a mock provider
	mockProvider := &MockProvider{
		name:     "test-provider",
		enabled:  true,
		priority: 1,
		rates: map[string]float64{
			"EUR": 0.85,
		},
		error: nil,
	}

	service := &RatesService{
		configuration: cfg,
		logger:        logger,
		providers:     []ExchangeRateProvider{mockProvider},
	}

	ctx := context.Background()

	// First call should hit the provider
	result1, err := service.GetRates(ctx, "USD")
	if err != nil {
		t.Fatalf("GetRates() first call error = %v", err)
	}

	// Second call should hit the cache
	result2, err := service.GetRates(ctx, "USD")
	if err != nil {
		t.Fatalf("GetRates() second call error = %v", err)
	}

	// Results should be the same (cached)
	if result1.Base != result2.Base {
		t.Errorf("GetRates() cached result Base = %v, want %v", result2.Base, result1.Base)
	}
	if result1.Timestamp != result2.Timestamp {
		t.Errorf("GetRates() cached result Timestamp = %v, want %v", result2.Timestamp, result1.Timestamp)
	}
}

func TestRatesService_GetProviderStatus(t *testing.T) {
	cfg := testutils.MockConfig()
	logger := testutils.MockLogger()

	// Create mock providers
	mockProvider1 := &MockProvider{
		name:     "provider1",
		enabled:  true,
		priority: 1,
	}
	mockProvider2 := &MockProvider{
		name:     "provider2",
		enabled:  false,
		priority: 2,
	}

	service := &RatesService{
		configuration: cfg,
		logger:        logger,
		providers:     []ExchangeRateProvider{mockProvider1, mockProvider2},
	}

	statuses := service.GetProviderStatus()

	if len(statuses) != 2 {
		t.Errorf("GetProviderStatus() length = %v, want %v", len(statuses), 2)
	}

	// Check first provider
	if statuses[0].Name != "provider1" {
		t.Errorf("GetProviderStatus() first provider Name = %v, want %v", statuses[0].Name, "provider1")
	}
	if !statuses[0].Enabled {
		t.Errorf("GetProviderStatus() first provider Enabled = %v, want %v", statuses[0].Enabled, true)
	}
	if statuses[0].Priority != 1 {
		t.Errorf("GetProviderStatus() first provider Priority = %v, want %v", statuses[0].Priority, 1)
	}

	// Check second provider
	if statuses[1].Name != "provider2" {
		t.Errorf("GetProviderStatus() second provider Name = %v, want %v", statuses[1].Name, "provider2")
	}
	if statuses[1].Enabled {
		t.Errorf("GetProviderStatus() second provider Enabled = %v, want %v", statuses[1].Enabled, false)
	}
	if statuses[1].Priority != 2 {
		t.Errorf("GetProviderStatus() second provider Priority = %v, want %v", statuses[1].Priority, 2)
	}
}

func TestRatesService_ConcurrentRequests(t *testing.T) {
	cfg := testutils.MockConfig()
	logger := testutils.MockLogger()

	// Create a mock provider
	mockProvider := &MockProvider{
		name:     "test-provider",
		enabled:  true,
		priority: 1,
		rates: map[string]float64{
			"EUR": 0.85,
		},
		error: nil,
	}

	service := &RatesService{
		configuration: cfg,
		logger:        logger,
		providers:     []ExchangeRateProvider{mockProvider},
	}

	ctx := context.Background()

	// Make concurrent requests
	results := make(chan models.RatesResponse, 10)
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func() {
			result, err := service.GetRates(ctx, "USD")
			if err != nil {
				errors <- err
				return
			}
			results <- result
		}()
	}

	// Collect results
	successCount := 0
	errorCount := 0

	for i := 0; i < 10; i++ {
		select {
		case <-results:
			successCount++
		case <-errors:
			errorCount++
		}
	}

	if successCount == 0 {
		t.Errorf("Concurrent requests: no successful requests")
	}
	if errorCount > 0 {
		t.Errorf("Concurrent requests: %v errors occurred", errorCount)
	}
}
