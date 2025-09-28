package ratelimit

import (
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/dalfonso89/currency-exchange-service/config"
	"github.com/dalfonso89/currency-exchange-service/logger"
)

// Limiter implements a token bucket rate limiter per IP
type Limiter struct {
	Configuration *config.Config
	logger        logger.Logger

	// Map of IP -> token bucket
	clientBuckets map[string]*TokenBucket
	bucketsMutex  sync.RWMutex

	// Cleanup goroutine control
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// TokenBucket represents a token bucket for rate limiting
type TokenBucket struct {
	capacity     int
	tokens       int
	lastRefill   time.Time
	refillRate   int
	refillPeriod time.Duration
}

// NewLimiter creates a new rate limiter
func NewLimiter(configuration *config.Config, logger logger.Logger) *Limiter {
	rateLimiter := &Limiter{
		Configuration: configuration,
		logger:        logger,
		clientBuckets: make(map[string]*TokenBucket),
		cleanupTicker: time.NewTicker(2 * time.Minute),
		stopCleanup:   make(chan struct{}),
	}

	// Start cleanup goroutine
	go rateLimiter.cleanup()

	return rateLimiter
}

// Allow checks if a request from the given IP is allowed
func (rateLimiter *Limiter) Allow(clientIP string) bool {
	if !rateLimiter.Configuration.RateLimitEnabled {
		return true
	}

	rateLimiter.bucketsMutex.Lock()
	defer rateLimiter.bucketsMutex.Unlock()

	// Get or create bucket for this IP
	bucket, exists := rateLimiter.clientBuckets[clientIP]
	if !exists {
		bucket = &TokenBucket{
			capacity:     rateLimiter.Configuration.RateLimitBurst,
			tokens:       rateLimiter.Configuration.RateLimitBurst,
			lastRefill:   time.Now(),
			refillRate:   rateLimiter.Configuration.RateLimitRequests,
			refillPeriod: rateLimiter.Configuration.RateLimitWindow,
		}
		rateLimiter.clientBuckets[clientIP] = bucket
	}

	return bucket.Allow()
}

// Middleware returns an HTTP middleware for rate limiting
func (rateLimiter *Limiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			clientIP := rateLimiter.GetClientIP(request)

			if !rateLimiter.Allow(clientIP) {
				rateLimiter.logger.Warnf("Rate limit exceeded for IP: %s", clientIP)
				responseWriter.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rateLimiter.Configuration.RateLimitRequests))
				responseWriter.Header().Set("X-RateLimit-Remaining", "0")
				responseWriter.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(rateLimiter.Configuration.RateLimitWindow).Unix()))
				http.Error(responseWriter, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(responseWriter, request)
		})
	}
}

// GetClientIP extracts the real client IP from the request
func (rateLimiter *Limiter) GetClientIP(request *http.Request) string {
	// Check X-Forwarded-For header
	if xForwardedFor := request.Header.Get("X-Forwarded-For"); xForwardedFor != "" {
		if clientIP := net.ParseIP(xForwardedFor); clientIP != nil {
			return clientIP.String()
		}
		// If multiple IPs, take the first one
		if host, _, err := net.SplitHostPort(xForwardedFor); err == nil {
			if clientIP := net.ParseIP(host); clientIP != nil {
				return clientIP.String()
			}
		}
	}

	// Check X-Real-IP header
	if xRealIP := request.Header.Get("X-Real-IP"); xRealIP != "" {
		if clientIP := net.ParseIP(xRealIP); clientIP != nil {
			return clientIP.String()
		}
	}

	// Fall back to RemoteAddr
	clientIP, _, parseError := net.SplitHostPort(request.RemoteAddr)
	if parseError != nil {
		return request.RemoteAddr
	}
	return clientIP
}

// cleanup removes old buckets to prevent memory leaks
func (rateLimiter *Limiter) cleanup() {
	for {
		select {
		case <-rateLimiter.cleanupTicker.C:
			rateLimiter.bucketsMutex.Lock()
			currentTime := time.Now()
			for clientIP, bucket := range rateLimiter.clientBuckets {
				// Remove buckets that haven't been refilled for a long time
				if currentTime.Sub(bucket.lastRefill) > bucket.refillPeriod*2 {
					delete(rateLimiter.clientBuckets, clientIP)
				}
			}
			rateLimiter.bucketsMutex.Unlock()
		case <-rateLimiter.stopCleanup:
			rateLimiter.cleanupTicker.Stop()
			return
		}
	}
}

// Stop stops the cleanup goroutine
func (rateLimiter *Limiter) Stop() {
	close(rateLimiter.stopCleanup)
}

// Allow checks if a token is available in the bucket
func (tokenBucket *TokenBucket) Allow() bool {
	now := time.Now()

	// Refill tokens based on time elapsed
	timeElapsed := now.Sub(tokenBucket.lastRefill)
	tokensToAdd := int(timeElapsed / tokenBucket.refillPeriod * time.Duration(tokenBucket.refillRate))

	if tokensToAdd > 0 {
		tokenBucket.tokens = min(tokenBucket.capacity, tokenBucket.tokens+tokensToAdd)
		tokenBucket.lastRefill = now
	}

	// Check if we have tokens available
	if tokenBucket.tokens > 0 {
		tokenBucket.tokens--
		return true
	}

	return false
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
