# Project Milestones: NomadCrew Backend

## v1.1 Infrastructure Migration (Shipped: 2026-01-12)

**Delivered:** Migrated from Google Cloud Run to AWS EC2 with Coolify, establishing production infrastructure at https://api.nomadcrew.uk

**Phases completed:** 13-19 (8 plans total)

**Key accomplishments:**
- AWS EC2 m8g.large (ARM Graviton4) deployed with Coolify PaaS
- Let's Encrypt SSL with auto-renewal for api.nomadcrew.uk
- GitHub Actions CI/CD with Coolify GitHub App deployment
- Grafana Cloud synthetic monitoring with Discord alerts
- GCP Cloud Run decommissioned, secrets cleaned up

**Stats:**
- 7 phases, 8 plans
- 2 days from start to ship
- Infrastructure fully migrated

**Git range:** Phase 13 start → Phase 19 complete

**What's next:** v1.2 Developer Experience (Windows mobile dev workflow)

---

## v1.0 Codebase Refactoring (Shipped: 2026-01-12)

**Delivered:** Domain-by-domain refactoring of the Go backend API, reducing complexity and removing technical debt while maintaining all existing functionality.

**Phases completed:** 1-12 (16 plans total)

**Key accomplishments:**
- Implemented proper admin role check (was hardcoded false - security fix)
- Established consistent error handling with c.Error() pattern
- Removed 660+ lines of deprecated code
- Standardized all handlers with getUserIDFromContext pattern
- Cleaned up notification architecture (NotificationService vs NotificationFacadeService)
- Verified permission architecture is secure (Phase 9 weather analysis)

**Stats:**
- 59 files changed
- 6,033 insertions, 1,202 deletions
- 57,583 total lines of Go code
- 3 days from start to ship

**Git range:** `353fcb3` (first Phase 6 commit) → `072da9b` (Phase 12 complete)

**What's next:** v1.1 Infrastructure Migration

---

*For detailed phase information, see [milestones/](milestones/) directory*
