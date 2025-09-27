package models

import (
	"testing"
	"time"
)

func TestRatesResponse(t *testing.T) {
	tests := []struct {
		name     string
		response RatesResponse
		expected func(RatesResponse) bool
	}{
		{
			name: "valid rates response",
			response: RatesResponse{
				Base:      "USD",
				Timestamp: 1640995200,
				Rates: map[string]float64{
					"EUR": 0.85,
					"GBP": 0.73,
					"JPY": 110.0,
				},
				Provider: "test-provider",
			},
			expected: func(r RatesResponse) bool {
				return r.Base == "USD" &&
					r.Timestamp == 1640995200 &&
					len(r.Rates) == 3 &&
					r.Rates["EUR"] == 0.85 &&
					r.Rates["GBP"] == 0.73 &&
					r.Rates["JPY"] == 110.0 &&
					r.Provider == "test-provider"
			},
		},
		{
			name: "empty rates response",
			response: RatesResponse{
				Base:      "",
				Timestamp: 0,
				Rates:     map[string]float64{},
				Provider:  "",
			},
			expected: func(r RatesResponse) bool {
				return r.Base == "" &&
					r.Timestamp == 0 &&
					len(r.Rates) == 0 &&
					r.Provider == ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.expected(tt.response) {
				t.Errorf("RatesResponse validation failed")
			}
		})
	}
}

func TestConvertQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    ConvertQuery
		expected func(ConvertQuery) bool
	}{
		{
			name: "valid convert query",
			query: ConvertQuery{
				From:   "USD",
				To:     "EUR",
				Amount: 100.0,
			},
			expected: func(q ConvertQuery) bool {
				return q.From == "USD" &&
					q.To == "EUR" &&
					q.Amount == 100.0
			},
		},
		{
			name: "zero amount",
			query: ConvertQuery{
				From:   "USD",
				To:     "EUR",
				Amount: 0.0,
			},
			expected: func(q ConvertQuery) bool {
				return q.From == "USD" &&
					q.To == "EUR" &&
					q.Amount == 0.0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.expected(tt.query) {
				t.Errorf("ConvertQuery validation failed")
			}
		})
	}
}

func TestConvertResponse(t *testing.T) {
	tests := []struct {
		name     string
		response ConvertResponse
		expected func(ConvertResponse) bool
	}{
		{
			name: "valid convert response",
			response: ConvertResponse{
				From:      "USD",
				To:        "EUR",
				Amount:    100.0,
				Rate:      0.85,
				Converted: 85.0,
				Provider:  "test-provider",
			},
			expected: func(r ConvertResponse) bool {
				return r.From == "USD" &&
					r.To == "EUR" &&
					r.Amount == 100.0 &&
					r.Rate == 0.85 &&
					r.Converted == 85.0 &&
					r.Provider == "test-provider"
			},
		},
		{
			name: "zero conversion",
			response: ConvertResponse{
				From:      "USD",
				To:        "USD",
				Amount:    100.0,
				Rate:      1.0,
				Converted: 100.0,
				Provider:  "test-provider",
			},
			expected: func(r ConvertResponse) bool {
				return r.From == "USD" &&
					r.To == "USD" &&
					r.Amount == 100.0 &&
					r.Rate == 1.0 &&
					r.Converted == 100.0 &&
					r.Provider == "test-provider"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.expected(tt.response) {
				t.Errorf("ConvertResponse validation failed")
			}
		})
	}
}

func TestCacheEntry(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(60 * time.Second)

	tests := []struct {
		name     string
		entry    CacheEntry
		expected func(CacheEntry) bool
	}{
		{
			name: "valid cache entry",
			entry: CacheEntry{
				Data: RatesResponse{
					Base:      "USD",
					Timestamp: now.Unix(),
					Rates: map[string]float64{
						"EUR": 0.85,
					},
					Provider: "test-provider",
				},
				ExpiresAt: expiresAt,
			},
			expected: func(e CacheEntry) bool {
				return e.Data.Base == "USD" &&
					e.Data.Timestamp == now.Unix() &&
					len(e.Data.Rates) == 1 &&
					e.Data.Rates["EUR"] == 0.85 &&
					e.Data.Provider == "test-provider" &&
					e.ExpiresAt.Equal(expiresAt)
			},
		},
		{
			name: "expired cache entry",
			entry: CacheEntry{
				Data: RatesResponse{
					Base:      "USD",
					Timestamp: now.Add(-120 * time.Second).Unix(),
					Rates:     map[string]float64{},
					Provider:  "test-provider",
				},
				ExpiresAt: now.Add(-60 * time.Second),
			},
			expected: func(e CacheEntry) bool {
				return e.Data.Base == "USD" &&
					e.ExpiresAt.Before(now)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.expected(tt.entry) {
				t.Errorf("CacheEntry validation failed")
			}
		})
	}
}

func TestHealthCheck(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		health   HealthCheck
		expected func(HealthCheck) bool
	}{
		{
			name: "healthy status",
			health: HealthCheck{
				Status:    "healthy",
				Timestamp: now,
				Version:   "1.0.0",
				Uptime:    "1m30s",
			},
			expected: func(h HealthCheck) bool {
				return h.Status == "healthy" &&
					h.Timestamp.Equal(now) &&
					h.Version == "1.0.0" &&
					h.Uptime == "1m30s"
			},
		},
		{
			name: "unhealthy status",
			health: HealthCheck{
				Status:    "unhealthy",
				Timestamp: now,
				Version:   "1.0.0",
				Uptime:    "0s",
			},
			expected: func(h HealthCheck) bool {
				return h.Status == "unhealthy" &&
					h.Timestamp.Equal(now) &&
					h.Version == "1.0.0" &&
					h.Uptime == "0s"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.expected(tt.health) {
				t.Errorf("HealthCheck validation failed")
			}
		})
	}
}

func TestAPIResponse(t *testing.T) {
	tests := []struct {
		name     string
		response APIResponse
		expected func(APIResponse) bool
	}{
		{
			name: "valid API response",
			response: APIResponse{
				Data:   map[string]interface{}{"id": 1, "title": "test"},
				Status: 200,
			},
			expected: func(r APIResponse) bool {
				return r.Status == 200 &&
					r.Data != nil
			},
		},
		{
			name: "error API response",
			response: APIResponse{
				Data:   map[string]interface{}{"error": "not found"},
				Status: 404,
			},
			expected: func(r APIResponse) bool {
				return r.Status == 404 &&
					r.Data != nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.expected(tt.response) {
				t.Errorf("APIResponse validation failed")
			}
		})
	}
}

func TestErrorResponse(t *testing.T) {
	tests := []struct {
		name     string
		response ErrorResponse
		expected func(ErrorResponse) bool
	}{
		{
			name: "valid error response",
			response: ErrorResponse{
				Error:   "validation error",
				Message: "invalid input parameters",
				Code:    400,
			},
			expected: func(r ErrorResponse) bool {
				return r.Error == "validation error" &&
					r.Message == "invalid input parameters" &&
					r.Code == 400
			},
		},
		{
			name: "server error response",
			response: ErrorResponse{
				Error:   "internal server error",
				Message: "something went wrong",
				Code:    500,
			},
			expected: func(r ErrorResponse) bool {
				return r.Error == "internal server error" &&
					r.Message == "something went wrong" &&
					r.Code == 500
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.expected(tt.response) {
				t.Errorf("ErrorResponse validation failed")
			}
		})
	}
}
