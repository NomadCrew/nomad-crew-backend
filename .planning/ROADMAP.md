# Roadmap: NomadCrew Backend Refactoring

## Current Milestone: v1.0 — Codebase Refactoring

### Overview

Domain-by-domain refactoring of the NomadCrew backend API to reduce complexity, remove duplication, and improve architecture while maintaining existing functionality.

**Approach:** Refactor each domain completely before moving to the next
**Constraints:** No API changes, no database changes, tests must pass

---

## Phase 1: Trip Domain Handler Refactoring
**Status:** Complete (2026-01-10)
**Research Required:** No

**Goal:** Reduce complexity in trip_handler.go, extract helper functions, improve error handling

**Scope:**
- `handlers/trip_handler.go` - Main trip HTTP handlers
- `handlers/trip_handler_test.go` - Trip handler tests

**Key Tasks:**
- Extract repeated validation logic into helper functions
- Standardize error response formatting
- Break large handler methods into smaller focused functions
- Ensure consistent context key usage
- Update tests to match refactored code

**Success Criteria:**
- All existing tests pass
- No individual handler method exceeds 50 lines
- Consistent error handling pattern across all trip endpoints

**Dependencies:** None

---

## Phase 2: Trip Domain Service/Model Refactoring
**Status:** Complete (2026-01-10)
**Research Required:** No

**Goal:** Simplify trip service layer, remove duplication, improve separation of concerns

**Scope:**
- `models/trip/model.go` - Trip aggregate root
- `models/trip/service/trip_model_coordinator.go` - Trip business logic
- `models/trip/service/trip_member_service.go` - Membership operations
- `models/trip/command/*.go` - CQRS commands
- `models/trip/validation/*.go` - Validation logic

**Key Tasks:**
- Review model vs coordinator responsibilities
- Remove any duplicate business logic
- Ensure clear separation between commands and queries
- Standardize validation error messages
- Fix membership status validation TODO (line 39)

**Success Criteria:**
- All existing tests pass
- Clear single responsibility for each service method
- No duplicate business logic between model and coordinator

**Dependencies:** Phase 1 (handler refactoring may reveal service issues)

---

## Phase 3: Trip Domain Store Refactoring
**Status:** Complete (2026-01-10)
**Research Required:** No

**Goal:** Clean up trip store adapter, ensure consistent query patterns

**Scope:**
- `internal/store/sqlcadapter/trip_store.go` - Trip store implementation
- `internal/store/interfaces.go` - Store interface definitions (trip section)
- `store/postgres/trip_store_pg_mock_test.go` - Store tests

**Key Tasks:**
- Review store interface for unnecessary methods
- Ensure consistent error wrapping
- Verify all SQLC queries are properly used
- Remove any manual query building (use SQLC)

**Success Criteria:**
- All existing tests pass
- Consistent error handling in all store methods
- Interface matches actual usage needs

**Dependencies:** Phase 2

---

## Phase 4: User Domain Refactoring
**Status:** Complete (2026-01-11)
**Research Required:** Yes (admin role implementation options) - DONE

**Goal:** Fix admin check implementation, add missing permission checks, reduce handler complexity

**Scope:**
- `handlers/user_handler.go` - User HTTP handlers (260, 343, 620 line issues)
- `models/user/service/user_service.go` - User business logic
- `internal/store/sqlcadapter/user_store.go` - User store implementation
- `services/supabase_service.go` - Supabase integration

**Key Tasks:**
- ✅ **CRITICAL:** Implement proper admin role check (currently hardcoded false at lines 260, 343)
- ✅ **CRITICAL:** Add trip membership check at line 620 (or document why not needed) - Removed unused parameter
- Extract repeated validation patterns
- Standardize error responses
- Review Supabase service integration

**Completed Plans:**
- Plan 4-01: Admin role implementation (JWT app_metadata extraction)
- Plan 4-02: Handler cleanup (removed unused tripId parameter)

**Success Criteria:**
- ✅ Admin role check properly implemented
- ✅ Trip membership verification in place (removed unused parameter, use trip member endpoints)
- All existing tests pass
- ✅ Security issues resolved

**Dependencies:** Phase 3 (may need TripStore for membership check)

---

## Phase 5: Location Domain Refactoring
**Status:** Complete (2026-01-11)
**Research Required:** No

**Goal:** Simplify location tracking handlers and service, ensure proper authorization

**Scope:**
- `handlers/location_handler.go` - Location HTTP handlers
- `internal/handlers/location_ws_handler.go` - WebSocket location handler
- `models/location/service/location_management_service.go` - Location service
- `internal/store/sqlcadapter/location_store.go` - Location store

**Key Tasks:**
- Review location update authorization (trip membership)
- Standardize HTTP and WebSocket handler patterns
- Remove any duplicate logic between handlers
- Ensure consistent error handling

