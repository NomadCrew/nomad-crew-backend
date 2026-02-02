# NomadCrew Backend Troubleshooting Guide

> **Last Updated:** 2026-02-02

## Database Issues

### Missing Tables (500 errors)

If the backend returns 500 errors with messages like `relation "xxx" does not exist`, the table needs to be created.

**Required tables:**

| Table | Purpose | Migration |
|-------|---------|-----------|
| `user_profiles` | User data linked to Supabase auth | [Gist](https://gist.github.com/naqeebali-shamsi/2873041bd902e36cb0ea24cdccfc8ae9) |
| `user_push_tokens` | Push notification tokens | [Gist](https://gist.github.com/naqeebali-shamsi/6d85501ad0b1c0e71fc1410c904aa513) |

**Running migrations on Coolify PostgreSQL:**

```bash
# SSH to EC2, then:

# 1. Download migration
curl -sL <gist_raw_url> -o /tmp/migration.sql

# 2. Execute
sudo docker exec -i $(docker ps -q -f name=postgres) psql -U postgres -d postgres < /tmp/migration.sql
```

### Database Architecture

```
┌─────────────────────────────────┐     ┌─────────────────────────────────┐
│      SUPABASE (Auth Only)       │     │   COOLIFY POSTGRESQL (Data)     │
│                                 │     │                                 │
│  auth.users (managed)           │────▶│  user_profiles                  │
│  - id (UUID)                    │     │  - id (matches auth.users.id)   │
│  - email                        │     │  - email, username, etc.        │
│                                 │     │                                 │
│  Used for:                      │     │  All other tables:              │
│  - Google Sign-In               │     │  - trips, locations, todos      │
│  - Apple Sign-In                │     │  - notifications, chat, etc.    │
│  - JWT token issuance           │     │  - user_push_tokens             │
└─────────────────────────────────┘     └─────────────────────────────────┘
```

---

## Authentication Issues

### JWT Validation Failing

**Check JWKS endpoint:**
```bash
curl https://eihszqnmmgbrcxtymskn.supabase.co/auth/v1/.well-known/jwks.json
```

**Verify backend can reach Supabase:**
```bash
docker exec -it <backend_container> curl -I https://eihszqnmmgbrcxtymskn.supabase.co
```

### "Authorization header missing" Errors

This usually means the mobile app isn't sending the JWT token. Check:
1. User is authenticated in the app
2. API client is attaching `Authorization: Bearer <token>` header
3. Token hasn't expired

---

## Coolify/Docker Issues

### Check Container Logs

```bash
# List containers
docker ps

# View logs
docker logs <container_id> --tail 100 -f

# Check backend specifically
docker logs $(docker ps -q -f name=backend) --tail 100
```

### Database Connection

```bash
# Test PostgreSQL connection
docker exec -it $(docker ps -q -f name=postgres) psql -U postgres -d postgres -c "SELECT 1"

# List tables
docker exec -it $(docker ps -q -f name=postgres) psql -U postgres -d postgres -c "\dt"
```

### Redis Connection

```bash
# Test Redis (if self-hosted)
docker exec -it $(docker ps -q -f name=redis) redis-cli ping

# For Upstash (TLS required)
redis-cli -h <host>.upstash.io -p 6379 --tls -a <password> ping
```

---

## Common Error Messages

| Error | Cause | Fix |
|-------|-------|-----|
| `relation "user_profiles" does not exist` | Missing table | Run user_profiles migration |
| `relation "user_push_tokens" does not exist` | Missing table | Run user_push_tokens migration |
| `token_missing` | No Authorization header | Check mobile app sends JWT |
| `invalid_token` | JWT validation failed | Check Supabase JWT secret config |
| `FIS_AUTH_ERROR` | Firebase not configured | See mobile troubleshooting |

---

## Health Check

```bash
# Full health check
curl https://api.nomadcrew.uk/health

# Expected response:
# {"status":"healthy","database":"up","redis":"up"}
```

---

## See Also

- [Infrastructure Documentation](../.planning/INFRASTRUCTURE.md)
- [Frontend Troubleshooting](../../nomad-crew-frontend/docs/DEVELOPMENT_SETUP.md)
