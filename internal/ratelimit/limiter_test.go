package ratelimit

import (
	"currency-exchange-api/internal/testutils"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewLimiter(t *testing.T) {
	cfg := testutils.MockConfig()
	logger := testutils.MockLogger()

	limiter := NewLimiter(cfg, logger)

	if limiter == nil {
		t.Fatal("NewLimiter() returned nil")
	}
	if limiter.Configuration != cfg {
		t.Errorf("NewLimiter() configuration = %v, want %v", limiter.Configuration, cfg)
	}
	if limiter.logger != logger {
		t.Errorf("NewLimiter() logger = %v, want %v", limiter.logger, logger)
	}
	if limiter.clientBuckets == nil {
		t.Errorf("NewLimiter() clientBuckets is nil")
	}
	if limiter.cleanupTicker == nil {
		t.Errorf("NewLimiter() cleanupTicker is nil")
	}
	if limiter.stopCleanup == nil {
		t.Errorf("NewLimiter() stopCleanup is nil")
	}
}

func TestLimiter_Allow(t *testing.T) {
	tests := []struct {
		name             string
		rateLimitEnabled bool
		clientIP         string
		requests         int
		expected         []bool
	}{
		{
			name:             "rate limiting disabled",
			rateLimitEnabled: false,
			clientIP:         "192.168.1.1",
			requests:         5,
			expected:         []bool{true, true, true, true, true},
		},
		{
			name:             "rate limiting enabled - within limit",
			rateLimitEnabled: true,
			clientIP:         "192.168.1.1",
			requests:         3,
			expected:         []bool{true, true, true},
		},
		{
			name:             "rate limiting enabled - exceed limit",
			rateLimitEnabled: true,
			clientIP:         "192.168.1.1",
			requests:         12, // More than burst limit (10)
			expected:         []bool{true, true, true, true, true, true, true, true, true, true, false, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testutils.MockConfig()
			cfg.RateLimitEnabled = tt.rateLimitEnabled
			cfg.RateLimitBurst = 10
			cfg.RateLimitRequests = 100
			cfg.RateLimitWindow = 60 * time.Second

			logger := testutils.MockLogger()
			limiter := NewLimiter(cfg, logger)

			// Make requests
			for i := 0; i < tt.requests; i++ {
				result := limiter.Allow(tt.clientIP)
				expected := tt.expected[i]
				if result != expected {
					t.Errorf("Allow() request %d = %v, want %v", i, result, expected)
				}
			}
		})
	}
}

func TestLimiter_Allow_DifferentIPs(t *testing.T) {
	cfg := testutils.MockConfig()
	cfg.RateLimitEnabled = true
	cfg.RateLimitBurst = 5
	cfg.RateLimitRequests = 100
	cfg.RateLimitWindow = 60 * time.Second

	logger := testutils.MockLogger()
	limiter := NewLimiter(cfg, logger)

	// Test different IPs should have separate buckets
	ip1 := "192.168.1.1"
	ip2 := "192.168.1.2"

	// Both IPs should be able to make requests within their limits
	for i := 0; i < 5; i++ {
		if !limiter.Allow(ip1) {
			t.Errorf("Allow() IP1 request %d = false, want true", i)
		}
		if !limiter.Allow(ip2) {
			t.Errorf("Allow() IP2 request %d = false, want true", i)
		}
	}

	// Both IPs should be rate limited after exceeding burst
	if limiter.Allow(ip1) {
		t.Errorf("Allow() IP1 after burst = true, want false")
	}
	if limiter.Allow(ip2) {
		t.Errorf("Allow() IP2 after burst = true, want false")
	}
}

func TestLimiter_GetClientIP(t *testing.T) {
	cfg := testutils.MockConfig()
	logger := testutils.MockLogger()
	limiter := NewLimiter(cfg, logger)

	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		expected   string
	}{
		{
			name: "X-Forwarded-For header",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.195",
			},
			remoteAddr: "192.168.1.1:12345",
			expected:   "203.0.113.195",
		},
		{
			name: "X-Real-IP header",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.195",
			},
			remoteAddr: "192.168.1.1:12345",
			expected:   "203.0.113.195",
		},
		{
			name:       "RemoteAddr fallback",
			headers:    map[string]string{},
			remoteAddr: "192.168.1.1:12345",
			expected:   "192.168.1.1",
		},
		{
			name: "X-Forwarded-For with port",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.195:8080",
			},
			remoteAddr: "192.168.1.1:12345",
			expected:   "203.0.113.195",
		},
		{
			name: "Invalid X-Forwarded-For falls back to RemoteAddr",
			headers: map[string]string{
				"X-Forwarded-For": "invalid-ip",
			},
			remoteAddr: "192.168.1.1:12345",
			expected:   "192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr

			for header, value := range tt.headers {
				req.Header.Set(header, value)
			}

			result := limiter.GetClientIP(req)
			if result != tt.expected {
				t.Errorf("GetClientIP() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestLimiter_Middleware(t *testing.T) {
	cfg := testutils.MockConfig()
	cfg.RateLimitEnabled = true
	cfg.RateLimitBurst = 2
	cfg.RateLimitRequests = 100
	cfg.RateLimitWindow = 60 * time.Second

	logger := testutils.MockLogger()
	limiter := NewLimiter(cfg, logger)

	// Create a test handler
	handler := limiter.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Test within rate limit
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Middleware() within limit status = %v, want %v", w.Code, http.StatusOK)
	}

	// Test exceeding rate limit - make requests until we hit the limit
	successCount := 0
	rateLimitedCount := 0

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Handle status codes using tagged switch
		switch w.Code {
		case http.StatusOK:
			successCount++
		case http.StatusTooManyRequests:
			rateLimitedCount++
		default:
			// Log unexpected status codes for debugging
			t.Logf("Unexpected status code: %d", w.Code)
		}
	}

	// We should have some successful requests and some rate limited
	if successCount == 0 {
		t.Errorf("Middleware() no successful requests")
	}
	if rateLimitedCount == 0 {
		t.Errorf("Middleware() no rate limited requests")
	}
}

