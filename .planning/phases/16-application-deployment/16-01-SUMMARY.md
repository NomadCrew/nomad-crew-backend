---
phase: 16-application-deployment
plan: 01
subsystem: infra
tags: [coolify, docker, aws, ec2, deployment]

# Dependency graph
requires:
  - phase: 15-cicd-pipeline-migration
    provides: GitHub App integration, webhook deployment setup
provides:
  - Running Go backend on AWS EC2 via Coolify
  - Health check endpoints verified
  - External service connections (Neon, Upstash, Supabase)
affects: [17-domain-ssl-configuration, 18-monitoring-setup]

# Tech tracking
tech-stack:
  added: []
  patterns: [Coolify deployment, Docker multi-stage build, health check probes]

key-files:
  created: []
  modified:
    - infrastructure/aws/main.tf

key-decisions:
  - "Used port 8081:8080 mapping to avoid port 8080 conflict with Coolify"
  - "DNS only (gray cloud) in Cloudflare for Let's Encrypt compatibility"

patterns-established:
  - "Coolify GitHub App for auto-deploy on push to main"
  - "Health check on /health/liveness with 30s start period"

issues-created: []

# Metrics
duration: ~60min
completed: 2026-01-12
---

# Phase 16 Plan 01: Application Deployment Summary

**NomadCrew Go backend deployed to AWS EC2 via Coolify with all health checks passing and external services connected**

## Performance

- **Duration:** ~60 min (including EC2 recreation recovery)
- **Started:** 2026-01-12
- **Completed:** 2026-01-12
- **Tasks:** 3 (all checkpoints - human action/verify)

## Accomplishments

- Configured 20 environment variables in Coolify
- Deployed Go backend via Dockerfile build
- Health checks passing (liveness, readiness, full health)
- Database (Neon PostgreSQL) connected
- Cache (Upstash Redis) connected
- API endpoints responding correctly (401 for unauthenticated)

## Configuration Details

| Setting | Value |
|---------|-------|
| Application URL | http://3.130.209.141:8081 |
| Health Check Path | /health/liveness |
| Container Port | 8080 |
| Host Port | 8081 |
| Build Pack | Dockerfile |
| Start Period | 30s |

## Challenges Encountered

### EC2 Instance Recreation
During Terraform apply to add port 8081 to security group, the EC2 instance was destroyed and recreated due to resource naming changes. This required:
- Reinstalling Coolify from scratch
- Reconfiguring GitHub App integration
- Re-adding all environment variables
- Reconfiguring health checks

**Lesson learned:** Add `lifecycle { prevent_destroy = true }` to EC2 instance in Terraform to prevent accidental destruction.

### Port Conflict
Port 8080 was already in use on the host (by Coolify). Solution: Used port mapping 8081:8080.

## Decisions Made

1. **Port 8081 for direct access** - Avoids conflict with Coolify's internal services on 8080
2. **Coolify GitHub App** - Preferred over webhook for simpler integration and auto-deploy

## Deviations from Plan

### Infrastructure Changes
- Added port 8081 to AWS security group via Terraform
- EC2 instance was recreated (unplanned), requiring full Coolify reinstallation

**Impact:** Added ~30 min to execution time, but deployment ultimately successful.

## Issues Encountered

- EC2 recreation wiped Coolify installation - resolved by reinstalling
- Port 8080 conflict - resolved by using 8081
- Dockerfile not found error - resolved by selecting correct repo (nomad-crew-backend, not .github)

## Next Phase Readiness

Phase 16 complete. Ready for **Phase 17: Domain & SSL Configuration**.

**What's needed for Phase 17:**
- Configure Cloudflare DNS (A record: api → 3.130.209.141)
- Set domain in Coolify to `https://api.nomadcrew.uk`
- Let's Encrypt certificate auto-generation via Traefik

**Application URL:** http://3.130.209.141:8081
**Coolify Dashboard:** http://3.130.209.141:8000

---
*Phase: 16-application-deployment*
*Completed: 2026-01-12*
