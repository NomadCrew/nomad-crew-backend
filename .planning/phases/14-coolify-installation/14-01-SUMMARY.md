---
phase: 14-coolify-installation
plan: 01
subsystem: infra
tags: [coolify, docker, paas, arm64, traefik]

# Dependency graph
requires:
  - phase: 13-oracle-cloud-setup
    provides: EC2 instance at 3.130.209.141
provides:
  - Running Coolify v4.0.0-beta.460 instance for git-push deployments
  - Docker 27.0.3 and Docker Compose installed
  - Traefik reverse proxy configured
  - PostgreSQL 15 and Redis 7 for Coolify internal use
affects: [15-cicd-pipeline, 16-application-deployment, 17-domain-ssl-config]

# Tech tracking
tech-stack:
  added: [coolify, docker, traefik, postgres, redis]
  patterns: [self-hosted-paas, container-orchestration]

key-files:
  created: []
  modified:
    - infrastructure/aws/main.tf

key-decisions:
  - "Admin account: admin@nomadcrew.uk"
  - "Port 8000 opened for initial setup (Traefik will handle 80/443 after domain config)"

patterns-established:
  - "Coolify for application deployment and management"
  - "Docker-based container orchestration"

issues-created: []

# Metrics
duration: 8min
completed: 2026-01-11
---

# Phase 14 Plan 01: Coolify Installation Summary

**Coolify v4.0.0-beta.460 installed on AWS EC2 ARM Graviton with Docker, Traefik, and admin account configured at admin@nomadcrew.uk**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-01-11T05:55:00Z
- **Completed:** 2026-01-11T06:03:00Z
- **Tasks:** 3 (2 auto + 1 checkpoint)
- **Files modified:** 1

## Accomplishments

- Coolify v4.0.0-beta.460 installed and running on ARM64 EC2 instance
- Docker 27.0.3 installed automatically by Coolify installer
- All containers healthy: coolify, coolify-realtime, coolify-redis, coolify-db
- AWS security group updated with port 8000 for dashboard access
- Admin account created (admin@nomadcrew.uk)
- Server validated and connected in Coolify dashboard

## Task Commits

1. **Tasks 1-2: Install Coolify + Configure security group** - `b7768ed` (feat)

**Plan metadata:** (this commit)

## Files Created/Modified

- `infrastructure/aws/main.tf` - Added port 8000 ingress rule for Coolify dashboard

## Decisions Made

| Decision | Rationale |
|----------|-----------|
| Admin email: admin@nomadcrew.uk | Consistent with domain for the project |
| Port 8000 opened to 0.0.0.0/0 | Required for initial setup; Traefik will handle 80/443 after domain config |
| Used official Coolify installer | Handles Docker installation with compatible versions automatically |

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- **Disk space warning:** Installer warned about 29GB vs 30GB required, but proceeded successfully
- **Resolution:** Warning only; Coolify installed and runs fine on 29GB

## Server Details

| Component | Version/Details |
|-----------|-----------------|
| Coolify | v4.0.0-beta.460 |
| Docker | 27.0.3 |
| Coolify Helper | 1.0.12 |
| Coolify Realtime | 1.0.10 |
| PostgreSQL | 15-alpine |
| Redis | 7-alpine |

## Coolify Containers

```
coolify          - Main dashboard (healthy)
coolify-realtime - WebSocket server (healthy)
coolify-redis    - Session/cache store (healthy)
coolify-db       - PostgreSQL database (healthy)
```

## Next Phase Readiness

Ready for **Phase 15: CI/CD Pipeline Migration**

**Coolify Dashboard:** http://3.130.209.141:8000
**Admin Account:** admin@nomadcrew.uk

**Prerequisites for Phase 15:**
- [x] Coolify installed and accessible
- [x] Admin account created
- [x] Server connected in Coolify dashboard
- [ ] GitHub integration to configure
- [ ] Webhook or git-push deployment to set up

---
*Phase: 14-coolify-installation*
*Completed: 2026-01-11*
