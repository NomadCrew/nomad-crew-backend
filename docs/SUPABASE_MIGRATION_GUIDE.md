# WebSocket to Supabase Realtime Migration Guide

## Overview

This document guides you through the migration from our custom WebSocket implementation to Supabase Realtime. This migration will eliminate approximately 5,000 lines of complex WebSocket code and replace it with simple REST endpoints that leverage Supabase's real-time infrastructure.

## Architecture Changes

### Previous Architecture
```
Mobile App → WebSocket → Go Backend → Redis/PostgreSQL
                ↓
          Complex Handler
                ↓
        Event Broadcasting
```

### New Architecture
```
Mobile App → REST API → Go Backend → Supabase
     ↓                                    ↑
     └──── Realtime Subscription ────────┘
```

## Migration Timeline & Phases

### Phase 1: Database Setup (Day 1)
- Run migration scripts in Supabase (already implemented in `000002_supabase_realtime.up.sql`)
- Enable realtime on required tables
- Set up RLS policies (implemented in the migration)
- Test permissions

### Phase 2: New Endpoints (Day 2-3)
- Create new simplified endpoints (implemented in `handlers/chat_handler.go` and `handlers/location_handler.go`)
- Add Supabase client to Go (implemented in `services/supabase_service.go`)
- Implement validation layer
- Add rate limiting

### Phase 3: Parallel Running (Day 4-5)
- Deploy new endpoints alongside WebSocket (implemented in `router/router.go`)
- Mobile team tests new implementation
- Monitor both systems
- Fix any issues

### Phase 4: Migration (Day 6)
- Feature flag to switch traffic
- Monitor error rates
- Gradual rollout (10% → 50% → 100%)

### Phase 5: Cleanup (Day 7)
- Remove WebSocket code (use `scripts/cleanup_websocket.ps1` for Windows or `scripts/cleanup_websocket.sh` for Linux/Mac)
- Remove unused dependencies
- Update documentation
- Archive old code

## Implementation Details

### 1. Database Schema

We've added the following tables:
- `supabase_chat_messages`: Stores chat messages with direct trip references
- `supabase_chat_reactions`: Stores reactions to messages
- `supabase_chat_read_receipts`: Tracks the last message read by each user in each trip
- `supabase_user_presence`: Tracks user online status and typing state

The locations table has been enhanced with:
- `is_sharing_enabled`: Boolean to control location sharing
- `sharing_expires_at`: Timestamp to automatically expire location sharing

### 2. Row-Level Security (RLS)

RLS policies ensure data security:
- Users can only see chat messages for trips they're members of
- Users can only update their own messages
- Location sharing respects user privacy settings

### 3. New API Endpoints

#### Chat Endpoints
- `POST /v1/trips/:tripID/messages`: Send a message
- `GET /v1/trips/:tripID/messages`: Get message history
- `PUT /v1/trips/:tripID/messages/read`: Mark messages as read
- `POST /v1/trips/:tripID/messages/:messageID/reactions`: Add reaction
- `DELETE /v1/trips/:tripID/messages/:messageID/reactions/:emoji`: Remove reaction

#### Location Endpoints
- `PUT /v1/locations`: Update user location
- `GET /v1/trips/:tripID/locations`: Get trip member locations

### 4. Client Implementation

Front-end clients should:
1. Use the new REST endpoints for sending/receiving messages
2. Subscribe to Supabase Realtime channels for real-time updates
3. Implement proper error handling for both REST calls and Realtime events

## Testing 

### Unit Tests
- Unit tests for all new handlers have been implemented
- Run tests with `go test ./handlers/...`

### Integration Tests
- Integration tests for the new endpoints are in `tests/integration/supabase_test.go`
- Run with `go test ./tests/integration/...`

### Load Testing
- Load test scripts are in `scripts/load_tests/`
- Run with k6: `k6 run scripts/load_tests/chat_load_test.js`

## Rollback Plan

If issues arise, you can:
1. Use the feature flag to revert to the WebSocket implementation
2. Run the down migration: `migrate -path db/migrations -database "postgres://..." down 1`
3. Restore the previous version of the application code

## References

- [Supabase Realtime Documentation](https://supabase.com/docs/guides/realtime)
- [PostgreSQL RLS Documentation](https://www.postgresql.org/docs/current/ddl-rowsecurity.html)

## Contact

For questions or assistance, contact:
- Backend Lead: [Insert contact]
- DevOps: [Insert contact]
- Mobile Team Lead: [Insert contact] 