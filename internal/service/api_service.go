package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"currency-exchange-api/internal/config"

	"github.com/sirupsen/logrus"
)

// APIService handles external API calls
type APIService struct {
	configuration *config.Config
	logger        *logrus.Logger
	httpClient    *http.Client
}

// NewAPIService creates a new API service
func NewAPIService(configuration *config.Config, logger *logrus.Logger) *APIService {
	httpTransport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
	}
	return &APIService{
		configuration: configuration,
		logger:        logger,
		httpClient:    &http.Client{Timeout: configuration.Timeout, Transport: httpTransport},
	}
}

// HealthCheck performs a health check on the external API
func (apiService *APIService) HealthCheck(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, "GET", apiService.configuration.APIBaseURL+"/posts/1", nil)
	if err != nil {
		return err
	}

	response, err := apiService.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("API health check failed with status: %d", response.StatusCode)
	}

	return nil
}

// FetchPosts fetches all posts from the external API
func (apiService *APIService) FetchPosts(ctx context.Context) ([]map[string]interface{}, error) {
	return apiService.fetchJSONArray(ctx, "/posts")
}

// FetchPostByID fetches a specific post by ID from the external API
func (apiService *APIService) FetchPostByID(ctx context.Context, id int) (map[string]interface{}, error) {
	return apiService.fetchJSONObject(ctx, fmt.Sprintf("/posts/%d", id))
}

// FetchUsers fetches all users from the external API
func (apiService *APIService) FetchUsers(ctx context.Context) ([]map[string]interface{}, error) {
	return apiService.fetchJSONArray(ctx, "/users")
}

// FetchComments fetches all comments from the external API
func (apiService *APIService) FetchComments(ctx context.Context) ([]map[string]interface{}, error) {
	return apiService.fetchJSONArray(ctx, "/comments")
}

// fetchJSONArray fetches a JSON array from the external API
func (apiService *APIService) fetchJSONArray(ctx context.Context, endpoint string) ([]map[string]interface{}, error) {
	url := apiService.configuration.APIBaseURL + endpoint
	request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	response, err := apiService.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", response.StatusCode, string(body))
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// fetchJSONObject fetches a JSON object from the external API
func (apiService *APIService) fetchJSONObject(ctx context.Context, endpoint string) (map[string]interface{}, error) {
	url := apiService.configuration.APIBaseURL + endpoint
	request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	response, err := apiService.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", response.StatusCode, string(body))
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}
