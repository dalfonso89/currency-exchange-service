package service

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
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
    logger         *logger.Logger
    httpClient     *http.Client

    cacheMutex sync.RWMutex
    cache      models.CacheEntry

    singleFlightGroup singleflight.Group
}

func NewRatesService(configuration *config.Config, logger *logger.Logger) *RatesService {
    httpTransport := &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 100,
        IdleConnTimeout:     90 * time.Second,
        DisableCompression:  false,
    }
    return &RatesService{
        configuration: configuration,
        logger:         logger,
        httpClient:     &http.Client{Timeout: configuration.Timeout, Transport: httpTransport},
    }
}

// GetRates concurrently queries providers, returns first successful response and caches it.
func (ratesService *RatesService) GetRates(requestContext context.Context, baseCurrency string) (models.RatesResponse, error) {
    // serve from cache when valid and base unchanged
    ratesService.cacheMutex.RLock()
    if ratesService.cache.Data.Base == strings.ToUpper(baseCurrency) && time.Now().Before(ratesService.cache.ExpiresAt) {
        cachedResponse := ratesService.cache.Data
        ratesService.cacheMutex.RUnlock()
        return cachedResponse, nil
    }
    ratesService.cacheMutex.RUnlock()

    cacheKey := "rates:" + strings.ToUpper(baseCurrency)
    result, error, _ := ratesService.singleFlightGroup.Do(cacheKey, func() (interface{}, error) {
        type providerResult struct {
            data models.RatesResponse
            err  error
        }
        requestContext, cancel := context.WithTimeout(requestContext, ratesService.configuration.Timeout)
        defer cancel()
        resultsChannel := make(chan providerResult, 4)

        go func() {
            exchangeRateAPIURL := fmt.Sprintf("%s/%s", strings.TrimRight(ratesService.configuration.ExchangeRateAPIBaseURL, "/"), strings.ToUpper(baseCurrency))
            responseData, fetchError := ratesService.fetchJSON(requestContext, exchangeRateAPIURL, "erapi", "")
            resultsChannel <- providerResult{responseData, fetchError}
        }()
        go func() {
            openExchangeRatesURL := fmt.Sprintf("%s?app_id=%s&base=%s", strings.TrimRight(ratesService.configuration.OpenExchangeRatesBaseURL, "/"), ratesService.configuration.OpenExchangeRatesAPIKey, strings.ToUpper(baseCurrency))
            responseData, fetchError := ratesService.fetchJSON(requestContext, openExchangeRatesURL, "openexchangerates", "")
            resultsChannel <- providerResult{responseData, fetchError}
        }()
        go func() {
            frankfurterAPIURL := fmt.Sprintf("%s?base=%s", strings.TrimRight(ratesService.configuration.FrankfurterAPIBaseURL, "/"), strings.ToUpper(baseCurrency))
            responseData, fetchError := ratesService.fetchJSON(requestContext, frankfurterAPIURL, "frankfurter", "")
            resultsChannel <- providerResult{responseData, fetchError}
        }()
        go func() {
            exchangeRateHostURL := fmt.Sprintf("%s?base=%s", strings.TrimRight(ratesService.configuration.ExchangeRateHostBaseURL, "/"), strings.ToUpper(baseCurrency))
            responseData, fetchError := ratesService.fetchJSON(requestContext, exchangeRateHostURL, "exchangerate.host", "")
            resultsChannel <- providerResult{responseData, fetchError}
        }()

        var firstError error
        for providerIndex := 0; providerIndex < 4; providerIndex++ {
            select {
            case <-requestContext.Done():
                if firstError == nil {
                    firstError = requestContext.Err()
                }
            case providerResult := <-resultsChannel:
                if providerResult.err == nil {
                    ratesService.cacheMutex.Lock()
                    ratesService.cache = models.CacheEntry{Data: providerResult.data, ExpiresAt: time.Now().Add(ratesService.configuration.RatesCacheTTL)}
                    ratesService.cacheMutex.Unlock()
                    return providerResult.data, nil
                }
                if firstError == nil {
                    firstError = providerResult.err
                } else {
                    ratesService.logger.Warnf("All providers failed: %v", providerResult.err)
                }
            }
        }
        return models.RatesResponse{}, firstError
    })
    if error != nil {
        return models.RatesResponse{}, error
    }
    return result.(models.RatesResponse), nil
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
        From: fromCurrency,
        To: toCurrency,
        Amount: amount,
        Rate: exchangeRate,
        Converted: convertedAmount,
        Provider: exchangeRates.Provider,
    }, nil
}

