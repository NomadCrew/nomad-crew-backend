# NomadCrew Migration Checklist

> **Goal:** Migrate from Supabase-managed database to self-hosted PostgreSQL
> **Time Estimate:** 2-3 hours
> **Difficulty:** Medium

---

## Pre-Migration Status

- [x] New Supabase project created (for Auth only)
- [x] Schema restored via migrations (17 tables, all with RLS enabled)
- [x] MCP integration configured (`.mcp.json`)
- [ ] Self-hosted PostgreSQL set up
- [ ] Backend configured for new infrastructure
- [ ] OAuth configured
- [ ] Production deployed

### Verified via MCP (2026-02-01)
- **Project URL:** `https://kijatqtrwdzltelqzadx.supabase.co`
- **Tables:** 17 (all with RLS enabled, 0 rows - ready for production)
- **Security Advisories:** 6 minor warnings (function search_path - non-critical)

---

## Phase 1: Supabase Auth Configuration (30 min)

### 1.1 Get API Keys (New Format - 2025+)

> **Note:** Supabase now uses `sb_publishable_...` and `sb_secret_...` format keys.
> Legacy `anon`/`service_role` keys still work but are deprecated.

- [x] Go to: https://supabase.com/dashboard/project/kijatqtrwdzltelqzadx/settings/api-keys
- [x] Copy and save securely:
  - [x] **Publishable Key**: `sb_publishable_Eh3-ggrUEh9vrMssapcwlg_M__xSXkc` (for mobile app)
  - [x] **Legacy Anon Key**: `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...` (backend uses this for JWKS fetch)
  - [x] **Secret Key**: `sb_secret__wd0flMhioQ1OF0Hyes05Q_e5dYQA04` (for Go backend server-side)
  - [x] **JWT Validation**: Uses JWKS endpoint (asymmetric keys) - NO secret needed!

### 1.2 Configure Google OAuth
- [ ] Go to: https://console.cloud.google.com/apis/credentials
- [ ] Create or select project
- [ ] Create OAuth 2.0 Client ID (Web application)
- [ ] Add authorized redirect URI:
  ```
  https://kijatqtrwdzltelqzadx.supabase.co/auth/v1/callback
  ```
- [ ] Copy Client ID and Client Secret
- [ ] Go to: Supabase Dashboard → Authentication → Providers → Google
- [ ] Enable Google provider
- [ ] Paste Client ID and Client Secret
- [ ] Save

### 1.3 Configure Auth Settings
- [ ] Go to: Supabase Dashboard → Authentication → URL Configuration
- [ ] Set Site URL: `https://app.nomadcrew.uk` (or your mobile app deep link)
- [ ] Add Redirect URLs:
  ```
  https://app.nomadcrew.uk/**
  nomadcrew://auth/callback
  exp://localhost:8081/--/auth/callback
  ```

---

## Phase 2: Self-Hosted PostgreSQL Setup (45 min)

### Option A: Using Coolify (Recommended)

#### 2A.1 Add PostgreSQL in Coolify
- [ ] Log into Coolify dashboard
- [ ] Click "New Resource" → "Database" → "PostgreSQL"
- [ ] Configure:
  - Database Name: `nomadcrew`
  - Username: `postgres`
  - Password: `[GENERATE 32+ CHAR PASSWORD]`
  - Version: `17`
- [ ] Deploy and wait for healthy status

#### 2A.2 Get Connection String
- [ ] In Coolify, click on the PostgreSQL resource
- [ ] Copy the internal connection string (for backend on same server):
  ```
  postgres://postgres:PASSWORD@postgres-container:5432/nomadcrew
  ```
- [ ] Or external connection string (if needed):
  ```
  postgres://postgres:PASSWORD@your-server-ip:5432/nomadcrew
  ```

### Option B: Docker Compose (Alternative)

#### 2B.1 SSH into EC2
```bash
ssh -i your-key.pem ubuntu@your-ec2-ip
```

#### 2B.2 Create docker-compose.yml
```yaml
version: "3.8"
services:
  postgres:
    image: postgres:17-alpine
    restart: always
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: YOUR_SECURE_PASSWORD
      POSTGRES_DB: nomadcrew
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD", "pg_isready", "-U", "postgres"]
      interval: 30s
      timeout: 10s
      retries: 5

volumes:
  postgres_data:
```

#### 2B.3 Start PostgreSQL
```bash
docker-compose up -d postgres
docker-compose ps  # Verify it's running
```

---

## Phase 3: Database Schema Setup (30 min)

### 3.1 Connect to New PostgreSQL
```bash
# Using psql
psql postgres://postgres:PASSWORD@localhost:5432/nomadcrew

# Or via Docker
docker exec -it postgres-container psql -U postgres -d nomadcrew
```

### 3.2 Run Migrations
```bash
# From your local machine with migrations
psql $DATABASE_URL -f db/migrations/000001_init.up.sql
psql $DATABASE_URL -f db/migrations/000002_*.up.sql
# ... continue for all migrations

# Or run all at once
for f in db/migrations/*.up.sql; do
  psql $DATABASE_URL -f "$f"
done
```

