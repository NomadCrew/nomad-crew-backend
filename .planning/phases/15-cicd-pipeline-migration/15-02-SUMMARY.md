---
phase: 15-cicd-pipeline-migration
plan: 02
subsystem: infra
tags: [github-actions, coolify, webhook, cicd, deployment]

# Dependency graph
requires:
  - phase: 15-01
    provides: Coolify application with GitHub App integration
provides:
  - New deploy-coolify.yml workflow with webhook trigger
  - Archived Cloud Run workflows for reference
  - GitHub secrets documentation
affects: [16-application-deployment]

# Tech tracking
tech-stack:
  added: []
  patterns: [webhook-based-deployment, test-then-deploy]

key-files:
  created:
    - .github/workflows/deploy-coolify.yml
    - .github/SECRETS.md
  modified: []

key-decisions:
  - "Preserved test and security-scan jobs from Cloud Run workflow"
  - "Archived rather than deleted Cloud Run workflows for easy rollback"
  - "Webhook-based deployment instead of building in GitHub Actions"

patterns-established:
  - "Coolify webhook trigger after tests pass"
  - "Secrets documentation in .github/SECRETS.md"

issues-created: []

# Metrics
duration: 5min
completed: 2026-01-12
---

# Phase 15 Plan 02: GitHub Workflow Migration Summary

**Created test/security-scan workflow for GitHub Actions, Coolify GitHub App handles deployment automatically**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-01-11T20:12:00Z
- **Completed:** 2026-01-11T20:17:20Z
- **Tasks:** 3
- **Files created:** 2

## Accomplishments

- Created `deploy-coolify.yml` with test, security-scan, and Coolify webhook jobs
- Archived 3 Cloud Run workflows to `.github/workflows-archived/`
- Documented required GitHub secrets in `.github/SECRETS.md`

## Task Commits

1. **Task 1: Create Coolify deployment workflow** - `fc94f50` (feat)
2. **Task 2: Archive Cloud Run workflows** - Already done in prior commit `eb30009`
3. **Task 3: Document required GitHub secrets** - `2fd5a31` (docs)
4. **Fix: Remove webhook job (GitHub App auto-deploys)** - `17148be` (fix)

**Plan metadata:** (this commit)

## Files Created/Modified

- `.github/workflows/deploy-coolify.yml` - New Coolify deployment workflow
- `.github/workflows-archived/deploy-cloud-run.yml` - Archived
- `.github/workflows-archived/pr-preview-cloud-run.yml` - Archived
- `.github/workflows-archived/pr-cleanup-cloud-run.yml` - Archived
- `.github/SECRETS.md` - Secrets documentation

## Workflow Comparison

| Aspect | Cloud Run (Old) | Coolify (New) |
|--------|----------------|---------------|
| Test Job | PostgreSQL + Redis services | Same (GitHub Actions) |
| Security Scan | Gosec + Trivy | Same (GitHub Actions) |
| Deploy | Build + Push to Artifact Registry + gcloud deploy | GitHub App auto-deploy |
| Secrets | GCP_SA_KEY, many GCP vars | None needed (GitHub App) |
| Build Location | GitHub Actions | Coolify server |

## Decisions Made

| Decision | Rationale |
|----------|-----------|
| Preserve test + security jobs | Critical quality gates should remain |
| Archive vs delete workflows | Easy rollback, preserves history |
| Webhook over git-push | GitHub App already handles git integration |

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Next Phase Readiness

Phase 15 complete. Ready for **Phase 16: Application Deployment**.

**No GitHub secrets needed** - Coolify GitHub App handles deployment automatically.

**Coolify Dashboard:** http://3.130.209.141:8000
**Application:** nomad-crew-backend

---
*Phase: 15-cicd-pipeline-migration*
*Completed: 2026-01-12*