func TestTokenBucket_Allow(t *testing.T) {
	tests := []struct {
		name         string
		capacity     int
		tokens       int
		refillRate   int
		refillPeriod time.Duration
		requests     int
		expected     []bool
	}{
		{
			name:         "sufficient tokens",
			capacity:     5,
			tokens:       5,
			refillRate:   10,
			refillPeriod: 1 * time.Second,
			requests:     3,
			expected:     []bool{true, true, true},
		},
		{
			name:         "insufficient tokens",
			capacity:     5,
			tokens:       2,
			refillRate:   10,
			refillPeriod: 1 * time.Second,
			requests:     5,
			expected:     []bool{true, true, false, false, false},
		},
		{
			name:         "no tokens",
			capacity:     5,
			tokens:       0,
			refillRate:   10,
			refillPeriod: 1 * time.Second,
			requests:     3,
			expected:     []bool{false, false, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket := &TokenBucket{
				capacity:     tt.capacity,
				tokens:       tt.tokens,
				lastRefill:   time.Now(),
				refillRate:   tt.refillRate,
				refillPeriod: tt.refillPeriod,
			}

			for i := 0; i < tt.requests; i++ {
				result := bucket.Allow()
				expected := tt.expected[i]
				if result != expected {
					t.Errorf("TokenBucket.Allow() request %d = %v, want %v", i, result, expected)
				}
			}
		})
	}
}

func TestLimiter_Stop(t *testing.T) {
	cfg := testutils.MockConfig()
	logger := testutils.MockLogger()
	limiter := NewLimiter(cfg, logger)

	// Stop should not panic
	limiter.Stop()

	// Give cleanup goroutine time to stop
	time.Sleep(100 * time.Millisecond)
}