func (ratesService *RatesService) fetchJSON(requestContext context.Context, requestURL, providerName, apiKey string) (models.RatesResponse, error) {
    httpRequest, requestError := http.NewRequestWithContext(requestContext, "GET", requestURL, nil)
    if requestError != nil {
        return models.RatesResponse{}, requestError
    }
    if apiKey != "" {
        httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
    }
    httpRequest.Header.Set("Accept", "application/json")
    httpResponse, responseError := ratesService.httpClient.Do(httpRequest)
    if responseError != nil {
        return models.RatesResponse{}, responseError
    }
    defer httpResponse.Body.Close()
    if httpResponse.StatusCode != http.StatusOK {
        responseBody, _ := io.ReadAll(httpResponse.Body)
        return models.RatesResponse{}, fmt.Errorf("%s returned %d: %s", providerName, httpResponse.StatusCode, string(responseBody))
    }
    responseBody, readError := io.ReadAll(httpResponse.Body)
    if readError != nil {
        return models.RatesResponse{}, readError
    }

    switch providerName {
    case "erapi":
        // { "base_code":"USD", "time_last_update_unix":123, "rates":{...} }
        var exchangeRateAPIPayload struct {
            BaseCode           string             `json:"base_code"`
            TimeLastUpdateUnix int64              `json:"time_last_update_unix"`
            Rates              map[string]float64 `json:"rates"`
        }
        if unmarshalError := json.Unmarshal(responseBody, &exchangeRateAPIPayload); unmarshalError != nil {
            return models.RatesResponse{}, unmarshalError
        }
        if exchangeRateAPIPayload.BaseCode == "" || len(exchangeRateAPIPayload.Rates) == 0 {
            return models.RatesResponse{}, fmt.Errorf("invalid response from %s", providerName)
        }
        return models.RatesResponse{Base: exchangeRateAPIPayload.BaseCode, Timestamp: exchangeRateAPIPayload.TimeLastUpdateUnix, Rates: exchangeRateAPIPayload.Rates, Provider: providerName}, nil

    case "openexchangerates":
        // { "base":"USD", "timestamp":123, "rates":{...} }
        var openExchangeRatesPayload struct {
            Base      string             `json:"base"`
            Timestamp int64              `json:"timestamp"`
            Rates     map[string]float64 `json:"rates"`
        }
        if unmarshalError := json.Unmarshal(responseBody, &openExchangeRatesPayload); unmarshalError != nil {
            return models.RatesResponse{}, unmarshalError
        }
        if openExchangeRatesPayload.Base == "" || len(openExchangeRatesPayload.Rates) == 0 {
            return models.RatesResponse{}, fmt.Errorf("invalid response from %s", providerName)
        }
        return models.RatesResponse{Base: openExchangeRatesPayload.Base, Timestamp: openExchangeRatesPayload.Timestamp, Rates: openExchangeRatesPayload.Rates, Provider: providerName}, nil

    case "frankfurter":
        // { "base":"USD", "date":"2020-01-01", "rates":{...} }
        var frankfurterPayload struct {
            Base  string             `json:"base"`
            Date  string             `json:"date"`
            Rates map[string]float64 `json:"rates"`
        }
        if unmarshalError := json.Unmarshal(responseBody, &frankfurterPayload); unmarshalError != nil {
            return models.RatesResponse{}, unmarshalError
        }
        if frankfurterPayload.Base == "" || len(frankfurterPayload.Rates) == 0 {
            return models.RatesResponse{}, fmt.Errorf("invalid response from %s", providerName)
        }
        // frankfurter has no unix timestamp in payload; use response Date when needed; fallback to now
        currentTimestamp := time.Now().Unix()
        return models.RatesResponse{Base: frankfurterPayload.Base, Timestamp: currentTimestamp, Rates: frankfurterPayload.Rates, Provider: providerName}, nil

    case "exchangerate.host":
        // { "base":"USD", "timestamp":123, "rates":{...} }
        var exchangeRateHostPayload struct {
            Base      string             `json:"base"`
            Timestamp int64              `json:"timestamp"`
            Rates     map[string]float64 `json:"rates"`
        }
        if unmarshalError := json.Unmarshal(responseBody, &exchangeRateHostPayload); unmarshalError != nil {
            return models.RatesResponse{}, unmarshalError
        }
        if exchangeRateHostPayload.Base == "" || len(exchangeRateHostPayload.Rates) == 0 {
            return models.RatesResponse{}, fmt.Errorf("invalid response from %s", providerName)
        }
        return models.RatesResponse{Base: exchangeRateHostPayload.Base, Timestamp: exchangeRateHostPayload.Timestamp, Rates: exchangeRateHostPayload.Rates, Provider: providerName}, nil
    }

    return models.RatesResponse{}, fmt.Errorf("unknown provider: %s", providerName)
}


