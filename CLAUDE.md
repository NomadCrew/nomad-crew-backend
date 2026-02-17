# CLAUDE.md — NomadCrew Backend

Parent context: `../CLAUDE.md` (project-wide rules, cross-repo integration, agent patterns)

## Development Commands

```bash
# Run the server
go run main.go

# Run with hot reload
air

# Run tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Lint code
golangci-lint run

# Format code
gofmt -w .

# Generate Swagger docs
swag init -g main.go -o ./static/docs/api

# Run migrations
psql -d <db> -f db/migrations/init.sql
```

## High-Level Architecture

### Stack
- **Language:** Go 1.24
- **Framework:** Gin
- **Database:** PostgreSQL (pgx driver)
- **Cache:** Redis
- **Auth:** Supabase + JWT
- **Real-time:** WebSockets (nhooyr.io/websocket)
- **Logging:** Uber Zap
- **Config:** Viper

### Project Structure
```
├── main.go              # Entry point
├── router/              # API routes and middleware setup
├── handlers/            # HTTP handlers (parse input, call services)
├── models/*/service/    # Business logic (NEW - use this pattern)
├── services/            # Legacy business logic (migrate away from)
├── db/                  # Data access layer (pgx queries)
├── middleware/          # JWT, CORS, rate limiting, error handling
├── internal/
│   ├── ws/              # WebSocket manager
│   └── events/          # Event definitions and dispatch
├── types/               # Shared type definitions
├── errors/              # Custom error types
└── config/              # Configuration loading
```

### Key Patterns

1. **Layered Architecture:**
   - Handlers → Services → DB
   - Permission checks happen in service layer
   - Handlers only parse/validate input

2. **New Feature Convention:**
   - Place in `models/<feature>/service/`
   - NOT in legacy `services/` directory

3. **Error Handling:**
   - Use custom error types from `errors/`
   - Wrap errors with context
   - See Logging Architecture below for where to log

4. **WebSocket Pattern:**
   - Trip-specific: `/v1/trips/:id/chat/ws/events`
   - General updates: `/v1/ws`

## Logging Architecture

Errors log **once** at the middleware boundary. No manual logging before `c.Error()`.

### Rules by Layer

| Layer | Rule |
|---|---|
| **DB/Store** (`internal/store/`, `db/`) | Return errors only. Never log errors. |
| **Services** (`models/*/service/`, `services/`) | Return errors only. Never log errors that are returned. |
| **Handlers** (`handlers/`) | Call `c.Error(err)` only. Never call `log.Error/Warn` before `c.Error()`. |
| **Auth/RBAC middleware** | Call `c.Error(err)` only. No manual logging. |
| **Error handler middleware** | Logs once per error: **WARN** for 4xx, **ERROR** for 5xx. |

### Exceptions (when logging IS allowed)

- **Non-fatal errors where execution continues** (e.g., weather update fails but trip creation succeeds) — use `Warnw`
- **Background goroutines** that don't return errors to callers — log at appropriate level
- **Event publishing failures** where the error is swallowed — use `Warnw`
- **Debug-level tracing** — always fine, filtered by `LOG_LEVEL`

### What NOT to do

```go
// BAD: log-and-return (creates duplicate when caller also logs)
log.Errorw("Failed to get user", "error", err)
return nil, fmt.Errorf("failed to get user: %w", err)

// GOOD: just return (let the caller/middleware log)
return nil, fmt.Errorf("failed to get user: %w", err)

// BAD: log-then-c.Error (middleware already logs via c.Error)
log.Errorw("Failed to list items", "error", err)
c.Error(errors.NewDatabaseError(err))

// GOOD: just c.Error (middleware logs once)
c.Error(errors.NewDatabaseError(err))
```

### Security

Never log secrets, tokens, or API keys. Use `logger.MaskJWT()`, `logger.MaskEmail()`, etc.

### Reference Implementation

`handlers/todo_handler.go` — uses `c.Error()` exclusively with no manual logging.

## Feature Modules

**Trips:**
- Models: Trip, Trip Member
- Database tables: `trips`, `trip_members`
- Service layer: `models/trip/service/`
- Key handlers: `handlers/trip_handler.go`

**Chat:**
- Models: Message, Read Receipt
- Database tables: `chat_messages`, `chat_read_receipts`
- Service layer: `models/chat/service/`
- WebSocket: `/v1/trips/:id/chat/ws/events`
- Real-time message delivery and read status

**Todos:**
- Models: Todo, TodoAssignment
- Database tables: `todos`, `todo_assignments`
- Service layer: `models/todo/service/`
- CRUD operations, assignment tracking
- Trip integration: `TripModelInterface` in `models/todo.go` provides `GetTripMembers()`

**Polls:**
- Models: Poll, PollOption, PollVote
- Database tables: `polls`, `poll_options`, `poll_votes`
- Migration: `000008_add_poll_expiration.sql` (adds `expires_at` column)
- Service layer: `models/poll/service/`
- Features:
  - Expiration: `expires_at` timestamp, `DurationMinutes` on create (5-2880 min, default 1440 = 24h)
  - Close restrictions: poll can only be manually closed when expired OR all trip members have voted
  - Voting blocked on expired polls
  - Vote counting: `CountUniqueVotersByPoll()` in PollStore interface
- Trip integration: Uses `GetTripMembers()` for vote quorum check

