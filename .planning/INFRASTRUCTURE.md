# NomadCrew Infrastructure Documentation

> **Last Updated:** 2026-02-02
> **Purpose:** Single source of truth for infrastructure decisions and setup

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         CLIENTS                                  │
│            iOS App (React Native) / Android App                  │
└──────────────────────────┬──────────────────────────────────────┘
                           │ HTTPS
┌──────────────────────────▼──────────────────────────────────────┐
│                    SUPABASE CLOUD                                │
│                    (Auth Only)                                   │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  - Google OAuth                                          │    │
│  │  - JWT token issuance                                    │    │
│  │  - User management                                       │    │
│  │  - Free tier: 50K MAU                                    │    │
│  └─────────────────────────────────────────────────────────┘    │
│  Project: kijatqtrwdzltelqzadx                                   │
│  URL: https://kijatqtrwdzltelqzadx.supabase.co                  │
└──────────────────────────┬──────────────────────────────────────┘
                           │ JWT Token
┌──────────────────────────▼──────────────────────────────────────┐
│                    AWS EC2 (Coolify)                             │
│                    api.nomadcrew.uk                              │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  GO BACKEND (main application)                           │    │
│  │  - Gin HTTP framework                                    │    │
│  │  - JWT validation (Supabase JWKS)                        │    │
│  │  - Business logic                                        │    │
│  │  - WebSocket server                                      │    │
│  └─────────────────────────────────────────────────────────┘    │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  POSTGRESQL 17 (self-hosted via Coolify)                 │    │
│  │  - All application data                                  │    │
│  │  - Extensions: uuid-ossp, pgcrypto                       │    │
│  │  - No external dependency                                │    │
│  └─────────────────────────────────────────────────────────┘    │
└──────────────────────────┬──────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────┐
│                    EXTERNAL SERVICES                             │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐             │
│  │   UPSTASH    │ │   RESEND     │ │   EXPO       │             │
│  │   (Redis)    │ │   (Email)    │ │   (Push)     │             │
│  │   Pub/Sub    │ │   Invites    │ │   Notifs     │             │
│  └──────────────┘ └──────────────┘ └──────────────┘             │
│  ┌──────────────┐ ┌──────────────┐                              │
│  │  GEOAPIFY    │ │   PEXELS     │                              │
│  │  (Geocoding) │ │   (Images)   │                              │
│  └──────────────┘ └──────────────┘                              │
└─────────────────────────────────────────────────────────────────┘
```

---

## Service Credentials

### Supabase (Auth Only)

> **Note:** Supabase now uses **JWT Signing Keys** (asymmetric RS256/ES256) instead of legacy JWT secret.
> The backend validates JWTs via JWKS endpoint - no JWT secret needed.

| Key | Value | Format | Use |
|-----|-------|--------|-----|
| Project ID | `kijatqtrwdzltelqzadx` | - | Reference |
| URL | `https://kijatqtrwdzltelqzadx.supabase.co` | - | JWKS endpoint, API calls |
| **Publishable Key** | `sb_publishable_Eh3-ggrUEh9vrMssapcwlg_M__xSXkc` | `sb_publishable_...` | Mobile app, frontend |
| **Secret Key** | `sb_secret__wd0flMhioQ1OF0Hyes05Q_e5dYQA04` | `sb_secret_...` | Go backend server-side calls |
| Legacy Anon Key | `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...` | JWT | Backend JWKS fetch (still works) |

**JWT Validation:** Backend uses JWKS endpoint (`/auth/v1/.well-known/jwks.json`) for asymmetric key validation.
**Where to find keys:** Dashboard → Settings → API Keys

### PostgreSQL (Self-hosted)
| Key | Value |
|-----|-------|
| Host | `localhost` (same server) or Coolify internal |
| Port | `5432` |
| Database | `nomadcrew` |
| User | `postgres` |
| Password | `[SET DURING COOLIFY SETUP]` |

### Redis (Upstash)
| Key | Value |
|-----|-------|
| Host | `[YOUR_UPSTASH_HOST].upstash.io` |
| Port | `6379` |
| Password | `[FROM UPSTASH DASHBOARD]` |
| TLS | `true` |

### External APIs
| Service | Key Location |
|---------|--------------|
| Resend | Dashboard > API Keys |
| Geoapify | Dashboard > API Keys |
| Pexels | Dashboard > API Keys |
| Expo | Auto (via expo-server-sdk) |

