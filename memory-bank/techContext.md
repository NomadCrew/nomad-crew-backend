# Technical Context

## Technology Stack

### Core Technologies
- **Language:** Go 1.21+
- **Framework:** Gin (Web Framework)
- **Database:** PostgreSQL 15+
- **Cache:** Redis 7+

### Key Dependencies
- `github.com/gin-gonic/gin` - HTTP web framework
- `github.com/jackc/pgx/v5` - PostgreSQL driver and toolkit
- `github.com/redis/go-redis/v9` - Redis client
- `github.com/golang-jwt/jwt/v5` - JWT authentication
- `go.uber.org/zap` - Structured logging
- `github.com/spf13/viper` - Configuration management

### External Services
1. **Open-Meteo API**
   - Weather data provider
   - Free tier with no API key required
   - Endpoint: `api.open-meteo.com`

2. **Nominatim**
   - Geocoding service (fallback)
   - Free, open-source
   - Rate limited

## Development Setup

### Prerequisites
1. Go 1.21 or higher
2. PostgreSQL 15+
3. Redis 7+
4. Make (for running commands)

### Environment Variables
```env
# Server
PORT=8080
GIN_MODE=debug

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=nomad_crew

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# JWT
JWT_SECRET=your-secret-key
JWT_EXPIRATION=24h

# External Services
WEATHER_API_BASE_URL=https://api.open-meteo.com/v1
```

### Local Development
1. Clone repository
2. Copy `.env.example` to `.env`
3. Install dependencies: `go mod download`
4. Start PostgreSQL and Redis
5. Run migrations: `make migrate-up`
6. Start server: `make run`

### Database Migrations
- Location: `db/migrations/`
- Tool: `golang-migrate`
- Commands:
  - `make migrate-up` - Apply migrations
  - `make migrate-down` - Rollback migrations
  - `make migrate-create name=xyz` - Create new migration

### Testing
- Run tests: `make test`
- Run with coverage: `make test-coverage`
- Integration tests require running PostgreSQL and Redis

## Technical Constraints

### Performance
- Redis caching for frequently accessed data
- Connection pooling for PostgreSQL
- Rate limiting on public endpoints

### Security
- JWT-based authentication
- CORS configuration
- Input validation
- Prepared statements for SQL
- Environment variable management
- No sensitive data in logs

### Scalability
- Stateless application design
- Redis for session management
- Configurable database pool size
- Graceful shutdown handling

## Dependencies

### Core Dependencies
```go
module nomad-crew-backend

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/jackc/pgx/v5 v5.5.0
	github.com/redis/go-redis/v9 v9.3.0
	github.com/golang-jwt/jwt/v5 v5.0.0
	go.uber.org/zap v1.26.0
	github.com/spf13/viper v1.17.0
)
```

### Development Dependencies
- `golangci-lint` - Linting
- `golang-migrate` - Database migrations
- `mockgen` - Mock generation for testing
- `swag` - Swagger documentation

## Monitoring & Logging

### Logging
- Structured logging with Zap
- Log levels: DEBUG, INFO, WARN, ERROR
- Request ID tracking
- Performance metrics

### Metrics
- Request duration
- Database query timing
- Cache hit/miss rates
- Error rates by endpoint

## Deployment

### Requirements
- Go runtime
- PostgreSQL database
- Redis instance
- Environment configuration

### Process
1. Build binary: `make build`
2. Run migrations
3. Configure environment
4. Start application

*This file documents technical implementation details and requirements.* 