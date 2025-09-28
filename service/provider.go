package service

import (
	"context"
	"currency-exchange-api/config"
	"currency-exchange-api/logger"
	"currency-exchange-api/models"
)

// ExchangeRateProvider defines the interface for exchange rate providers
type ExchangeRateProvider interface {
	GetName() string
	IsEnabled() bool
	GetPriority() int
	GetRates(ctx context.Context, baseCurrency string) (models.RatesResponse, error)
}

// ProviderFactory creates exchange rate providers based on configuration
type ProviderFactory struct {
	configuration *config.Config
	logger        logger.Logger
}

// NewProviderFactory creates a new provider factory
func NewProviderFactory(configuration *config.Config, logger logger.Logger) *ProviderFactory {
	return &ProviderFactory{
		configuration: configuration,
		logger:        logger,
	}
}

// CreateProviders creates all configured providers
func (factory *ProviderFactory) CreateProviders() []ExchangeRateProvider {
	var providers []ExchangeRateProvider

	for _, providerConfig := range factory.configuration.ExchangeRateProviders {
		if providerConfig.Enabled {
			provider := NewHTTPExchangeRateProvider(providerConfig, factory.logger)
			providers = append(providers, provider)
		}
	}

	return providers
}