---

## Database Schema

### Important: Dual Database Architecture

```
┌─────────────────────────────────┐     ┌─────────────────────────────────┐
│      SUPABASE (Auth Only)       │     │   COOLIFY POSTGRESQL (Data)     │
│                                 │     │                                 │
│  auth.users (managed)           │     │  user_profiles ◄── Backend uses │
│  - id (UUID)                    │────►│  - id (matches auth.users.id)   │
│  - email                        │     │  - email, username              │
│  - created_at                   │     │  - first_name, last_name        │
│                                 │     │  - avatar_url, preferences      │
│  Used for:                      │     │                                 │
│  - Google Sign-In               │     │  All other tables:              │
│  - Apple Sign-In                │     │  - trips, locations, todos      │
│  - JWT token issuance           │     │  - notifications, chat, etc.    │
└─────────────────────────────────┘     └─────────────────────────────────┘
```

**Key Point:** When a user signs in via Supabase, the backend creates a corresponding `user_profiles` row in Coolify PostgreSQL using the same UUID.

### Tables (18 total)

**User & Auth:**
1. `user_profiles` - User data (linked to Supabase auth.users by UUID)

**Trips:**
2. `trips` - Trip entities
3. `trip_memberships` - User-trip relationships
4. `trip_invitations` - Pending invitations
5. `locations` - Location sharing data
6. `todos` - Trip tasks

**Social:**
7. `notifications` - In-app notifications
8. `expenses` - Trip expenses
9. `categories` - Reference data

**Chat:**
10. `chat_groups` - Chat channels
11. `chat_messages` - Chat content
12. `chat_group_members` - Chat membership
13. `chat_message_reactions` - Emoji reactions

**Legacy/Supabase Sync (may be deprecated):**
14. `users` - Legacy user table (use user_profiles instead)
15. `supabase_chat_messages` - Realtime chat (Supabase sync)
16. `supabase_chat_reactions` - Realtime reactions
17. `supabase_chat_read_receipts` - Read tracking
18. `supabase_user_presence` - Online status

### user_profiles Table Schema

```sql
CREATE TABLE public.user_profiles (
    id            UUID PRIMARY KEY,      -- Same as Supabase auth.users.id
    email         TEXT UNIQUE NOT NULL,  -- User's email
    username      TEXT UNIQUE NOT NULL,  -- Display username
    first_name    TEXT DEFAULT '',
    last_name     TEXT DEFAULT '',
    avatar_url    TEXT DEFAULT '',
    contact_email TEXT,                  -- Optional discoverable email
    preferences   JSONB,                 -- User settings
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_user_profiles_email ON user_profiles(email);
CREATE INDEX idx_user_profiles_username ON user_profiles(username);
CREATE INDEX idx_user_profiles_contact_email ON user_profiles(contact_email);
```

### Custom Types (ENUMs)
- `trip_status`: PLANNING, ACTIVE, COMPLETED, CANCELLED
- `membership_role`: OWNER, MEMBER, ADMIN
- `membership_status`: ACTIVE, INACTIVE
- `todo_status`: COMPLETE, INCOMPLETE
- `notification_type`: 10 types
- `invitation_status`: PENDING, ACCEPTED, DECLINED
- `location_privacy`: hidden, approximate, precise

### Extensions Required
- `uuid-ossp` - UUID generation
- `pgcrypto` - Cryptographic functions

---

## Environment Variables

