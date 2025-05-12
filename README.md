# Nomad Crew - Backend

## Overview

This repository contains the backend API for the **Nomad Crew** platform — a mobile-first app that simplifies group travel planning through real-time coordination, expense tracking, and collaborative features.

The backend is a **monolithic Go application** built using the **Gin** web framework. It exposes RESTful endpoints and WebSocket interfaces to power features like:

- User authentication and profile management
- Comprehensive trip planning and management
- Trip-specific real-time chat and event streaming via WebSockets
- Real-time location tracking for users and trip members
- Trip-specific to-do list management
- Member invitation system for trips (acceptance flow needs verification)
- User notification system (real-time delivery strategy under review for optimization)
- Role-based permissions (via middleware and service layer checks)

It integrates with **PostgreSQL** for persistent storage, **Redis** for caching, and **Supabase** for authentication services.

👉 [Frontend Repository](https://github.com/NomadCrew/nomad-crew-frontend)

---

## MVP Release Focus

We're currently working toward an MVP release with these priorities:

### Core MVP Functionality

- User authentication (via Supabase) and account/profile management
- Trip Creation, Management: Create, list, search, view, update, delete trips; manage trip status.
- Trip Member Management: Add members to trips, update their roles, remove them.
- Trip Invitation System: Invite users to trips. (Note: Invitation acceptance and lifecycle management flows require full verification and potential implementation).
- Trip-Specific Real-Time Chat: WebSocket-based chat within trips for coordination, including listing messages and managing read status. (Message sending/reactions via HTTP are in progress). Chat is automatically created with each trip.
- Location Sharing: Real-time location updates for users (general) and specifically for trip members.
- Trip-Specific To-Do Lists: Manage tasks and checklists within the context of a trip.
- Basic Notification System: Users can receive and manage notifications. (Note: The real-time delivery mechanism for general notifications is under review for cost-effectiveness and efficiency, distinct from active trip chat).

### Current Focus

- Stabilizing critical services (chat, websockets, auth)
- Ensuring core API endpoints work reliably
- Fixing compilation errors and critical bugs
- Verifying end-to-end flows for key user journeys

### Post-MVP Improvements

- Enhanced documentation with complete Swagger annotations
- Expanded test coverage
- Non-critical bug fixes and optimizations
- Additional features as prioritized

> Our goal is to deliver a functional, stable product as quickly as possible to gather user feedback.

---

## Architecture

The codebase follows a layered, modular architecture to promote testability and maintainability:

1. **`main.go`** – Application entry point: sets up dependencies and starts the server.
2. **`router/`** – Defines API routes and middleware.
3. **`handlers/`** – Gin HTTP handlers: parse input, validate, call services.
4. **`models/*/service/`** – Business logic layer for each domain (e.g., trips, locations, crews). All permission checks happen here.
5. **`db/`** – Data access layer using `pgx` to query PostgreSQL.
6. **`middleware/`** – Custom middleware for JWT validation, CORS, error handling, rate limiting.
7. **`internal/`** – Internal systems including:
   - `ws/`: WebSocket manager for live communication
   - `events/`: Internal event struct definitions and dispatching

> ⚠️ Some legacy logic still resides in `services/`. All new features should follow the `models/*/service/` convention.

Refer to `PROJECT_STRUCTURE.md` for a detailed breakdown of the directory structure.

---

## Technology Stack

| Layer               | Tool / Library                             |
|---------------------|---------------------------------------------|
| Language            | Go                                          |
| Web Framework       | Gin                                         |
| Database            | PostgreSQL + `pgx` driver                   |
| Caching             | Redis                                       |
| Real-time           | Gorilla WebSocket                           |
| Authentication      | Supabase + JWT (`golang-jwt/jwt/v5`)        |
| Config Management   | Viper (YAML + Env vars)                    |
| Logging             | Uber Zap                                    |
| Migrations          | SQL-based (`db/migrations/`)               |
| Containerization    | Docker                                      |
| CI/CD               | GitHub Actions                              |
| Deployment          | Google Cloud Run (Primary), Fly.io (Backup)|
| Testing             | Go `testing`, `testify`, `testcontainers-go`|

---

## API Documentation

The API is documented using Swagger/OpenAPI. You can access the interactive documentation at `/swagger/index.html` when the server is running.

## Generating Documentation

To generate or update the API documentation:

1. Ensure your handler functions are properly annotated with Swagger comments
2. Run the following command:

```bash
swag init -g main.go -o ./static/docs/api
```

## Documentation Tool

We provide a documentation helper tool to assist with adding Swagger annotations:

```bash
cd scripts/doc_generator
go run main.go
```

This will scan all handlers and provide templates for undocumented endpoints.

For more information, see the [API Documentation Guide](docs/api-documentation-guide.md).

---

## Authentication Flow

User authentication is handled by Supabase. After logging in, Supabase issues a JWT that the backend validates via JWKS. Auth middleware extracts and verifies the token on protected endpoints. Role and permission checks are enforced in service layers.

---

## Real-Time Features

The application leverages WebSockets and an internal event system for real-time updates:

- **Trip-Specific Chat & Events:** A dedicated WebSocket connection (`/v1/trips/:id/chat/ws/events`) is established when a user engages with a specific trip's chat. This connection handles real-time message exchange and other trip-specific events directly related to that chat session.
- **Live Location Updates:** User and trip member location data is updated in real-time.
- **General Notifications & Updates:** The system supports broadcasting other events and general notifications (e.g., new trip invitations, general alerts). The current real-time delivery strategy for these general updates (potentially via the `/v1/ws` endpoint or other mechanisms) is being evaluated to ensure cost-effectiveness and optimal performance, differentiating from the active trip-chat WebSockets.

Internal events are defined in `internal/events/` and may be dispatched through services like `services/event_service.go` (potentially using Redis Pub/Sub for inter-service communication if applicable).

---

## Getting Started (Local Setup)

1. **Clone the repository**
2. **Install Go** (<https://golang.org/doc/install>)
3. **Set up environment variables**  
   Copy `.env.example` → `.env` and configure:
   - Database URL
   - Redis URL
   - Supabase keys
   - JWT secrets
   - 3rd party API keys (Geoapify, Pexels, Resend)
4. **Install dependencies**

   ```bash
   go mod tidy

```

5. **Start DB and Redis (Docker)**

   ```bash
   docker-compose up -d
   ```

6. **Run migrations**

   ```bash
   psql -d <db> -f db/migrations/init.sql
   ```

7. **Run the app**

   ```bash
   go run main.go
   ```

---

## Common Dev Commands

| Task        | Command              |
| ----------- | -------------------- |
| Run server  | `go run main.go`     |
| Run tests   | `go test ./...`      |
| Lint code   | `golangci-lint run`  |
| Live reload | `air` (if installed) |
| Format code | `gofmt -w .`         |

---

## Environment Variables

See `.env.example` for a full list. Below are key variables:

### Core

- `DB_CONNECTION_STRING`
- `REDIS_ADDRESS`
- `JWT_SECRET_KEY`

### Supabase

- `SUPABASE_URL`
- `SUPABASE_ANON_KEY`
- `SUPABASE_SERVICE_KEY`
- `SUPABASE_JWT_SECRET`

### Others

- `GEOAPIFY_KEY`, `PEXELS_API_KEY`, `RESEND_API_KEY`
- `SERVER_ENVIRONMENT`, `LOG_LEVEL`, `ALLOWED_ORIGINS`

---

## Deployment

The app is designed for cloud-native deployment with Docker and GitHub Actions.

### Targets

- ✅ **Primary**: Google Cloud Run
- 🛑 **Backup**: Fly.io (manual fallback)

### CI/CD Workflows

- `deploy-cloud-run.yml`, `pr-preview-cloud-run.yml` – deploy on merge or PR
- `main.yml` – runs tests, security checks, builds/pushes Docker image
- `golang-cilint.yml` – linting workflow

### Notes

- Always use HTTPS in production.
- Use secret managers (e.g., GCP Secret Manager) to avoid hardcoding secrets.

---

## Contribution Guidelines

- Follow [GitHub Flow](https://docs.github.com/en/get-started/quickstart/github-flow).
- Branch naming:
  `feature/<name>` for features, `bugfix/<name>` for fixes
- Ensure all new logic follows `models/*/service/` structure
- Add tests and docs for new endpoints or behaviors

---

## Project Resources

- **Frontend**: [Nomad Crew Frontend](https://github.com/NomadCrew/nomad-crew-frontend)
- **Architecture Diagram**: Coming soon
- **Schema Diagrams**: Coming soon
- **API Docs**: Coming soon (Swagger)

---

## Contact

- **Maintainers**: \[Add yourself here]
- **Community**: [Join Slack](https://join.slack.com/t/slack-les9847/shared_invite/zt-2a0dqjzvk-YLC9TQFBExNnPFsH9yAB6g)

---

**Nomad Crew** is built to make group travel less chaotic and more memorable. If you'd like to contribute or have suggestions, we'd love to hear from you.
