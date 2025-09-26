# Currency Exchange API

A high-performance real-time currency exchange rate microservice built with Go and the Gin framework. This service aggregates exchange rates from multiple free APIs, provides currency conversion capabilities, and offers a robust RESTful API for real-time financial data.

## Features

- **Real-Time Exchange Rates**: Fetches live currency rates from 4 different free APIs
- **Multi-Provider Aggregation**: Concurrent fetching from Exchange Rate API, Open Exchange Rates, Frankfurter, and Exchange Rate Host
- **Currency Conversion**: Convert between any supported currencies with real-time rates
- **High Performance**: Built with Gin framework for optimal speed and low latency
- **Rate Limiting**: Token bucket rate limiting per client IP to prevent abuse
- **Concurrent Processing**: Efficient handling using goroutines and channels
- **Smart Caching**: In-memory caching with configurable TTL to reduce API calls
- **Health Monitoring**: Comprehensive health checks with external API status
- **Security**: Automatic security headers and request tracking
- **Production Ready**: Docker support, graceful shutdown, and comprehensive logging

## API Endpoints

### Health Check
- `GET /health` - Service health status with external API connectivity

### Currency Exchange
- `GET /api/v1/rates` - Get exchange rates (default: USD base)
- `GET /api/v1/rates/:base` - Get rates for specific base currency
- `GET /api/v1/convert?from=USD&to=EUR&amount=100` - Convert between currencies
- `GET /api/v1/currencies` - List supported currencies

### Legacy Endpoints (Backward Compatibility)
- `GET /api/v1/posts` - Fetch all posts
- `GET /api/v1/posts/:id` - Fetch specific post by ID
- `GET /api/v1/users` - Fetch all users
- `GET /api/v1/comments` - Fetch all comments

## Quick Start

### Using Docker Compose (Recommended)

1. Clone the repository
2. Run the service:
   ```bash
   docker-compose up --build
   ```

The service will be available at `http://localhost:8080`

### Running Locally

1. **Prerequisites**:
   - Go 1.21 or later
   - Git

2. **Install dependencies**:
   ```bash
   go mod download
   ```

3. **Run the service**:
   ```bash
   go run main.go
   ```

4. **Test the service**:
   ```bash
   # Health check
   curl http://localhost:8080/health
   
   # Get USD rates
   curl http://localhost:8080/api/v1/rates
   
   # Get EUR rates
   curl http://localhost:8080/api/v1/rates/EUR
   
   # Convert 100 USD to EUR
   curl "http://localhost:8080/api/v1/convert?from=USD&to=EUR&amount=100"
   
   # Get supported currencies
   curl http://localhost:8080/api/v1/currencies
   ```

## API Usage Examples

### Currency Exchange Rates

**Get rates for USD (default):**
```bash
curl http://localhost:8080/api/v1/rates
```

**Get rates for EUR:**
```bash
curl http://localhost:8080/api/v1/rates/EUR
```

**Response:**
```json
{
  "base": "USD",
  "timestamp": 1640995200,
  "rates": {
    "EUR": 0.85,
    "GBP": 0.73,
    "JPY": 110.25
  },
  "provider": "erapi"
}
```

### Currency Conversion

**Convert 100 USD to EUR:**
```bash
curl "http://localhost:8080/api/v1/convert?from=USD&to=EUR&amount=100"
```

**Response:**
```json
{
  "from": "USD",
  "to": "EUR",
  "amount": 100,
  "rate": 0.85,
  "converted": 85.0,
  "provider": "erapi"
}
```

### Supported Currencies

**Get list of supported currencies:**
```bash
curl http://localhost:8080/api/v1/currencies
```

**Response:**
```json
{
  "currencies": ["USD", "EUR", "GBP", "JPY", "AUD", "CAD", "CHF", "CNY", "SEK", "NZD", "BRL", "RUB", "INR", "KRW", "SGD", "HKD", "NOK", "MXN", "TRY", "ZAR", "PLN", "CZK", "HUF", "ILS", "CLP", "PHP", "AED", "COP", "SAR", "THB"],
  "count": 30
}
```

