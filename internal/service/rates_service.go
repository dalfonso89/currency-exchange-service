package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"currency-exchange-api/internal/config"
	"currency-exchange-api/internal/models"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"
)

// Custom error types for better error handling with type switches
type ErrorType int

const (
	ErrorTypeNoProviders ErrorType = iota
	ErrorTypeContextCancelled
	ErrorTypeProviderFailed
	ErrorTypeNetworkError
	ErrorTypeInvalidResponse
	ErrorTypeUnknown
)

// ServiceError represents a service-specific error with type information
type ServiceError struct {
	Type    ErrorType
	Message string
	Cause   error
}

// ProviderStatus represents the status of a provider
type ProviderStatus struct {
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	Priority int    `json:"priority"`
}

func (e ServiceError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

// classifyError classifies an error and returns appropriate error type
func classifyError(err error) ErrorType {
	if err == nil {
		return ErrorTypeUnknown
	}

	// Use type switch for error classification
	switch e := err.(type) {
	case *ServiceError:
		return e.Type
	default:
		// Check error message patterns
		errMsg := err.Error()
		switch {
		case strings.Contains(strings.ToLower(errMsg), "context canceled") || strings.Contains(strings.ToLower(errMsg), "context deadline exceeded"):
			return ErrorTypeContextCancelled
		case strings.Contains(strings.ToLower(errMsg), "network") || strings.Contains(strings.ToLower(errMsg), "connection") || strings.Contains(strings.ToLower(errMsg), "timeout"):
			return ErrorTypeNetworkError
		case strings.Contains(strings.ToLower(errMsg), "invalid response") || strings.Contains(strings.ToLower(errMsg), "parse"):
			return ErrorTypeInvalidResponse
		default:
			return ErrorTypeUnknown
		}
	}
}

type RatesService struct {
	configuration *config.Config
	logger        *logrus.Logger
	providers     []ExchangeRateProvider

	cacheMutex sync.RWMutex
	cache      models.CacheEntry

	singleFlightGroup singleflight.Group
}

func NewRatesService(configuration *config.Config, logger *logrus.Logger) *RatesService {
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
		return models.RatesResponse{}, &ServiceError{
			Type:    ErrorTypeNoProviders,
			Message: "no exchange rate providers configured",
		}
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

	// Use labeled loop for proper break control
collectLoop:
	for i := 0; i < len(ratesService.providers); i++ {
		select {
		case <-requestContext.Done():
			if firstError == nil {
				firstError = &ServiceError{
					Type:    ErrorTypeContextCancelled,
					Message: "request context cancelled",
					Cause:   requestContext.Err(),
				}
			}
			break collectLoop
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

			// Handle provider errors using type switches
			errorType := classifyError(result.err)
			switch errorType {
			case ErrorTypeContextCancelled:
				ratesService.logger.Warnf("Provider cancelled: %v", result.err)
			case ErrorTypeNetworkError:
				ratesService.logger.Warnf("Provider network error: %v", result.err)
			case ErrorTypeInvalidResponse:
				ratesService.logger.Warnf("Provider invalid response: %v", result.err)
			default:
				ratesService.logger.Warnf("Provider failed: %v", result.err)
			}

			if firstError == nil {
				firstError = &ServiceError{
					Type:    ErrorTypeProviderFailed,
					Message: "provider request failed",
					Cause:   result.err,
				}
			}
		}
	}

	// If we get here, all providers failed
	ratesService.logger.Errorf("All %d exchange rate providers failed", len(ratesService.providers))
	return models.RatesResponse{}, firstError
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
