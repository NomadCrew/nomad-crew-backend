---
phase: 19-cloud-run-decommissioning
plan: 01
subsystem: infra
tags: [gcp, cloud-run, artifact-registry, decommissioning, cleanup]

# Dependency graph
requires:
  - phase: 18-monitoring-setup
    provides: Grafana Cloud monitoring to verify AWS stability before decommissioning
provides:
  - GCP Cloud Run decommissioned
  - Artifact Registry cleaned up
  - GitHub secrets removed
  - Zero GCP infrastructure costs
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns: []

key-files:
  created: []
  modified: []

key-decisions:
  - "Cloud Run and Artifact Registry were already deleted/never deployed - APIs disabled"
  - "Network usage from Firebase is minimal (~$0-2/month) - kept for mobile app"

patterns-established: []

issues-created: []

# Metrics
duration: 15min
completed: 2026-01-12
---

# Phase 19 Plan 01: Cloud Run Decommissioning Summary

**GCP Cloud Run infrastructure verified as already decommissioned; GitHub secrets cleaned up, AWS production confirmed stable**

## Performance

- **Duration:** 15 min
- **Started:** 2026-01-12T10:13:00Z
- **Completed:** 2026-01-12T10:28:00Z
- **Tasks:** 3 (all human-action checkpoints)
- **Files modified:** 0 (infrastructure cleanup only)

## Accomplishments

- Verified AWS production API is healthy and stable (1h35m+ uptime)
- Confirmed Cloud Run service (`nomadcrew-backend`) already deleted/never deployed
- Confirmed Artifact Registry (`nomadcrew-containers`) already deleted/API disabled
- Removed `GCP_SA_KEY` secret from GitHub repository
- Verified no Cloud Storage buckets or Compute Engine instances remain

## Resource Status

| Resource | Type | Status | Project |
|----------|------|--------|---------|
| nomadcrew-backend | Cloud Run | Already deleted (API disabled) | nomadcrew-11fd4 |
| nomadcrew-containers | Artifact Registry | Already deleted (API disabled) | nomadcrew-11fd4 |
| GCP_SA_KEY | GitHub Secret | Deleted | NomadCrew/nomad-crew-backend |
| Cloud Storage | Buckets | None found | nomadcrew-11fd4 |
| Compute Engine | Instances | None found | nomadcrew-11fd4 |

## Cost Impact

- **GCP Cloud Run:** $0/month (already gone)
- **GCP Network (Firebase):** ~$0-2/month (minimal, kept for mobile app)
- **AWS EC2 (m8g.large):** ~$163/month
- **Net result:** No Cloud Run charges, AWS is primary infrastructure

## Verification

- [x] AWS API: https://api.nomadcrew.uk - Healthy (database UP, redis UP)
- [x] Grafana checks: All 3 synthetic checks passing
- [x] Cloud Run: API disabled, no services exist
- [x] Artifact Registry: API disabled, no repositories exist
- [x] GitHub secrets: GCP_SA_KEY removed
- [x] Archived workflows: `.github/workflows-archived/` (3 files)

## Decisions Made

- Cloud Run and Artifact Registry resources were found to be already deleted or never successfully deployed (APIs showing as SERVICE_DISABLED)
- Firebase services kept enabled for mobile app functionality (FCM, etc.)
- Project `nomadcrew-11fd4` kept active for Firebase - not deleted entirely

## Deviations from Plan

None - discovery that resources were already gone simplified the process.

## Issues Encountered

None - the decommissioning was simpler than expected since GCP resources were already cleaned up.

## Next Steps

- **v1.1 Infrastructure Migration milestone COMPLETE**
- All 7 phases (13-19) finished
- Production running on AWS EC2 with Coolify
- Monitoring active via Grafana Cloud
- Ready for next milestone (v1.2 or feature work)

---
*Phase: 19-cloud-run-decommissioning*
*Completed: 2026-01-12*
