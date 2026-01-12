---
phase: 17-domain-ssl-config
plan: 01
subsystem: infra
tags: [cloudflare, lets-encrypt, ssl, traefik, coolify, dns]

# Dependency graph
requires:
  - phase: 16-application-deployment
    provides: Running application at http://3.130.209.141:8081
provides:
  - Production HTTPS API at https://api.nomadcrew.uk
  - Let's Encrypt SSL certificate (auto-renewing)
  - HTTP to HTTPS redirect
affects: [monitoring, frontend-config, api-clients]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Coolify Traefik reverse proxy for SSL termination
    - DNS-only (gray cloud) Cloudflare for Let's Encrypt compatibility

key-files:
  created: []
  modified:
    - infrastructure/aws/main.tf
    - infrastructure/aws/terraform.tfvars

key-decisions:
  - "Upgraded EC2 from t4g.small to m8g.large (4 vCPU, 16 GB) for production stability"
  - "DNS-only mode in Cloudflare (gray cloud) required for Let's Encrypt HTTP-01 challenge"
  - "Domain field in Coolify should be without https:// prefix"

patterns-established:
  - "Coolify domain config: use bare domain (api.nomadcrew.uk) not https:// prefix"

issues-created: []

# Metrics
duration: 45min
completed: 2026-01-12
---

# Phase 17 Plan 01: Domain & SSL Configuration Summary

**Production HTTPS API deployed at https://api.nomadcrew.uk with Let's Encrypt certificate, upgraded to m8g.large Graviton4 instance**

## Performance

- **Duration:** ~45 min
- **Started:** 2026-01-12T08:00:00Z
- **Completed:** 2026-01-12T08:48:00Z
- **Tasks:** 3 (all checkpoints)
- **Files modified:** 2

## Accomplishments

- DNS A record configured: `api.nomadcrew.uk` → `3.130.209.141`
- Let's Encrypt SSL certificate provisioned via Coolify/Traefik
- HTTP to HTTPS redirect working
- All health checks passing via HTTPS
- EC2 instance upgraded from t4g.small (2GB) to m8g.large (16GB)

## Configuration Details

| Setting | Value |
|---------|-------|
| Domain | api.nomadcrew.uk |
| SSL Provider | Let's Encrypt (R12) |
| Certificate Expiry | Apr 12, 2026 |
| EC2 Instance | m8g.large (4 vCPU, 16 GB Graviton4) |
| Monthly Cost | ~$163/month |

## Verified Endpoints

| Endpoint | Status | Response |
|----------|--------|----------|
| `https://api.nomadcrew.uk/health` | 200 | Database UP, Redis UP |
| `https://api.nomadcrew.uk/health/liveness` | 200 | OK |
| `https://api.nomadcrew.uk/v1/trips` | 401 | Auth required (correct) |
| `https://api.nomadcrew.uk/v1/users` | 401 | Auth required (correct) |
| `http://...` → `https://...` | 302 | Redirect works |

**Note:** API routes are at `/v1/...` not `/api/v1/...`

## Decisions Made

1. **Instance Upgrade:** t4g.small was running out of memory with Coolify + Traefik + app. Upgraded to m8g.large (Graviton4) for production stability (~$163/month vs ~$14/month)
2. **DNS Mode:** Used Cloudflare DNS-only (gray cloud) instead of proxied to allow Let's Encrypt HTTP-01 challenge
3. **Domain Format:** Coolify requires bare domain (`api.nomadcrew.uk`) without `https://` prefix

## Issues Encountered

1. **Instance Unresponsive:** Initial t4g.small instance became unresponsive (Coolify dashboard timeout). Required reboot and upgrade to m8g.large.
2. **Traefik 404:** After adding domain, Traefik returned 404. Fixed by removing `https://` prefix from domain field and redeploying.
3. **SSL Timeout:** Initial SSL connection timed out - ports 80/443 were already open but Traefik wasn't routing. Fixed after instance upgrade and redeploy.

## Files Modified

- `infrastructure/aws/main.tf` - Updated cost estimates for m8g.large
- `infrastructure/aws/terraform.tfvars` - Changed instance_type to m8g.large

## Next Phase Readiness

Ready for Phase 18: Monitoring Setup
- HTTPS endpoint available for health checks
- Stable m8g.large instance with headroom for monitoring agents
- Grafana Cloud free tier can scrape `/health` endpoints

---
*Phase: 17-domain-ssl-config*
*Completed: 2026-01-12*
