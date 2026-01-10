# Technology Stack

**Analysis Date:** 2026-01-10

## Languages

**Primary:**
- Go 1.24.0 - All application code (`go.mod`)

**Secondary:**
- SQL - Database queries and migrations (`db/queries/*.sql`, `db/migrations/*.sql`)
- YAML - Configuration files (`config/*.yaml`, `sqlc.yaml`, `docker-compose.yml`)

## Runtime

**Environment:**
- Go 1.24.0 (latest stable)
- Linux/Docker containers in production

**Package Manager:**
- Go modules
- Lockfile: `go.sum` present

## Frameworks

**Core:**
- Gin v1.10.0 - HTTP web framework (`router/router.go`, `handlers/*.go`)
- pgx/v5 v5.7.6 - PostgreSQL driver (`db/db_client.go`)

**Testing:**
- testify v1.10.0 - Assertions and mocking (`*_test.go`)
- testcontainers-go v0.37.0 - Integration testing with Docker

**Build/Dev:**
- Air - Hot reloading for development (`.air.toml`, `Dockerfile.dev`)
- sqlc - Type-safe SQL code generation (`sqlc.yaml`)
- swag - Swagger documentation generation (`docs/`)

## Key Dependencies

**Critical:**
- gin-gonic/gin v1.10.0 - HTTP routing and middleware (`router/router.go`)
- jackc/pgx/v5 v5.7.6 - PostgreSQL connection pool (`db/db_client.go`)
- redis/go-redis/v9 v9.7.3 - Redis client for caching and pub/sub (`internal/events/`)
- supabase-community/supabase-go v0.0.4 - Supabase client for auth and realtime (`main.go`)
- golang-jwt/jwt/v5 v5.2.1 - JWT token handling (`internal/auth/jwt.go`)
- nhooyr.io/websocket v1.8.17 - WebSocket support (`internal/websocket/`)

**Infrastructure:**
- uber-go/zap v1.27.0 - Structured logging (`logger/logger.go`)
- spf13/viper v1.19.0 - Configuration management (`config/config.go`)
- prometheus/client_golang v1.14.0 - Metrics (`router/router.go`)
- resend/resend-go/v2 v2.15.0 - Email service (`services/email_service.go`)
- lestrrat-go/jwx/v2 v2.0.21 - JWKS validation (`middleware/jwks_cache.go`)

## Configuration

**Environment:**
- `.env` files for local development
- Environment variables in production
- Required vars: `DB_CONNECTION_STRING`, `REDIS_ADDRESS`, `SUPABASE_URL`, `SUPABASE_ANON_KEY`, `SUPABASE_SERVICE_KEY`, `JWT_SECRET_KEY`

**Build:**
- `config/config.*.yaml` - Environment-specific configuration
- `sqlc.yaml` - SQL code generation config
- `.air.toml` - Hot reload configuration

## Platform Requirements

**Development:**
- Any platform with Go 1.24+ and Docker
- Docker Compose for local PostgreSQL and Redis
- Air for hot reloading

**Production:**
- Docker containers
- Fly.io deployment (`fly.toml`)
- Neon PostgreSQL (serverless)
- Upstash Redis (serverless)
- Supabase for auth and realtime features

---

*Stack analysis: 2026-01-10*
*Update after major dependency changes*