### 3.3 Verify Schema
```sql
-- Check tables exist
\dt

-- Should show:
-- users, trips, trip_memberships, trip_invitations,
-- locations, todos, notifications, expenses, categories,
-- chat_groups, chat_messages, chat_group_members, chat_message_reactions,
-- supabase_chat_messages, supabase_chat_reactions,
-- supabase_chat_read_receipts, supabase_user_presence

-- Check extensions
\dx

-- Should show uuid-ossp, pgcrypto
```

---

## Phase 4: Backend Configuration (30 min)

### 4.1 Update Environment Variables

Create or update `.env` for production:

```env
# Database (Self-hosted)
DATABASE_URL=postgres://postgres:YOUR_PASSWORD@postgres:5432/nomadcrew?sslmode=disable

# Supabase (Auth Only) - NEW JWT SIGNING KEYS (2025+)
# Backend validates JWTs via JWKS endpoint - uses asymmetric keys!
SUPABASE_URL=https://kijatqtrwdzltelqzadx.supabase.co
SUPABASE_ANON_KEY=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6ImtpamF0cXRyd2R6bHRlbHF6YWR4Iiwicm9sZSI6ImFub24iLCJpYXQiOjE3Njk5Mzg0NTYsImV4cCI6MjA4NTUxNDQ1Nn0.LL4YlGqp6q_4u-EavgL6GlT9gByYVRB9irRRkT7lZuU
SUPABASE_SERVICE_KEY=sb_secret__wd0flMhioQ1OF0Hyes05Q_e5dYQA04

# This sentinel value tells the backend to use JWKS validation (asymmetric keys)
# instead of legacy HS256 shared secret validation
SUPABASE_JWT_SECRET=new-supabase-uses-jwks-validation-instead

# Keep other settings the same
```

### 4.2 Update Coolify Environment
- [ ] Go to Coolify → Your Backend Service → Environment Variables
- [ ] Update `DATABASE_URL` to point to self-hosted PostgreSQL
- [ ] Update Supabase keys to new project
- [ ] Save and redeploy

### 4.3 Test Backend Connection
```bash
# Check logs after deploy
coolify logs backend-service

# Or test health endpoint
curl https://api.nomadcrew.uk/health
```

---

## Phase 5: Mobile App Configuration (15 min)

### 5.1 Update Supabase Client Config
Find your Supabase initialization code and update:

```javascript
// Before (old project)
const supabaseUrl = 'https://old-project.supabase.co'
const supabaseAnonKey = 'old-key...'

// After (new project)
const supabaseUrl = 'https://kijatqtrwdzltelqzadx.supabase.co'
const supabaseAnonKey = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6ImtpamF0cXRyd2R6bHRlbHF6YWR4Iiwicm9sZSI6ImFub24iLCJpYXQiOjE3Njk5Mzg0NTYsImV4cCI6MjA4NTUxNDQ1Nn0.LL4YlGqp6q_4u-EavgL6GlT9gByYVRB9irRRkT7lZuU'
```

### 5.2 Test OAuth Flow
- [ ] Build app locally
- [ ] Test Google Sign-In
- [ ] Verify JWT is received
- [ ] Test API call with JWT

---

## Phase 6: Verification (30 min)

### 6.1 Auth Flow
- [ ] Sign up new user with Google
- [ ] Sign in existing user
- [ ] Refresh token works
- [ ] Logout works

### 6.2 Core Features
- [ ] Create a trip
- [ ] Invite a member (check email sends)
- [ ] Accept invitation
- [ ] Send chat message
- [ ] Update location
- [ ] Create todo

### 6.3 Real-time Features
- [ ] WebSocket connects
- [ ] Chat messages appear in real-time
- [ ] Presence updates work

### 6.4 Push Notifications
- [ ] Register device token
- [ ] Receive test notification

---

## Phase 7: Cleanup (15 min)

### 7.1 Remove Old Resources
- [ ] Delete old NeonDB database (if applicable)
- [ ] Remove old Supabase project (if accessible)
- [ ] Update any documentation referencing old project

### 7.2 Update Documentation
- [ ] Update README with new setup instructions
- [ ] Update any deployment docs
- [ ] Commit `.planning/INFRASTRUCTURE.md`

---

## Rollback Plan

If something goes wrong:

1. **Backend won't start:**
   - Check DATABASE_URL is correct
   - Verify PostgreSQL container is running
   - Check Coolify logs

2. **Auth not working:**
   - Verify SUPABASE_JWT_SECRET matches dashboard
   - Check JWKS endpoint is accessible
   - Ensure OAuth redirect URIs are correct

3. **Database connection issues:**
   - Check firewall allows port 5432
   - Verify credentials
   - Test with psql directly

---

## Post-Migration Tasks

- [ ] Set up automated database backups
- [ ] Configure monitoring/alerting
- [ ] Document backup restoration process
- [ ] Test disaster recovery

---

## Support Contacts

- **Supabase:** support@supabase.io or Discord
- **Coolify:** Discord community
- **Upstash:** support@upstash.com

---

## Notes

_Add any notes or issues encountered during migration:_

```
Date:
Issue:
Resolution:
```