### Required for Production
```env
# Server
PORT=8080
SERVER_ENVIRONMENT=production
ALLOWED_ORIGINS=https://app.nomadcrew.uk
FRONTEND_URL=https://app.nomadcrew.uk
LOG_LEVEL=info

# Database (Self-hosted PostgreSQL)
DATABASE_URL=postgres://postgres:PASSWORD@localhost:5432/nomadcrew?sslmode=disable

# Redis (Upstash)
REDIS_ADDRESS=your-redis.upstash.io:6379
REDIS_PASSWORD=your_upstash_password
REDIS_DB=0
REDIS_USE_TLS=true

# Supabase (Auth Only) - NEW JWT SIGNING KEYS (2025+)
# Backend validates JWTs via JWKS endpoint - no JWT secret needed!
SUPABASE_URL=https://kijatqtrwdzltelqzadx.supabase.co
SUPABASE_ANON_KEY=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6ImtpamF0cXRyd2R6bHRlbHF6YWR4Iiwicm9sZSI6ImFub24iLCJpYXQiOjE3Njk5Mzg0NTYsImV4cCI6MjA4NTUxNDQ1Nn0.LL4YlGqp6q_4u-EavgL6GlT9gByYVRB9irRRkT7lZuU
SUPABASE_SERVICE_KEY=sb_secret__wd0flMhioQ1OF0Hyes05Q_e5dYQA04

# JWT validation uses JWKS (asymmetric keys), set this to disable legacy HS256:
SUPABASE_JWT_SECRET=new-supabase-uses-jwks-validation-instead

# Security
JWT_SECRET_KEY=[MIN 32 CHARS]

# Email (Resend)
RESEND_API_KEY=re_xxxx
EMAIL_FROM_ADDRESS=notifications@nomadcrew.uk
EMAIL_FROM_NAME=NomadCrew

# External APIs
GEOAPIFY_KEY=your_key
PEXELS_API_KEY=your_key
```

### Required for Local Development
```env
# Server
PORT=8080
SERVER_ENVIRONMENT=development
ALLOWED_ORIGINS=http://localhost:3000,http://localhost:5173
FRONTEND_URL=http://localhost:3000
LOG_LEVEL=debug

# Database (Local Docker)
DATABASE_URL=postgres://postgres:password@localhost:5432/nomadcrew?sslmode=disable

# Redis (Local Docker)
REDIS_ADDRESS=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_USE_TLS=false

# Supabase (Same project, or local Supabase)
SUPABASE_URL=https://kijatqtrwdzltelqzadx.supabase.co
SUPABASE_ANON_KEY=...
SUPABASE_SERVICE_KEY=...
SUPABASE_JWT_SECRET=...

# Security
JWT_SECRET_KEY=development-secret-key-minimum-32-characters

# Email (Resend - dev key or skip)
RESEND_API_KEY=re_dev_xxxx
EMAIL_FROM_ADDRESS=dev@nomadcrew.local
EMAIL_FROM_NAME=NomadCrew Dev

# External APIs (dev keys or skip)
GEOAPIFY_KEY=your_dev_key
PEXELS_API_KEY=your_dev_key
```

---

## Local Development Setup

### Prerequisites
- Go 1.24+
- Docker & Docker Compose
- Git

### Quick Start
```bash
# 1. Clone repository
git clone https://github.com/NomadCrew/nomad-crew-backend.git
cd nomad-crew-backend

# 2. Start local services
docker-compose up -d postgres redis

# 3. Copy environment file
cp .env.example.local .env

# 4. Edit .env with your Supabase keys

# 5. Run migrations
psql $DATABASE_URL -f db/migrations/000001_init.up.sql

# 6. Start server
go run main.go

# 7. Verify
curl http://localhost:8080/health
```

### Docker Compose (Local Services)
```yaml
version: "3.8"
services:
  postgres:
    image: postgres:17-alpine
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: password
      POSTGRES_DB: nomadcrew
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD", "pg_isready", "-U", "postgres"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

volumes:
  postgres_data:
```

---

## Production Deployment

### Current Setup
- **Provider:** AWS EC2
- **Instance:** Graviton4 (ARM64)
- **Orchestration:** Coolify
- **Domain:** api.nomadcrew.uk
- **SSL:** Automatic via Coolify

### Deployment Process
1. Push to `main` branch
2. GitHub webhook triggers Coolify
3. Coolify builds Docker image
4. Rolling deployment with health checks

### Coolify PostgreSQL Setup
1. In Coolify dashboard, add new "PostgreSQL" resource
2. Configure:
   - Database name: `nomadcrew`
   - Username: `postgres`
   - Password: `[GENERATE STRONG PASSWORD]`
3. Note the internal connection string
4. Update backend environment variable

---

## Backup & Recovery

### Database Backups
```bash
# Manual backup
pg_dump $DATABASE_URL > backup_$(date +%Y%m%d).sql

# Restore
psql $DATABASE_URL < backup_20260201.sql
```

### Automated Backups (Coolify)
- Enable in Coolify PostgreSQL settings
- Frequency: Daily
- Retention: 7 days
- Storage: S3 or local

---

## Monitoring

### Health Endpoints
- `GET /health` - Full health check
- `GET /health/liveness` - Kubernetes liveness
- `GET /health/readiness` - Kubernetes readiness
- `GET /metrics` - Prometheus metrics

