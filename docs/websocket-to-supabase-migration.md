# WebSocket to Supabase Realtime Migration

This document outlines the migration from our custom WebSocket implementation to Supabase Realtime for real-time features in the Nomad Crew backend.

## Migration Overview

### What Was Removed

- Custom WebSocket handlers (`handlers/ws_handler.go`)
- WebSocket middleware (`middleware/websocket.go`)
- WebSocket client logic (`internal/ws/client.go`)
- WebSocket-related tests
- WebSocket routes in `router.go`
- WebSocket health checks
- WebSocket type definitions in `types/chat.go`
- Dependency: `github.com/gorilla/websocket`

### What Was Added

- Supabase Realtime handlers:
  - `ChatHandlerSupabase`
  - `LocationHandlerSupabase`
- Feature flag system to control which implementation is active (`config.FeatureFlags.EnableSupabaseRealtime`)
- Direct database tables for chat and presence functionality
- Backend-to-Supabase communication service

## Benefits of Migration

1. **Code Reduction**: ~2,000 lines of error-prone WebSocket code eliminated
2. **Reliability**: Improved connection stability with Supabase's managed service
3. **Scalability**: Better handling of large numbers of concurrent connections
4. **Developer Experience**: Simplified API for real-time features
5. **Maintenance**: Reduced custom code to maintain

## Architecture Changes

### Previous Architecture

```
Mobile App → HTTP/WS → Go Backend → Custom WebSocket Handler → Event Broadcasting
                ↓                            ↓
            Supabase Auth              NeonDB PostgreSQL
```

### New Architecture

```
Mobile App → HTTP → Go Backend → NeonDB PostgreSQL
     ↓                               ↑
     └→ Supabase Realtime ←─────────┘ (via Postgres Logical Replication)
     ↓
Supabase Auth
```

## API Changes

### Removed Endpoints

- `/v1/ws` - Main WebSocket connection
- `/v1/trips/:id/chat/ws/events` - Trip-specific WebSocket stream

### New Endpoints (Supabase Realtime)

- `/v1/trips/:id/chat/messages` - REST endpoint for sending/receiving messages
- `/v1/trips/:id/chat/messages/read` - Update read status
- `/v1/trips/:id/chat/messages/:messageId/reactions` - Add reaction
- `/v1/trips/:id/chat/messages/:messageId/reactions/:emoji` - Remove reaction

## Integration for Frontend Developers

For frontend developers integrating with this new backend:

1. Use Supabase client with the provided Supabase URL and anon key
2. Subscribe to Supabase Realtime tables:
   - `supabase_chat_messages`
   - `supabase_chat_reactions`
   - `supabase_user_presence`
3. Use REST endpoints for all non-real-time operations
4. Implement offline support with message queuing

## Feature Flag Control

The migration is controlled by the feature flag `ENABLE_SUPABASE_REALTIME`. When enabled:

- WebSocket endpoints are not registered
- Supabase Realtime handlers are used
- New database tables are utilized

## Testing

To test the new implementation:

1. Set `ENABLE_SUPABASE_REALTIME=true` in your environment
2. Run the backend server
3. Use the new REST endpoints for chat and location operations
4. Connect to Supabase Realtime for real-time updates 