package service

import (
	"context"
	"currency-exchange-api/config"
	"currency-exchange-api/models"

	"github.com/sirupsen/logrus"
)

// ExchangeRateProvider defines the interface for exchange rate providers
type ExchangeRateProvider interface {
	GetName() string
	GetRates(ctx context.Context, baseCurrency string) (models.RatesResponse, error)
	IsEnabled() bool
	GetPriority() int
}

// ProviderFactory creates provider instances
type ProviderFactory struct {
	config *config.Config
	logger *logrus.Logger
}

// NewProviderFactory creates a new provider factory
func NewProviderFactory(config *config.Config, logger *logrus.Logger) *ProviderFactory {
	return &ProviderFactory{
		config: config,
		logger: logger,
	}
}

// CreateProviders creates all enabled providers
func (pf *ProviderFactory) CreateProviders() []ExchangeRateProvider {
	providers := make([]ExchangeRateProvider, 0, len(pf.config.ExchangeRateProviders))

	for _, providerConfig := range pf.config.ExchangeRateProviders {
		if !providerConfig.Enabled {
			continue
		}

		provider := NewHTTPExchangeRateProvider(providerConfig, pf.logger)
		providers = append(providers, provider)
	}

	return providers
}