**Wallet:**
- Models: WalletDocument (personal & group document storage)
- Database tables: `wallet_documents` (with `wallet_type` and `document_type` enums)
- Migration: `000012_wallet_documents.up.sql`
- Service layer: `models/wallet/service/`
- Handler: `handlers/wallet_handler.go`
- Features:
  - Personal documents: passport, visa, insurance, vaccination, loyalty cards
  - Group documents: flight bookings, hotel bookings, reservations, receipts (trip-scoped)
  - File upload: multipart, max 10MB, allowed types: PDF, JPEG, PNG, HEIC
  - MIME detection: `gabriel-vasile/mimetype` library (magic-byte based, not extension)
  - File storage: local filesystem (`/var/data/wallet-files/`), swappable `FileStorage` interface
  - Download: HMAC-signed JWT URLs → `GET /v1/wallet/files/:token` → `http.ServeFile`
  - Soft delete: `deleted_at` column, filtered by default
  - RBAC: personal = owner only; group = trip members (reuses `RequirePermission` middleware)
- API endpoints:
  - `POST/GET /v1/wallet/documents` — personal documents
  - `GET/PUT/DELETE /v1/wallet/documents/:id` — personal document by ID
  - `POST/GET /v1/trips/:id/wallet/documents` — group documents (trip-scoped)
  - `GET /v1/wallet/files/:token` — signed file download
- Test coverage: 38 service tests + 30 handler tests

**Notifications:**
- Models: Notification
- Database tables: `notifications`
- Service layer: `models/notification/service/`
- Push notifications via Expo Push API

**Location:**
- Models: Location
- Database tables: `locations`
- Service layer: `models/location/service/`
- WebSocket: Real-time location updates via `/v1/ws`

## Code Principles

- Use structured logging (Zap) — see Logging Architecture above
- Validate all input at handler level
- Check permissions in service layer
- Use context for request-scoped values
- Prefer explicit error handling over panics
- Write tests for new functionality

## Environment Variables

Key variables (see `.env.example` for full list):
- `DB_CONNECTION_STRING` - PostgreSQL connection
- `REDIS_ADDRESS` - Redis connection
- `SUPABASE_URL`, `SUPABASE_ANON_KEY`, `SUPABASE_SERVICE_KEY`
- `JWT_SECRET_KEY`, `SUPABASE_JWT_SECRET`
- `SERVER_ENVIRONMENT` - development/production
- `LOG_LEVEL` - debug/info/warn/error

## Testing

```bash
# Unit tests
go test ./...

# With verbose output
go test -v ./...

# Specific package
go test ./handlers/...

# Integration tests (requires testcontainers)
go test -tags=integration ./...
```

## API Documentation

- Swagger UI: `/swagger/index.html` (when server running)
- Generate docs: `swag init -g main.go -o ./static/docs/api`
- Use proper Swagger annotations on handlers

## Deployment

- **Production:** https://api.nomadcrew.uk
- **CI/CD:** GitHub Actions → Coolify webhook
- **Infrastructure:** AWS EC2 (Graviton4) + Docker

## Database Migration SoP (Production)

### Architecture
- **Supabase:** Auth only (Google/Apple Sign-In, JWT tokens)
- **Coolify PostgreSQL:** All application data (self-hosted on EC2)

### Running Migrations via GitHub Gist

When you need to run SQL migrations on the production Coolify PostgreSQL:

**Step 1: Create a public GitHub Gist with the SQL**
```bash
gh gist create --public --filename "migration_name.sql" --desc "Description" - << 'EOF'
-- Your SQL here
CREATE TABLE IF NOT EXISTS ...
EOF
```

**Step 2: On EC2, download the migration**
```bash
curl -sL https://gist.githubusercontent.com/<user>/<gist_id>/raw/<filename>.sql -o /tmp/migration.sql
```

**Step 3: Execute via Docker**
```bash
docker exec -i $(docker ps -q -f name=postgres) psql -U postgres -d postgres < /tmp/migration.sql
```

### Why This Approach?
- Large SQL blocks don't paste correctly in EC2 console
- Gists provide version history and easy sharing
- curl + pipe is reliable for multi-line SQL

### Example: Adding user_profiles table
```bash
# Download
curl -sL https://gist.githubusercontent.com/naqeebali-shamsi/2873041bd902e36cb0ea24cdccfc8ae9/raw/add_user_profiles_table.sql -o /tmp/migration.sql

# Execute
docker exec -i $(docker ps -q -f name=postgres) psql -U postgres -d postgres < /tmp/migration.sql
```

### Verify Migration
```bash
docker exec -i $(docker ps -q -f name=postgres) psql -U postgres -d postgres -c "\dt"
```

## Push Notifications

Push notifications are sent via Expo Push API → APNs (iOS) / FCM (Android).

### Key Files
- `services/push_service.go` - Expo Push API client
- `models/notification/service/notification_service.go` - Notification creation & push triggering

### Debugging Push Failures
1. Check logs for ticket ID: `Push notification ticket successful {"ticketId": "..."}`
2. Query Expo receipt API:
   ```bash
   curl -X POST https://exp.host/--/api/v2/push/getReceipts \
     -H "Content-Type: application/json" \
     -d '{"ids": ["<TICKET_ID>"]}'
   ```
3. `{"status": "ok"}` = delivered; `{"status": "error", ...}` = check error details

### iOS APNs Configuration
- APNs keys (.p8 files) are configured in Expo, NOT in the backend
- **Full setup guide:** See `nomad-crew-frontend/docs/PUSH_NOTIFICATIONS_SETUP.md`
- Common errors: `InvalidProviderToken`, `BadEnvironmentKeyInToken` → usually need new APNs key