**Success Criteria:**
- All existing tests pass
- Consistent authorization checks
- Clear separation between HTTP and WebSocket concerns

**Dependencies:** Phase 4 (user domain provides auth patterns)

---

## Phase 6: Notification Domain Refactoring
**Status:** Complete (2026-01-12)
**Research Required:** No

**Goal:** Clean up notification handlers and service, improve push notification patterns

**Completed Plans:**
- Plan 6-01: Interface consolidation - Removed duplicate interfaces, renamed NotificationFacadeService
- Plan 6-02: Handler pattern standardization - Refactored all 5 handler methods

**Scope:**
- `handlers/notification_handler.go` - Notification HTTP handlers (REFACTORED)
- `models/notification/notification_service.go` - Notification business logic
- `services/push_service.go` - Push notification service (already has proper batching)
- `internal/notification/client.go` - Notification facade client

**Outcome:**
- All handler methods use established patterns (getUserIDFromContext, c.Error())
- Batch notification documented as future enhancement (Expo service already has batching)
- Clear separation: NotificationService (database) vs NotificationFacadeService (external API)

**Success Criteria:** All met
- All existing tests pass
- Consistent notification delivery patterns with c.Error()
- Clear error handling for failed deliveries

**Dependencies:** Phase 5

---

## Phase 7: Todo Domain Refactoring
**Status:** Complete (2026-01-12)
**Research Required:** No

**Goal:** Simplify todo handler and model, ensure proper trip authorization

**Scope:**
- `handlers/todo_handler.go` - Todo HTTP handlers
- `models/model.go` - Todo model (legacy location)
- `internal/store/sqlcadapter/todo_store.go` - Todo store

**Key Tasks:**
- Review todo authorization (trip membership)
- Consider moving todo model to proper domain directory
- Standardize CRUD patterns
- Ensure consistent error responses

**Success Criteria:**
- All existing tests pass
- Todo model in appropriate location
- Consistent authorization pattern

**Dependencies:** Phase 6

---

## Phase 8: Chat Domain Refactoring
**Status:** Complete (2026-01-12)
**Research Required:** No

**Goal:** Clean up Supabase chat integration, ensure proper patterns

**Completed Plans:**
- Plan 8-01: Handler Consolidation and Pattern Standardization

**Scope:**
- `handlers/chat_handler_supabase.go` - Chat HTTP handlers (REFACTORED)
- `handlers/chat_handler.go` - Legacy chat handler (DEPRECATED)
- `models/chat/service/chat_service.go` - Chat service (UNUSED)

**Outcome:**
- ChatHandlerSupabase standardized with established patterns
- Notification support added (NotificationFacadeService)
- ChatHandler deprecated with TODO for Phase 12 removal
- GetTripMembers added to TripServiceInterface

**Success Criteria:** All met
- All existing tests pass
- Clean Supabase integration with consistent patterns
- ChatHandler deprecated, ChatHandlerSupabase is the active handler

**Dependencies:** Phase 7

---

## Phase 9: Weather Service Refactoring
**Status:** Not Started
**Research Required:** No

**Goal:** Add missing permission checks, clean up weather service

**Scope:**
- `models/weather/service/weather_service.go` - Weather service (TODO at lines 61, 85)

**Key Tasks:**
- **CRITICAL:** Add trip membership verification before weather operations
- Review weather data caching patterns
- Standardize error handling

**Success Criteria:**
- Permission checks implemented
- All existing tests pass
- Security issues resolved

**Dependencies:** Phase 8

---

## Phase 10: Middleware and Cross-Cutting Concerns
**Status:** Not Started
**Research Required:** No

**Goal:** Standardize middleware patterns, improve RBAC consistency

**Scope:**
- `middleware/auth.go` - Authentication middleware
- `middleware/rbac.go` - Authorization middleware
- `middleware/rate_limit.go` - Rate limiting
- `middleware/jwt_validator.go` - JWT validation
- `types/permission_matrix.go` - Permission definitions

**Key Tasks:**
- Review permission matrix completeness
- Ensure consistent middleware error responses
- Standardize context key usage across all middleware
- Review rate limiting configuration

**Success Criteria:**
- All existing tests pass
- Consistent middleware patterns
- Complete permission matrix coverage

**Dependencies:** Phases 1-9 (may discover middleware issues during domain refactoring)

---

## Phase 11: Event System and WebSocket Refactoring
**Status:** Not Started
**Research Required:** No

**Goal:** Clean up event publishing patterns, improve WebSocket hub

**Scope:**
- `internal/events/service.go` - Event service
- `internal/events/redis_publisher.go` - Redis pub/sub
- `internal/websocket/hub.go` - WebSocket connection manager
- `internal/websocket/handler.go` - WebSocket endpoint