## Configuration

The service can be configured using environment variables. Copy `env.example` to `.env` and modify as needed:

```bash
cp env.example .env
```

### Available Configuration Options

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `EXCHANGE_RATE_API_BASE_URL` | `https://open.er-api.com/v6/latest` | Exchange Rate API base URL |
| `EXCHANGE_RATE_API_KEY` | `` | Exchange Rate API key (optional) |
| `OPEN_EXCHANGE_RATES_BASE_URL` | `https://openexchangerates.org/api/latest.json` | Open Exchange Rates base URL |
| `OPEN_EXCHANGE_RATES_API_KEY` | `` | Open Exchange Rates API key (optional) |
| `FRANKFURTER_API_BASE_URL` | `https://api.frankfurter.app/latest` | Frankfurter API base URL |
| `EXCHANGE_RATE_HOST_BASE_URL` | `https://api.exchangerate.host/latest` | Exchange Rate Host base URL |
| `RATES_CACHE_TTL_SECONDS` | `60` | Cache TTL in seconds |

## Project Structure

```
.
├── main.go                 # Application entry point
├── go.mod                  # Go module definition
├── Dockerfile              # Docker configuration
├── docker-compose.yml      # Docker Compose configuration
├── env.example             # Environment variables example
├── README.md               # This file
└── internal/               # Internal packages
    ├── api/                # HTTP handlers and routes
    │   └── handlers.go
    ├── config/             # Configuration management
    │   └── config.go
    ├── logger/             # Logging utilities
    │   └── logger.go
    ├── middleware/         # Gin middleware
    │   └── gin_middleware.go
    ├── models/             # Data models
    │   └── models.go
    ├── platform/           # Platform-specific code
    │   ├── shutdown_windows.go
    │   └── shutdown_unix.go
    ├── ratelimit/          # Rate limiting
    │   └── limiter.go
    └── service/            # Business logic services
        ├── api_service.go
        └── rates_service.go
```

## Development

### Adding New API Endpoints

1. Add new methods to the appropriate service in `internal/service/`
2. Add corresponding handlers in `internal/api/handlers.go`
3. Register new routes in the `SetupRoutes()` method

### Adding New Middleware

1. Create middleware functions in `internal/middleware/gin_middleware.go`
2. Add them to the middleware chain in `SetupRoutes()`

### Adding New Data Models

1. Define new structs in `internal/models/models.go`
2. Update the service methods to handle the new data types

### Testing

```bash
# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with verbose output
go test -v ./...
```

## Monitoring and Observability

### Health Check

The service provides a health check endpoint at `/health` that returns:
- Service status
- Timestamp
- Version
- Uptime
- External API connectivity status

### Logging

The service uses structured JSON logging with the following levels:
- `debug`: Detailed information for debugging
- `info`: General information about service operation
- `warn`: Warning messages for potentially harmful situations
- `error`: Error messages for failed operations

### Metrics

Consider adding metrics collection using libraries like:
- Prometheus client for Go
- OpenTelemetry for distributed tracing

## Production Considerations

1. **Security**:
   - Use HTTPS in production
   - Implement proper authentication and authorization
   - Validate and sanitize all inputs
   - Use secrets management for API keys

2. **Performance**:
   - Implement caching for frequently accessed data
   - Use connection pooling for database connections
   - Consider implementing rate limiting

3. **Monitoring**:
   - Set up health check monitoring
   - Implement distributed tracing
   - Add application metrics
   - Set up alerting for critical errors

4. **Deployment**:
   - Use container orchestration (Kubernetes, Docker Swarm)
   - Implement blue-green or rolling deployments
   - Set up proper logging aggregation
   - Configure auto-scaling

## License

This project is licensed under the MIT License.



