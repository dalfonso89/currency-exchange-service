package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"currency-exchange-api/logger"
	"currency-exchange-api/middleware"
	"currency-exchange-api/models"
	"currency-exchange-api/ratelimit"
	"currency-exchange-api/service"
)

// HandlerConfig contains all dependencies for the Handlers
type HandlerConfig struct {
	Logger       logger.Logger
	RatesService *service.RatesService
	RateLimiter  *ratelimit.Limiter
}

// Handlers contains all HTTP handlers
type Handlers struct {
	logger       logger.Logger
	startTime    time.Time
	ratesService *service.RatesService
	rateLimiter  *ratelimit.Limiter
}

// NewHandlers creates a new handlers instance with all dependencies
func NewHandlers(config HandlerConfig) *Handlers {
	return &Handlers{
		logger:       config.Logger,
		startTime:    time.Now(),
		ratesService: config.RatesService,
		rateLimiter:  config.RateLimiter,
	}
}

// SetupRoutes configures all the routes using Gin
func (handlers *Handlers) SetupRoutes() *gin.Engine {
	// Set Gin mode based on environment
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Apply middleware
	router.Use(middleware.RequestLogger(handlers.logger))
	router.Use(gin.Recovery())
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.RequestID())
	router.Use(handlers.corsMiddleware())

	// Add rate limiting middleware if enabled
	if handlers.rateLimiter != nil {
		router.Use(handlers.rateLimitMiddleware())
	}

	// Health check endpoint
	router.GET("/health", handlers.HealthCheck)

	// API v1 routes
	apiV1 := router.Group("/api/v1")
	{
		// Currency exchange routes
		apiV1.GET("/rates", handlers.GetRates)
		apiV1.GET("/rates/:base", handlers.GetRatesByBase)
	}

	return router
}

// HealthCheck handles health check requests
func (handlers *Handlers) HealthCheck(context *gin.Context) {
	healthCheckResponse := models.HealthCheck{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Uptime:    time.Since(handlers.startTime).String(),
	}

	context.JSON(http.StatusOK, healthCheckResponse)
}

// GetRates returns latest rates for a base currency
func (handlers *Handlers) GetRates(context *gin.Context) {
	if handlers.ratesService == nil {
		handlers.writeErrorResponse(context, http.StatusServiceUnavailable, "rates service unavailable", "not configured")
		return
	}

	baseCurrency := context.DefaultQuery("base", "USD")
	requestContext := context.Request.Context()

	_, fetchError := handlers.ratesService.GetRates(requestContext, baseCurrency)
	if fetchError != nil {
		handlers.handleServiceError(context, fetchError)
		return
	}

	// Assuming exchangeRates is returned by GetRates, but it's currently ignored.
	// For this example, we'll just return a placeholder if no error.
	context.JSON(http.StatusOK, gin.H{"message": "Rates fetched successfully (placeholder)"})
}

// GetRatesByBase returns rates for a specific base currency using path parameter
func (handlers *Handlers) GetRatesByBase(context *gin.Context) {
	if handlers.ratesService == nil {
		handlers.writeErrorResponse(context, http.StatusServiceUnavailable, "rates service unavailable", "not configured")
		return
	}

	baseCurrency := strings.ToUpper(context.Param("base"))
	requestContext := context.Request.Context()

	_, fetchError := handlers.ratesService.GetRates(requestContext, baseCurrency)
	if fetchError != nil {
		handlers.handleServiceError(context, fetchError)
		return
	}

	// Assuming exchangeRates is returned by GetRates, but it's currently ignored.
	// For this example, we'll just return a placeholder if no error.
	context.JSON(http.StatusOK, gin.H{"message": "Rates fetched successfully (placeholder)"})
}

// writeErrorResponse writes an error response using Gin context
func (handlers *Handlers) writeErrorResponse(context *gin.Context, statusCode int, errorMessage, errorDetails string) {
	errorResponse := models.ErrorResponse{
		Error:   errorMessage,
		Message: errorDetails,
		Code:    statusCode,
	}

	context.JSON(statusCode, errorResponse)
}

// handleServiceError handles service errors using type switches
func (handlers *Handlers) handleServiceError(context *gin.Context, err error) {
	// Use type switch for error handling
	switch e := err.(type) {
	case *service.ServiceError:
		switch e.Type {
		case service.ErrorTypeNoProviders:
			handlers.writeErrorResponse(context, http.StatusServiceUnavailable, "no providers configured", e.Error())
		case service.ErrorTypeContextCancelled:
			handlers.writeErrorResponse(context, http.StatusRequestTimeout, "request cancelled", e.Error())
		case service.ErrorTypeNetworkError:
			handlers.writeErrorResponse(context, http.StatusBadGateway, "network error", e.Error())
		case service.ErrorTypeInvalidResponse:
			handlers.writeErrorResponse(context, http.StatusBadGateway, "invalid response", e.Error())
		default:
			handlers.writeErrorResponse(context, http.StatusInternalServerError, "service error", e.Error())
		}
	default:
		// Handle generic errors
		handlers.writeErrorResponse(context, http.StatusBadGateway, "failed to fetch rates", err.Error())
	}
}

// corsMiddleware adds CORS headers using Gin middleware
func (handlers *Handlers) corsMiddleware() gin.HandlerFunc {
	return func(context *gin.Context) {
		context.Header("Access-Control-Allow-Origin", "*")
		context.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		context.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle HTTP method using type switch
		switch context.Request.Method {
		case "OPTIONS":
			context.AbortWithStatus(http.StatusOK)
			return
		case "GET", "POST", "PUT", "DELETE":
			// Continue processing
		default:
			context.AbortWithStatus(http.StatusMethodNotAllowed)
			return
		}

		context.Next()
	}
}

// rateLimitMiddleware provides rate limiting using Gin middleware
func (handlers *Handlers) rateLimitMiddleware() gin.HandlerFunc {
	return func(context *gin.Context) {
		clientIP := handlers.rateLimiter.GetClientIP(context.Request)

		if !handlers.rateLimiter.Allow(clientIP) {
			handlers.logger.Warnf("Rate limit exceeded for IP: %s", clientIP)
			context.Header("X-RateLimit-Limit", strconv.Itoa(handlers.rateLimiter.Configuration.RateLimitRequests))
			context.Header("X-RateLimit-Remaining", "0")
			context.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(handlers.rateLimiter.Configuration.RateLimitWindow).Unix(), 10))
			context.JSON(http.StatusTooManyRequests, gin.H{"error": "Rate limit exceeded"})
			context.Abort()
			return
		}

		context.Next()
	}
}
