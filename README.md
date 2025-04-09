# Nomad Crew - Backend

## Overview

This repository contains the backend API for the Nomad Crew platform. It is a monolithic application built with Go and the Gin framework, providing RESTful endpoints and WebSocket capabilities to support features like user authentication, crew management, trip planning, and real-time communication. It integrates with PostgreSQL for primary data storage, Redis for caching and session management, and Supabase for authentication features.

## Architecture

The backend follows a layered architecture to promote separation of concerns:

1.  **`main.go`**: Entry point, orchestrates setup.
2.  **`router/`**: Defines API routes using Gin.
3.  **`handlers/`**: Contains Gin handlers to process HTTP requests, validate input, and call appropriate services.
4.  **`models/*/service/`**: Encapsulates business logic for specific domains (e.g., `models/trip/service/`, `models/location/service/`). This is the standard pattern for new development and ongoing refactoring. Permission checks are typically handled within these services.
5.  **`db/`**: Data access layer interacting with the PostgreSQL database using `pgx`.
6.  **`middleware/`**: Provides Gin middleware for concerns like authentication (JWT/JWKS), CORS, error handling, and rate limiting.
7.  **`internal/`**: Houses internal packages like WebSocket management (`ws/`) and event definitions (`events/`).

*Note:* Some older business logic might still reside in the `services/` directory, which is being progressively refactored into the `models/*/service/` pattern.

Refer to `PROJECT_STRUCTURE.md` for a detailed breakdown of directories and files.

## Technology Stack

-   **Language**: Go
-   **Web Framework**: Gin
-   **Database**: PostgreSQL (using `pgx` driver)
-   **Cache/KV Store**: Redis
-   **WebSockets**: Gorilla WebSocket
-   **Configuration**: Viper (YAML files + Environment Variables)
-   **Logging**: Zap
-   **Authentication**: JWT (`golang-jwt/jwt/v5`), Supabase integration
-   **Migrations**: SQL files (`db/migrations/`)
-   **Containerization**: Docker
-   **Testing**: Go `testing`, `testify`, `testcontainers-go`
-   **CI/CD**: GitHub Actions
-   **Deployment**: Google Cloud Run (Primary), Fly.io (Secondary)

## Getting Started

1.  **Clone the repository.**
2.  **Install Go** (See official Go documentation).
3.  **Setup Environment Variables**: Copy `.env.example` to `.env` and fill in the required values (Database connection, Redis address, JWT secrets, Supabase keys, external API keys).
4.  **Setup Dependencies**: Run `go mod tidy`.
5.  **Setup Local Database & Redis**: Use Docker Compose for easy setup: `docker-compose up -d`.
6.  **Run Database Migrations**: Execute the migration scripts located in `db/migrations/` (e.g., using `psql` or a migration tool).
7.  **Run the Application**: `go run main.go`.

## Environment Variables

Refer to the `.env.example` file for a comprehensive list. Key variables include:

### Required Core Variables
- `DB_CONNECTION_STRING`: PostgreSQL connection string (alternative to individual DB vars)
- `REDIS_ADDRESS`: Redis server address
- `JWT_SECRET_KEY`: JWT signing key

### Required Supabase Integration
- `SUPABASE_URL`, `SUPABASE_ANON_KEY`, `SUPABASE_SERVICE_KEY`, `SUPABASE_JWT_SECRET`

### Other Required Variables
- `GEOAPIFY_KEY`, `PEXELS_API_KEY`, `RESEND_API_KEY`

### Optional/Configuration Variables
- `SERVER_ENVIRONMENT`: `development`, `staging`, or `production`
- `ALLOWED_ORIGINS`: CORS allowed origins (comma-separated)
- `LOG_LEVEL`: `debug`, `info`, `warn`, `error`

## Deployment

The application is containerized using Docker and designed for cloud deployment.

-   **Primary Target**: Google Cloud Run
-   **Secondary Target**: Fly.io (Configuration available in `fly.toml`)

Deployment is automated via GitHub Actions workflows (`.github/workflows/`):

1.  **`deploy.yml`**: Handles deployment (likely configured for GCR or Fly.io). Requires necessary secrets (`FLY_API_TOKEN` for Fly.io, or GCP credentials for GCR) configured in GitHub repository secrets.
2.  **`main.yml`**: Handles CI steps like testing, linting, security scanning, and building/pushing the Docker image.
3.  **`golang-cilint.yml`**: Runs `golangci-lint`.

### Deployment Notes

-   Always use HTTPS in production.
-   Manage secrets securely (e.g., using cloud provider secret managers or environment variables injected during deployment).
-   Monitor application performance, logs, and database metrics.

## Contribution Guidelines

Follow the [GitHub Flow](https://docs.github.com/en/get-started/quickstart/github-flow). Branches should be named `feature/<your-feature-name>` or `bugfix/<bug-name>`. Ensure code quality and adherence to security best practices.

## Project Roadmap

*(Keep this section updated with high-level goals and phases)*

-   Phase 1: Core setup, Auth, Trip Management (Refactored)
-   Phase 2: Location Service Refactoring (Complete), Service Pattern Standardization (Ongoing)
-   Phase 3: Documentation Update (Ongoing), Feature Verification & Implementation
-   Phase 4: Comprehensive Testing, Deployment Hardening

## Contact and Support

-   **Maintainers**: [List of Maintainers]
-   **Community**: [Slack Channel](https://join.slack.com/t/slack-les9847/shared_invite/zt-2a0dqjzvk-YLC9TQFBExNnPFsH9yAB6g)

---
*README maintained with AI assistance. Refer to Memory Bank and `PROJECT_STRUCTURE.md` for detailed context.*