**Key Tasks:**
- Review event type consistency
- Standardize event publishing patterns across all domains
- Ensure proper WebSocket cleanup on disconnect
- Review Redis pub/sub error handling

**Success Criteria:**
- All existing tests pass
- Consistent event publishing across domains
- Proper connection lifecycle management

**Dependencies:** Phase 10

---

## Phase 12: Final Cleanup and Documentation
**Status:** Not Started
**Research Required:** No

**Goal:** Remove remaining TODO/FIXME comments, ensure consistency across codebase

**Scope:**
- All files with remaining TODO/FIXME comments
- `logger/logger.go` - CloudWatch TODO at line 47
- Integration tests in `tests/integration/`

**Key Tasks:**
- Address or document remaining TODO comments
- Ensure consistent error types usage
- Review integration test coverage
- Verify all refactoring goals met
- Final test run and validation

**Success Criteria:**
- All tests pass
- No critical TODO/FIXME remaining
- Codebase meets refactoring goals from PROJECT.md

**Dependencies:** Phases 1-11

---

## Summary

| Phase | Domain | Critical Issues | Research |
|-------|--------|-----------------|----------|
| 1 | Trip Handlers | None | No |
| 2 | Trip Service/Model | Membership validation | No |
| 3 | Trip Store | None | No |
| 4 | User Domain | Admin check, membership check | Yes |
| 5 | Location Domain | None | No |
| 6 | Notification Domain | Batch notifications | No |
| 7 | Todo Domain | None | No |
| 8 | Chat Domain | None | No |
| 9 | Weather Service | Permission checks | No |
| 10 | Middleware | None | No |
| 11 | Events/WebSocket | None | No |
| 12 | Final Cleanup | TODO comments | No |

**Total Phases:** 12
**Phases with Critical Security Issues:** 2 (Phase 4, Phase 9)
**Phases Requiring Research:** 1 (Phase 4)

---

## Milestone: v1.1 — Infrastructure Migration to AWS

### Overview

Migrate from Google Cloud Run ($24/month) to AWS EC2 with Coolify for cost-effective, reliable infrastructure.

**Target Stack:**
- **Compute**: AWS EC2 t4g.small (2 vCPU, 2 GB ARM Graviton) — ~$14/month
- **Orchestration**: Coolify (self-hosted PaaS)
- **Database**: Neon PostgreSQL (keep existing)
- **Cache**: Upstash Redis (keep existing)
- **Monitoring**: Grafana Cloud Free Tier

**Cost Savings:** $24/month → ~$14/month

**Note:** Originally planned for Oracle Cloud Always Free, but switched to AWS due to persistent ARM capacity issues in OCI regions.

---

## Phase 13: AWS EC2 Setup
**Status:** Complete (2026-01-11)
**Research Required:** No (switched from OCI to AWS)

**Goal:** Provision AWS EC2 ARM Graviton instance for backend deployment

**Scope:**
- Create VPC with public subnet
- Configure security group (ports 22, 80, 443)
- Provision t4g.small ARM Graviton instance
- Assign Elastic IP for stable addressing
- Set up SSH access

**Outcome:**
- AWS EC2 instance running at 3.130.209.141
- t4g.small (2 vCPU, 2 GB ARM Graviton)
- Ubuntu 22.04 ARM64
- Estimated cost: ~$14/month

**Note:** Originally attempted Oracle Cloud but switched to AWS due to "Out of host capacity" errors after 60+ retry attempts.

**Dependencies:** None

---

## Phase 14: Coolify Installation
**Status:** Complete (2026-01-11)
**Research Required:** Yes (Coolify setup on ARM, Docker configuration) - DONE

**Goal:** Install and configure Coolify on AWS EC2 for Heroku-like deployment experience

**Scope:**
- Install Docker on ARM instance
- Install Coolify using official install script
- Configure Coolify admin account
- Set up Coolify networking and reverse proxy
- Test basic deployment capability

**Research Topics:**
- Coolify ARM64 compatibility
- Docker installation on Ubuntu ARM
- Coolify resource requirements (2 GB RAM may be tight)

**Success Criteria:**
- Coolify dashboard accessible
- Can create projects and environments
- Reverse proxy (Traefik) working

**Outcome:**
- Coolify v4.0.0-beta.460 installed and running
- Docker 27.0.3 installed automatically
- Admin account: admin@nomadcrew.uk
- Dashboard: http://3.130.209.141:8000
- All containers healthy (coolify, realtime, redis, db)

**Dependencies:** Phase 13 (AWS EC2 instance at 3.130.209.141)

---

## Phase 15: CI/CD Pipeline Migration
**Status:** Complete (2026-01-12)
**Research Required:** No

**Goal:** Update GitHub Actions workflows for Coolify git-push deployments

**Completed Plans:**
- Plan 15-01: Coolify application setup with GitHub App integration
- Plan 15-02: GitHub workflow migration (deploy-coolify.yml, archive Cloud Run)

