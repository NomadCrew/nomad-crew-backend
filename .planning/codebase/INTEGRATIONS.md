# External Integrations

**Analysis Date:** 2026-01-10

## APIs & External Services

**Supabase:**
- Purpose: Authentication, realtime sync, user management
- SDK/Client: `supabase-community/supabase-go` v0.0.4
- Auth: `SUPABASE_URL`, `SUPABASE_ANON_KEY`, `SUPABASE_SERVICE_KEY`, `SUPABASE_JWT_SECRET`
- Features used: JWT auth, realtime subscriptions, user metadata
- Integration files: `main.go`, `services/supabase_service.go`, `middleware/jwt_validator.go`

**Resend (Email):**
- Purpose: Transactional emails (invitations, notifications)
- SDK/Client: `resend/resend-go/v2` v2.15.0
- Auth: `RESEND_API_KEY` env var
- Config: `EMAIL_FROM_ADDRESS`, `EMAIL_FROM_NAME`
- Integration file: `services/email_service.go`

**Pexels API:**
- Purpose: Trip cover images
- SDK/Client: Custom client in `pkg/pexels/`
- Auth: `PEXELS_API_KEY` env var
- Integration files: `pkg/pexels/client.go`, `handlers/trip_handler.go`

**Geoapify:**
- Purpose: Location/geocoding services
- Auth: `GEOAPIFY_KEY` env var
- Integration: Used for destination validation

**Expo Push Notifications:**
- Purpose: Mobile push notifications
- Client: `internal/notification/client.go`
- Integration: `services/push_service.go`
- Token storage: `push_tokens` table via `sqlcadapter/push_token_store.go`

## Data Storage

**PostgreSQL (Neon):**
- Provider: Neon (serverless PostgreSQL)
- Connection: `DB_CONNECTION_STRING` or individual `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`
- Client: `jackc/pgx/v5` v5.7.6 with connection pooling
- ORM: None - uses SQLC for type-safe queries
- Migrations: `db/migrations/` (numbered SQL files)
- Schema files: `db/migrations/000001_init.up.sql` and subsequent migrations
- Query files: `db/queries/*.sql`
- Generated code: `internal/sqlc/`

**Redis (Upstash):**
- Provider: Upstash (serverless Redis)
- Connection: `REDIS_ADDRESS`, `REDIS_PASSWORD`, `REDIS_DB`
- Client: `redis/go-redis/v9` v9.7.3
- Features used:
  - Pub/Sub for real-time events (`internal/events/redis_publisher.go`)
  - Rate limiting (`middleware/rate_limit.go`)
- TLS: Enabled in production via Upstash

## Authentication & Identity

**Auth Provider:**
- Supabase Auth - JWT-based authentication
- Implementation: JWKS validation via `lestrrat-go/jwx/v2`
- Token storage: Bearer token in Authorization header
- Session management: Stateless JWT with Supabase refresh

**JWKS Validation:**
- JWKS endpoint: `https://{project}.supabase.co/auth/v1/.well-known/jwks.json`
- Cache: `middleware/jwks_cache.go`
- Validator: `middleware/jwt_validator.go`

**Internal JWT (Legacy):**
- Library: `golang-jwt/jwt/v5`
- Used for: Internal token generation if needed
- Config: `JWT_SECRET_KEY` env var
- Files: `internal/auth/jwt.go`, `internal/auth/jwt_rotation.go`

## Monitoring & Observability

**Prometheus Metrics:**
- Endpoint: `/metrics`
- Client: `prometheus/client_golang` v1.14.0
- Integration: `router/router.go`

**Logging:**
- Framework: Zap (`go.uber.org/zap`)
- Level: Configurable via `LOG_LEVEL` env var
- Files: `logger/logger.go`

**Error Tracking:**
- Not detected (no Sentry or similar integration)
- Recommendation: Add Sentry for production error tracking

## CI/CD & Deployment

**Hosting:**
- Platform: Fly.io (`fly.toml`)
- Deployment: Docker containers
- Environment vars: Configured in Fly.io dashboard

**Docker:**
- Production: `Dockerfile`
- Development: `Dockerfile.dev` (with Air hot reload)
- Compose: `docker-compose.yml` for local development

**GitHub Actions:**
- Workflow: `.github/workflows/test.yml`
- Purpose: Run tests on PR/push

## Environment Configuration

**Development:**
- Required env vars: See `.env.example`
- Critical vars:
  - `DB_CONNECTION_STRING` - PostgreSQL connection
  - `REDIS_ADDRESS`, `REDIS_PASSWORD` - Redis connection
  - `SUPABASE_URL`, `SUPABASE_ANON_KEY`, `SUPABASE_SERVICE_KEY` - Supabase
  - `JWT_SECRET_KEY` - Legacy JWT signing
- Secrets location: `.env` file (gitignored)
- Local services: Docker Compose for PostgreSQL and Redis

**Production:**
- Secrets management: Fly.io environment variables
- Database: Neon PostgreSQL (serverless)
- Redis: Upstash Redis (serverless with TLS)
- Supabase: Production project

**Environment-Specific Config:**
- Config files: `config/config.{environment}.yaml`
- Environment detection: `SERVER_ENVIRONMENT` env var
- Files: `config/env_specific_config.go`

## Webhooks & Callbacks

**Incoming:**
- None detected (no webhook endpoints)

**Outgoing:**
- Event publishing to Redis pub/sub
- Push notifications to Expo Push Service
- Supabase realtime sync

## Rate Limiting

**Implementation:**
- Middleware: `middleware/rate_limit.go`
- Storage: Redis
- Config: `RATE_LIMIT_AUTH_REQUESTS_PER_MINUTE`, `RATE_LIMIT_WINDOW_SECONDS`
- Applied to: Auth endpoints (onboarding)

---

*Integration audit: 2026-01-10*
*Update when adding/removing external services*
