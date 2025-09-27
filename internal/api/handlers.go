package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"currency-exchange-api/internal/middleware"
	"currency-exchange-api/internal/models"
	"currency-exchange-api/internal/ratelimit"
	"currency-exchange-api/internal/service"
)

// Handlers contains all HTTP handlers
type Handlers struct {
	apiService   *service.APIService
	logger       *logrus.Logger
	startTime    time.Time
	ratesService *service.RatesService
	rateLimiter  *ratelimit.Limiter
}

// NewHandlers creates a new handlers instance
func NewHandlers(apiService *service.APIService, logger *logrus.Logger) *Handlers {
	return &Handlers{
		apiService: apiService,
		logger:     logger,
		startTime:  time.Now(),
	}
}

// WithRates attaches the RatesService after initialization
func (handlers *Handlers) WithRates(ratesService *service.RatesService) *Handlers {
	handlers.ratesService = ratesService
	return handlers
}

// WithRateLimit attaches the rate limiter after initialization
func (handlers *Handlers) WithRateLimit(rateLimiter *ratelimit.Limiter) *Handlers {
	handlers.rateLimiter = rateLimiter
	return handlers
}

// SetupRoutes configures all the routes using Gin
func (handlers *Handlers) SetupRoutes() *gin.Engine {
	// Set Gin mode based on environment
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()

	// Add custom Gin middleware
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

		// Legacy API routes (for backward compatibility)
		apiV1.GET("/posts", handlers.GetPosts)
		apiV1.GET("/posts/:id", handlers.GetPostByID)
		apiV1.GET("/users", handlers.GetUsers)
		apiV1.GET("/comments", handlers.GetComments)
	}

	return router
}

// HealthCheck handles health check requests
func (handlers *Handlers) HealthCheck(context *gin.Context) {
	requestContext := context.Request.Context()

	// Check external API health
	apiHealthError := handlers.apiService.HealthCheck(requestContext)

	healthStatus := "healthy"
	if apiHealthError != nil {
		healthStatus = "unhealthy"
		handlers.logger.Warnf("External API health check failed: %v", apiHealthError)
	}

	healthCheckResponse := models.HealthCheck{
		Status:    healthStatus,
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

	exchangeRates, fetchError := handlers.ratesService.GetRates(requestContext, baseCurrency)
	if fetchError != nil {
		handlers.writeErrorResponse(context, http.StatusBadGateway, "failed to fetch rates", fetchError.Error())
		return
	}

	context.JSON(http.StatusOK, exchangeRates)
}

// GetRatesByBase returns rates for a specific base currency using path parameter
func (handlers *Handlers) GetRatesByBase(context *gin.Context) {
	if handlers.ratesService == nil {
		handlers.writeErrorResponse(context, http.StatusServiceUnavailable, "rates service unavailable", "not configured")
		return
	}

	baseCurrency := strings.ToUpper(context.Param("base"))
	requestContext := context.Request.Context()

	exchangeRates, fetchError := handlers.ratesService.GetRates(requestContext, baseCurrency)
	if fetchError != nil {
		handlers.writeErrorResponse(context, http.StatusBadGateway, "failed to fetch rates", fetchError.Error())
		return
	}

	context.JSON(http.StatusOK, exchangeRates)
}

// GetPosts handles requests to fetch all posts
func (handlers *Handlers) GetPosts(context *gin.Context) {
	requestContext := context.Request.Context()

	handlers.logger.Info("Fetching all posts")

	posts, fetchError := handlers.apiService.FetchPosts(requestContext)
	if fetchError != nil {
		handlers.logger.Errorf("Failed to fetch posts: %v", fetchError)
		handlers.writeErrorResponse(context, http.StatusInternalServerError, "Failed to fetch posts", fetchError.Error())
		return
	}

	apiResponse := models.APIResponse{
		Data:   posts,
		Status: http.StatusOK,
	}

	context.JSON(http.StatusOK, apiResponse)
}

// GetPostByID handles requests to fetch a specific post by ID
func (handlers *Handlers) GetPostByID(context *gin.Context) {
	postIDString := context.Param("id")
	postID, parseError := strconv.Atoi(postIDString)
	if parseError != nil {
		handlers.writeErrorResponse(context, http.StatusBadRequest, "Invalid post ID", "Post ID must be a number")
		return
	}

	requestContext := context.Request.Context()
	handlers.logger.Infof("Fetching post with ID: %d", postID)

	post, fetchError := handlers.apiService.FetchPostByID(requestContext, postID)
	if fetchError != nil {
		handlers.logger.Errorf("Failed to fetch post %d: %v", postID, fetchError)
		handlers.writeErrorResponse(context, http.StatusInternalServerError, "Failed to fetch post", fetchError.Error())
		return
	}

	apiResponse := models.APIResponse{
		Data:   post,
		Status: http.StatusOK,
	}

	context.JSON(http.StatusOK, apiResponse)
}

// GetUsers handles requests to fetch all users
func (handlers *Handlers) GetUsers(context *gin.Context) {
	requestContext := context.Request.Context()

	handlers.logger.Info("Fetching all users")

	users, fetchError := handlers.apiService.FetchUsers(requestContext)
	if fetchError != nil {
		handlers.logger.Errorf("Failed to fetch users: %v", fetchError)
		handlers.writeErrorResponse(context, http.StatusInternalServerError, "Failed to fetch users", fetchError.Error())
		return
	}

	apiResponse := models.APIResponse{
		Data:   users,
		Status: http.StatusOK,
	}

	context.JSON(http.StatusOK, apiResponse)
}

// GetComments handles requests to fetch all comments
func (handlers *Handlers) GetComments(context *gin.Context) {
	requestContext := context.Request.Context()

	handlers.logger.Info("Fetching all comments")

	comments, fetchError := handlers.apiService.FetchComments(requestContext)
	if fetchError != nil {
		handlers.logger.Errorf("Failed to fetch comments: %v", fetchError)
		handlers.writeErrorResponse(context, http.StatusInternalServerError, "Failed to fetch comments", fetchError.Error())
		return
	}

	apiResponse := models.APIResponse{
		Data:   comments,
		Status: http.StatusOK,
	}

	context.JSON(http.StatusOK, apiResponse)
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

// corsMiddleware adds CORS headers using Gin middleware
func (handlers *Handlers) corsMiddleware() gin.HandlerFunc {
	return func(context *gin.Context) {
		context.Header("Access-Control-Allow-Origin", "*")
		context.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		context.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if context.Request.Method == "OPTIONS" {
			context.AbortWithStatus(http.StatusOK)
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