### Key Metrics to Watch
- Database connection pool usage
- Redis pub/sub lag
- API response times (p50, p95, p99)
- Error rates by endpoint
- WebSocket connection count

---

## Troubleshooting

### Common Issues

**Database connection refused**
```bash
# Check PostgreSQL is running
docker ps | grep postgres
# Check connection string
psql $DATABASE_URL -c "SELECT 1"
```

**JWT validation failing**
```bash
# Verify Supabase JWT secret matches
# Check JWKS endpoint is accessible
curl https://kijatqtrwdzltelqzadx.supabase.co/auth/v1/.well-known/jwks.json
```

**Redis connection timeout**
```bash
# For Upstash, ensure TLS is enabled
# Check firewall rules
redis-cli -h your-redis.upstash.io -p 6379 --tls -a YOUR_PASSWORD ping
```

### Mobile App Issues (2026-02-02)

**Google Sign-In DEVELOPER_ERROR 10**
- **Cause:** SHA-1 fingerprint mismatch
- **Fix:** Expo uses its own debug keystore, not system keystore
- **Correct SHA-1:** `5E:8F:16:06:2E:A3:CD:2C:4A:0D:54:78:76:BA:A6:F3:8C:AB:F6:25`
- Add this to Google Cloud Console OAuth Android client for `com.nomadcrew.app.dev`

**Push Notifications FIS_AUTH_ERROR**
- **Cause:** Firebase Installations API not enabled
- **Fix:** Enable these APIs in Google Cloud Console (project: `nomadcrew-11fd4`):
  1. Firebase Installations API
  2. FCM Registration API
  3. Firebase Cloud Messaging API
- Also check API key restrictions allow these APIs

**Missing Database Tables (500 errors)**

Tables required by backend that may be missing after migration:

| Table | Migration Gist |
|-------|----------------|
| `user_profiles` | https://gist.github.com/naqeebali-shamsi/2873041bd902e36cb0ea24cdccfc8ae9 |
| `user_push_tokens` | https://gist.github.com/naqeebali-shamsi/6d85501ad0b1c0e71fc1410c904aa513 |

Run migrations via EC2:
```bash
curl -sL <gist_raw_url> -o /tmp/migration.sql
sudo docker exec -i $(docker ps -q -f name=postgres) psql -U postgres -d postgres < /tmp/migration.sql
```

**ThemeProvider crash (frontend)**
- **Error:** `Cannot read property 'get' of undefined`
- **Cause:** Missing exports in `spacing.ts`
- **Fix:** Added `spacing`, `spacingUtils`, `componentSpacing`, etc. exports

**Notification module null error (frontend)**
- **Error:** `setNotificationChannelAsync of null`
- **Cause:** `Notifications` module never initialized
- **Fix:** Added `initializeNotificationsModule()` that sets `Notifications = ExpoNotifications` on physical devices

---

## Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-02-02 | Use Firebase/FCM for Android push notifications | Android platform requirement - no alternative |
| 2026-02-02 | Store google-services.json in SECRET/ folder | Keep sensitive config organized, gitignore if needed |
| 2026-02-02 | Use `user_push_tokens` table for push tokens | Required for backend push notification delivery |
| 2026-02-02 | Use `user_profiles` table for user data | Backend expects this table, separates from legacy `users` table |
| 2026-02-02 | Link user_profiles.id to Supabase auth.users.id | Maintains referential integrity across systems |
| 2026-02-01 | Move from Supabase DB to self-hosted | Free tier limits, project deletion risk |
| 2026-02-01 | Keep Supabase for Auth only | Good OAuth support, free tier sufficient |
| 2026-02-01 | Keep Upstash Redis | Serverless, pay-per-use, low maintenance |

---

## Future Considerations

### Offline-First Mobile (When Ready)
- Add PowerSync (~$49/mo)
- Enables SQLite sync on mobile
- No backend changes required

### Scaling Options
- Horizontal: Add more EC2 instances behind load balancer
- Vertical: Upgrade instance size
- Database: Read replicas if needed

---

## Contact & Resources

- **API Docs:** https://api.nomadcrew.uk/swagger/index.html
- **Supabase Dashboard:** https://supabase.com/dashboard/project/kijatqtrwdzltelqzadx
- **Coolify Dashboard:** [Your Coolify URL]
- **Upstash Dashboard:** https://console.upstash.com
