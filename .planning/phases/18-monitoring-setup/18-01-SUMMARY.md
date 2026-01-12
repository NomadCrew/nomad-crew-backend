---
phase: 18-monitoring-setup
plan: 01
subsystem: infra
tags: [grafana, monitoring, synthetic-checks, alerting, discord]

# Dependency graph
requires:
  - phase: 17-domain-ssl-config
    provides: Production URL (https://api.nomadcrew.uk) with SSL
provides:
  - Grafana Cloud monitoring account
  - Synthetic uptime checks for API endpoints
  - Discord alerting for downtime detection
affects: [19-cloud-run-decommissioning]

# Tech tracking
tech-stack:
  added: [grafana-cloud, discord-webhooks]
  patterns: [synthetic-monitoring, webhook-alerting]

key-files:
  created: []
  modified: []

key-decisions:
  - "Used Discord webhooks instead of email for alerting (more reliable)"
  - "Synthetic monitoring approach (no agent required on server)"
  - "3 probe locations for geographic coverage"

patterns-established:
  - "Synthetic monitoring: Cloud-based HTTP probes, no infrastructure changes needed"
  - "Alert routing: Discord webhook for team notifications"

issues-created: []

# Metrics
duration: 45min
completed: 2026-01-12
---

# Phase 18 Plan 01: Monitoring Setup Summary

**Grafana Cloud synthetic monitoring configured with 3 uptime checks and Discord alerting**

## Performance

- **Duration:** 45 min
- **Started:** 2026-01-12T09:01:00Z
- **Completed:** 2026-01-12T09:46:41Z
- **Tasks:** 3 (all checkpoint:human-action)
- **Files modified:** 0 (external service configuration only)

## Accomplishments

- Grafana Cloud free account created (nomadcrew5.grafana.net)
- 3 synthetic monitoring checks configured and passing
- Discord alerting configured for downtime notifications
- Cloudflare email routing set up for alerts@nomadcrew.uk (backup)

## Configuration Details

| Setting | Value |
|---------|-------|
| Grafana Cloud URL | nomadcrew5.grafana.net |
| Checks Created | 3 |
| Check Frequency | 1-5 min |
| Probe Locations | 3 (global coverage) |
| Alert Threshold | 5 min downtime |
| Alert Channel | Discord webhook |

## Synthetic Checks

| Check Name | URL | Frequency | Expected Status |
|------------|-----|-----------|-----------------|
| NomadCrew API Health | https://api.nomadcrew.uk/health | 1 min | 200 |
| NomadCrew Liveness | https://api.nomadcrew.uk/health/liveness | 1 min | 200 |
| NomadCrew API v1 | https://api.nomadcrew.uk/v1/trips | 5 min | 401 |

## Decisions Made

1. **Discord over Email:** Switched from email alerts to Discord webhooks after Grafana email delivery issues. Discord is more reliable and provides richer formatting for alerts.

2. **Synthetic Monitoring First:** Chose synthetic monitoring over full observability (Grafana Alloy agent) as it provides immediate value with zero infrastructure changes.

3. **401 as Success:** The `/v1/trips` check expects 401 (unauthorized) which confirms the API routing is working correctly without requiring authentication.

## Deviations from Plan

### Adjusted Approach

**1. [Rule 3 - Blocking] Switched from email to Discord alerting**
- **Found during:** Task 3 (Configure alerting)
- **Issue:** Grafana test emails not arriving at alerts@nomadcrew.uk despite Cloudflare email routing working for manual emails
- **Fix:** Configured Discord webhook as primary alert channel
- **Verification:** Test notification appeared in Discord #alerts channel

## Issues Encountered

- Grafana Cloud email delivery to Cloudflare-routed addresses was unreliable. Manual emails through Cloudflare routing worked, but Grafana's test notifications did not arrive. Switched to Discord which works reliably.

## Next Phase Readiness

Ready for Phase 19: Cloud Run Decommissioning
- Monitoring in place to detect issues during migration cutover
- 48+ hour stability window can begin
- Discord alerts will notify team if API goes down during decommissioning

---
*Phase: 18-monitoring-setup*
*Completed: 2026-01-12*
