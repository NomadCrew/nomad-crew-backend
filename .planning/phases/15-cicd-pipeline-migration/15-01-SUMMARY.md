---
phase: 15-cicd-pipeline-migration
plan: 01
subsystem: infra
tags: [coolify, github-app, cicd, webhooks, auto-deploy]

# Dependency graph
requires:
  - phase: 14-coolify-installation
    provides: Running Coolify v4.0.0-beta.460 instance
provides:
  - Coolify application configured with GitHub App integration
  - Automatic deployments on push to main branch
  - EC2 instance renamed to "sftp" for identification
affects: [15-02-github-workflow, 16-application-deployment]

# Tech tracking
tech-stack:
  added: [github-app]
  patterns: [github-app-integration, auto-deploy-on-push]

key-files:
  created: []
  modified: []

key-decisions:
  - "Used GitHub App integration instead of manual webhooks - automatic webhook handling"
  - "EC2 instance renamed to 'sftp' for easier identification"
  - "Application named 'nomad-crew-backend' in Coolify"

patterns-established:
  - "GitHub App for repository access and webhooks"
  - "Auto-deploy enabled for push events"

issues-created: []

# Metrics
duration: 15min
completed: 2026-01-11
---

# Phase 15 Plan 01: Coolify Application Setup Summary

**Coolify application configured with GitHub App integration for NomadCrew/nomad-crew-backend, enabling automatic deployments on push to main**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-01-11T19:50:00Z
- **Completed:** 2026-01-11T20:05:13Z
- **Tasks:** 2 (checkpoints only - human actions)
- **Files modified:** 0 (configuration in Coolify/AWS UI)

## Accomplishments

- GitHub App created and installed on NomadCrew organization
- Coolify application "nomad-crew-backend" created with GitHub App source
- Repository connected: NomadCrew/nomad-crew-backend (main branch)
- Dockerfile build pack configured with port 8080
- Auto-deploy enabled (triggers on push via GitHub App)
- EC2 instance renamed to "sftp" in AWS Console

## Configuration Details

| Setting | Value |
|---------|-------|
| Application Name | nomad-crew-backend |
| GitHub Repo | NomadCrew/nomad-crew-backend |
| Branch | main |
| Build Pack | Dockerfile |
| Port | 8080 |
| Source Type | GitHub App (automatic webhooks) |
| Auto Deploy | Enabled |
| EC2 Instance Name | sftp |

## Task Commits

No code commits - this plan consisted of UI configuration in Coolify and AWS Console.

**Plan metadata:** (this commit)

## Files Created/Modified

None - all configuration was done via Coolify dashboard and AWS Console.

## Decisions Made

| Decision | Rationale |
|----------|-----------|
| GitHub App over manual webhooks | Automatic webhook handling, no secret management needed |
| EC2 instance named "sftp" | User preference for instance identification |
| Application named "nomad-crew-backend" | Matches repository name for clarity |

## Deviations from Plan

### Approach Change

**Original plan:** Manual webhook configuration with secret
**Actual approach:** GitHub App integration

- **Reason:** GitHub App is the recommended approach in Coolify v4 - handles webhooks automatically, more secure, simpler setup
- **Impact:** Simpler configuration, no manual webhook secret management needed

## Issues Encountered

None - GitHub App installation completed smoothly.

## Next Phase Readiness

Ready for **Phase 15 Plan 02: GitHub Workflow Migration** (if exists) or **Phase 16: Application Deployment**

**Current State:**
- [x] Coolify application created
- [x] GitHub App connected
- [x] Auto-deploy enabled
- [ ] Environment variables to configure (Phase 16)
- [ ] First deployment to trigger

**Coolify Dashboard:** http://3.130.209.141:8000
**Application:** nomad-crew-backend (waiting for first deployment)

---
*Phase: 15-cicd-pipeline-migration*
*Completed: 2026-01-11*