**Outcome:**
- `deploy-coolify.yml` created with test, security-scan, and webhook deploy jobs
- Cloud Run workflows archived to `.github/workflows-archived/`
- GitHub secrets documented in `.github/SECRETS.md`
- No GCP references in active workflows

**Success Criteria:** ✅ All met
- Push to main triggers deployment to Coolify
- Environment variables to be configured in Phase 16
- Deployment logs accessible via Coolify dashboard

**Dependencies:** Phase 14

---

## Phase 16: Application Deployment
**Status:** Not Started
**Research Required:** No

**Goal:** Deploy NomadCrew Go backend via Coolify with existing Dockerfile

**Scope:**
- Configure application in Coolify
- Set up environment variables (Neon, Upstash, Supabase, etc.)
- Configure health checks
- Test application functionality
- Verify database and cache connectivity

**Key Tasks:**
- Create Coolify application from GitHub repo
- Configure build settings (Dockerfile)
- Set all required environment variables
- Test all API endpoints
- Verify external service integrations

**Success Criteria:**
- Application running on Coolify
- All health checks passing
- Database and cache connections working
- API endpoints responding correctly

**Dependencies:** Phase 15

---

## Phase 17: Domain & SSL Configuration
**Status:** Not Started
**Research Required:** No

**Goal:** Configure custom domain and SSL certificates for production

**Scope:**
- Update DNS records for nomadcrew.uk
- Configure domain in Coolify
- Set up Let's Encrypt SSL certificates
- Configure HTTP to HTTPS redirect
- Update CORS and allowed origins

**Key Tasks:**
- Add A record pointing to Oracle Cloud IP
- Configure domain in Coolify dashboard
- Enable automatic SSL via Let's Encrypt
- Test HTTPS access
- Update frontend configuration if needed

**Success Criteria:**
- nomadcrew.uk resolving to Oracle Cloud
- Valid SSL certificate installed
- HTTPS working with auto-redirect
- No mixed content warnings

**Dependencies:** Phase 16

---

## Phase 18: Monitoring Setup
**Status:** Not Started
**Research Required:** Yes (Grafana Cloud setup, Prometheus integration)

**Goal:** Set up free monitoring and observability with Grafana Cloud

**Scope:**
- Create Grafana Cloud free account
- Configure Prometheus metrics export
- Set up log aggregation
- Create dashboards for key metrics
- Configure alerting for critical issues

**Research Topics:**
- Grafana Cloud free tier limits
- Prometheus Go client integration
- Loki log shipping configuration

**Key Tasks:**
- Sign up for Grafana Cloud free tier
- Install Grafana Agent on Oracle Cloud
- Configure application metrics endpoint
- Create performance dashboards
- Set up alert rules

**Success Criteria:**
- Metrics visible in Grafana Cloud
- Logs searchable
- Alerts configured for downtime
- Dashboard showing key health indicators

**Dependencies:** Phase 17

---

## Phase 19: Cloud Run Decommissioning
**Status:** Not Started
**Research Required:** No

**Goal:** Safely shut down GCP Cloud Run and clean up resources

**Scope:**
- Verify Oracle Cloud deployment is stable (run for 48+ hours)
- Update any remaining DNS records
- Delete Cloud Run service
- Remove Artifact Registry images
- Clean up GCP IAM and service accounts
- Cancel/downgrade GCP billing

**Key Tasks:**
- Final verification of Oracle Cloud deployment
- Document any GCP resources to keep (if any)
- Delete Cloud Run service
- Clean up container images
- Update GitHub secrets to remove GCP credentials
- Archive old deployment workflows

**Success Criteria:**
- Cloud Run service deleted
- No unexpected GCP charges
- All traffic flowing through Oracle Cloud
- GitHub workflows updated

**Dependencies:** Phase 18 (monitoring confirms stability)

---

## v1.1 Summary

| Phase | Name | Status | Research | Critical |
|-------|------|--------|----------|----------|
| 13 | AWS EC2 Setup | Complete | No | No |
| 14 | Coolify Installation | Complete | Yes | No |
| 15 | CI/CD Pipeline Migration | Complete | No | No |
| 16 | Application Deployment | Not Started | No | Yes |
| 17 | Domain & SSL Config | Not Started | No | Yes |
| 18 | Monitoring Setup | Not Started | Yes | No |
| 19 | Cloud Run Decommissioning | Not Started | No | No |

**Total Phases:** 7
**Phases Complete:** 3 (Phase 13, 14, 15)
**Phases Requiring Research:** 1 (Phase 18)
**Critical Phases:** 2 (Phase 16, 17 - production traffic)

---

*Created: 2026-01-10*
*Approach: Domain-by-domain*
*Depth: Comprehensive*
