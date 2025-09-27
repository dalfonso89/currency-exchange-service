package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"currency-exchange-api/internal/config"
	"currency-exchange-api/internal/logger"
	"currency-exchange-api/internal/models"

	"golang.org/x/sync/singleflight"
)

type RatesService struct {
	configuration *config.Config
	logger        *logger.Logger
	providers     []ExchangeRateProvider

	cacheMutex sync.RWMutex
	cache      models.CacheEntry

	singleFlightGroup singleflight.Group
}

func NewRatesService(configuration *config.Config, logger *logger.Logger) *RatesService {
	// Create provider factory and get all enabled providers
	providerFactory := NewProviderFactory(configuration, logger)
	providers := providerFactory.CreateProviders()

	return &RatesService{
		configuration: configuration,
		logger:        logger,
		providers:     providers,
	}
}

// GetRates concurrently queries providers, returns first successful response and caches it.
func (ratesService *RatesService) GetRates(requestContext context.Context, baseCurrency string) (models.RatesResponse, error) {
	// serve from cache when valid and base unchanged
	ratesService.cacheMutex.RLock()
	if ratesService.cache.Data.Base == baseCurrency && time.Now().Before(ratesService.cache.ExpiresAt) {
		cachedResponse := ratesService.cache.Data
		ratesService.cacheMutex.RUnlock()
		return cachedResponse, nil
	}
	ratesService.cacheMutex.RUnlock()

	cacheKey := "rates:" + baseCurrency
	result, error, _ := ratesService.singleFlightGroup.Do(cacheKey, func() (interface{}, error) {
		return ratesService.fetchRatesFromProviders(requestContext, baseCurrency)
	})

	if error != nil {
		return models.RatesResponse{}, error
	}
	return result.(models.RatesResponse), nil
}

// fetchRatesFromProviders fetches rates from all enabled providers concurrently
func (ratesService *RatesService) fetchRatesFromProviders(requestContext context.Context, baseCurrency string) (models.RatesResponse, error) {
	if len(ratesService.providers) == 0 {
		return models.RatesResponse{}, fmt.Errorf("no exchange rate providers configured")
	}

	type providerResult struct {
		data models.RatesResponse
		err  error
	}

	// Create channels for results
	resultsChannel := make(chan providerResult, len(ratesService.providers))

	// Limit concurrent requests
	maxConcurrent := ratesService.configuration.MaxConcurrentRequests
	if maxConcurrent <= 0 {
		maxConcurrent = len(ratesService.providers)
	}

	semaphore := make(chan struct{}, maxConcurrent)

	// Launch goroutines for each provider
	for _, provider := range ratesService.providers {
		go func(p ExchangeRateProvider) {
			semaphore <- struct{}{}        // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			ratesService.logger.Debugf("Fetching rates from provider: %s", p.GetName())
			data, err := p.GetRates(requestContext, baseCurrency)
			resultsChannel <- providerResult{data, err}
		}(provider)
	}

	// Collect results
	var firstError error
	successCount := 0

	for i := 0; i < len(ratesService.providers); i++ {
		select {
		case <-requestContext.Done():
			if firstError == nil {
				firstError = requestContext.Err()
			}
			break
		case result := <-resultsChannel:
			if result.err == nil {
				// Cache the successful result
				ratesService.cacheMutex.Lock()
				ratesService.cache = models.CacheEntry{
					Data:      result.data,
					ExpiresAt: time.Now().Add(ratesService.configuration.RatesCacheTTL),
				}
				ratesService.cacheMutex.Unlock()

				ratesService.logger.Infof("Successfully fetched rates from provider: %s", result.data.Provider)
				return result.data, nil
			}

			successCount++
			if firstError == nil {
				firstError = result.err
			} else {
				ratesService.logger.Warnf("Provider %s failed: %v", ratesService.providers[i].GetName(), result.err)
			}
		}
	}

	// If we get here, all providers failed
	ratesService.logger.Errorf("All %d exchange rate providers failed", len(ratesService.providers))
	return models.RatesResponse{}, firstError
}

func (ratesService *RatesService) Convert(requestContext context.Context, fromCurrency, toCurrency string, amount float64) (models.ConvertResponse, error) {
	baseCurrency := strings.ToUpper(fromCurrency)
	exchangeRates, fetchError := ratesService.GetRates(requestContext, baseCurrency)
	if fetchError != nil {
		return models.ConvertResponse{}, fetchError
	}
	exchangeRate, rateExists := exchangeRates.Rates[strings.ToUpper(toCurrency)]
	if !rateExists {
		return models.ConvertResponse{}, fmt.Errorf("rate not found for %s", toCurrency)
	}
	convertedAmount := amount * exchangeRate
	return models.ConvertResponse{
		From:      fromCurrency,
		To:        toCurrency,
		Amount:    amount,
		Rate:      exchangeRate,
		Converted: convertedAmount,
		Provider:  exchangeRates.Provider,
	}, nil
}

// GetProviderStatus returns the status of all configured providers
func (ratesService *RatesService) GetProviderStatus() []ProviderStatus {
	statuses := make([]ProviderStatus, len(ratesService.providers))
	for i, provider := range ratesService.providers {
		statuses[i] = ProviderStatus{
			Name:     provider.GetName(),
			Enabled:  provider.IsEnabled(),
			Priority: provider.GetPriority(),
		}
	}
	return statuses
}

// ProviderStatus represents the status of a provider
type ProviderStatus struct {
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	Priority int    `json:"priority"`
}
