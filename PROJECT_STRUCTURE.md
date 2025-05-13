# NomadCrew Backend Project Structure

This document provides a high-level overview of the NomadCrew backend project structure.

## Root Directory (`./`)

- `main.go`: Application entry point. Initializes configuration, database connections (PostgreSQL, Redis, Supabase), services, handlers, middleware, defines API routes using Gin, and manages graceful server shutdown.
- `go.mod`, `go.sum`: Go module dependency management.
- `Dockerfile`: Defines the Docker image build process.
- `docker-compose.yml`: Configures services for local development (e.g., database, Redis).
- `.env`, `.env.example`: Environment variable configuration (local vs. example).
- `README.md`: Project overview and setup instructions.
- `.gitignore`: Specifies intentionally untracked files.
- `.cursor/`: Cursor-specific configuration and rules.
  - `rules/`: Contains custom instructions for the AI assistant.
    - `project-context.md`: Dynamically updated log of significant changes and current project state.
    - `go-dev.mdc`: Rules and guidelines for Go development specific to this project.
- `PROJECT_STRUCTURE.md`: (This file) An index of the project's structure and file summaries.

## `api/` (Legacy/Unused?)

- Contains older API-related code, likely superseded by newer structure.

## `assets/`

- Static assets, possibly email templates or other resources.

## `config/`

- Handles application configuration loading (e.g., from YAML files or environment variables).
- `config.go`, `config.*.yaml`: Defines and loads configuration structures.

## `db/`

- Database interaction layer (PostgreSQL).
- `db.go`: Initializes DB connections/pools.
- `db_client.go`: Wrapper around the pgxpool connection.
- `*.go` (e.g., `trip.go`, `todo.go`, `location.go`, `chat.go`): Data access logic (CRUD operations) for specific database tables/models.
- `migrations/`: SQL migration files for database schema changes.
- `test_setup.go`: Test helpers for database setup.

## `handlers/`

- Handles incoming HTTP requests (Gin handlers).
- `trip_handler.go`: Contains Gin handler functions for core trip operations (CRUD, status updates, search).
- `trip_chat_handler.go`: Manages HTTP and WebSocket communications for trip-specific chat, including listing messages, updating read status, and streaming real-time chat events. Chat is always tied to a specific trip.
- `member_handler.go`: Handles operations related to trip members (listing, adding, updating roles, removing).
- `invitation_handler.go`: Manages trip invitations, primarily sending invitations. (Note: Acceptance and full lifecycle management needs verification).
- `todo_handler.go`: Contains Gin handler functions for managing to-do items specifically within the context of a trip.
- `location_handler.go`: Handles location updates for users (general) and trip members.
- `user_handler.go`: Manages user profile operations, preferences, and Supabase data synchronization.
- `notification_handler.go`: Handles fetching and managing user notifications.
- `auth_handler.go`: Contains handlers related to authentication processes (though primary JWT validation is middleware-driven).
- `ws_handler.go`: Provides the generic WebSocket connection point (`/v1/ws`). Its role for broadcasting general notifications is under review for optimization. Trip-specific chat WebSockets are handled by `trip_chat_handler.go`.
- `health_handler.go`: Handlers for `/health` endpoints.
- `debug.go`: (If still present and used) Handlers for debugging endpoints (non-production).

## `internal/`

- Private application code, not intended for external use.
- `auth/`: Authentication-related logic (e.g., token generation/validation helpers). Potentially overlaps with `middleware/auth.go`.
- `errors/`: Custom error types for the application.
- `events/`: Defines event structures used for real-time communication (e.g., via Redis Pub/Sub).
- `store/`: Interfaces defining data storage operations (potentially older pattern).
- `utils/`: General utility functions.
- `ws/`: Provides foundational WebSocket connection management utilities (e.g., Upgrader, client connection wrappers, message read/write helpers). Specific WebSocket application logic (like trip chat event streaming) might be handled closer to or within dedicated handlers (e.g., `trip_chat_handler.go`).

## `logger/`

- Configures and provides the application's logger (Zap).

## `middleware/`

- Gin middleware functions.
- `auth.go`, `jwt_validator.go`, `jwks_cache.go`: Handles JWT authentication and validation.
- `auth_ws.go`: WebSocket-specific authentication.
- `cors.go`: Configures Cross-Origin Resource Sharing.
- `error_handler.go`: Centralized error handling middleware.
- `rate_limit.go`: Implements rate limiting (likely using Redis).
- `rbac.go`: Role-Based Access Control logic.
- `websocket.go`: Core WebSocket connection handling middleware.

## `models/`

- Defines data structures representing application entities (often matching database tables).
- `*.go` (e.g., `user.go`, `notification.go`, `todo.go`): Struct definitions for data models.
- `trip/`: Specific models related to the Trip entity.
- `model.go`: Potentially base model logic or older model definitions.

## `pkg/`

- Public library code, reusable across different projects (though likely internal to NomadCrew). Contains utilities or shared components.

## `router/`

- Defines the application's API routes using the Gin framework.
- `router.go`: Contains the `SetupRouter` function which initializes all middleware and maps HTTP methods and URL patterns to their respective handler functions in the `handlers/` directory. It organizes routes into groups (e.g., versioned, authenticated).

## `scripts/`

- Utility scripts for development or deployment tasks.

## `service/` (Service Layer - Evolving)

- This directory appears to be the newer, preferred location for business logic service implementations (e.g., `notification_service.go`).
- The project is migrating towards placing new service logic here, aiming for a clear separation of concerns where services encapsulate core business operations, coordinate data access, and are called by handlers.

## `services/` (Legacy Service Layer)

- Business logic layer. Encapsulates core application logic, coordinating data access and other operations.
- `*_service.go` (e.g., `chat_service.go`, `location_service.go`, `email_service.go`, `health_service.go`, `event_service.go`): Implementations of business logic for different domains.
- `redis_event_service.go`: Handles event publishing/subscribing via Redis.

## `store/` (Root Level)

- Contains specific data store implementations, often conforming to interfaces defined elsewhere (e.g., `internal/store` or directly used by services).
- `postgres/`: PostgreSQL specific implementations (e.g., `pg_notification_store.go`, `pg_user_store.go`).

## `tests/`

- Contains integration and end-to-end tests.
- `integration/`: Integration tests, possibly using testcontainers.
- `mocks/`: Generated mocks for testing.

## `types/`

- Shared type definitions used across the application (e.g., enums, common structs).

---
*This index should be kept up-to-date as the project evolves.* 