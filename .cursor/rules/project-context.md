# NomadCrew Backend Project Context

## Project Overview
- **Name**: NomadCrew Backend
- **Type**: RESTful API with WebSocket Support
- **Stack**: Go, Gin, PostgreSQL, Redis, WebSockets
- **Deployment**: Google Cloud Run
- **Environment**: Production (https://nomadcrew.uk), Preview (https://preview.nomadcrew.uk)

## Project Structure
```
├── api/           # API documentation and OpenAPI specs
├── config/        # Configuration management
├── db/           # Database migrations and schemas
├── domain/       # Domain models and business logic
├── errors/       # Custom error definitions
├── handlers/     # HTTP request handlers
├── internal/     # Internal packages
├── logger/       # Logging configuration
├── middleware/   # HTTP middleware components
├── models/       # Data models
├── pkg/          # Reusable packages
├── router/       # Route definitions
├── services/     # Business logic services
├── static/       # Static assets
├── tests/        # Integration and unit tests
└── types/        # Type definitions
```

## Core Components
1. **Database Layer**
   - PostgreSQL for persistent storage
   - Redis for caching and session management
   - Migrations managed via SQL files in `/db/migrations`

2. **API Layer**
   - RESTful endpoints using Gin framework
   - WebSocket support for real-time features
   - JWT-based authentication
   - Supabase integration

3. **External Services**
   - Resend.com for email services
   - Supabase for additional auth features
   - Geoapify for geocoding
   - Pexels API for images

## Recent Changes

### March 14, 2025, 17:45 UTC - Enhanced JWT Validation
- Updated JWT validation in `middleware/auth.go` to support multiple formats of Supabase JWT secrets:
  - Raw secret (as is)
  - Standard base64 decoded (StdEncoding)
  - Raw URL-safe base64 decoded (RawURLEncoding, without padding)
  - URL-safe base64 decoded (URLEncoding, with padding)
- Improved debug handler in `handlers/debug.go` for better JWT validation diagnostics
- Fixed issue with Supabase tokens not having `kid` claim, falling back to static secret validation

## CI/CD Pipeline
1. **Pull Request Workflow**
   - Linting via golang-cilint
   - Preview environment deployment
   - Automatic cleanup of preview environments

2. **Main Branch Deployment**
   - Automated tests
   - Security scanning
   - Production deployment to Google Cloud Run

## Current State (March 13, 2024)
1. **Active Features**
   - User authentication and authorization
   - Database operations with PostgreSQL
   - Redis caching implementation
   - WebSocket real-time communication
   - Email service integration
   - Geocoding functionality
   - Image management

2. **Infrastructure**
   - Docker containerization
   - Google Cloud Run deployment
   - Prometheus metrics
   - Zap logging
   - Health check endpoints

## Recent Updates
- March 13, 2024: Initial project context documentation created
- March 12, 2024: Updated CI/CD pipeline with preview environments
- March 10, 2024: Implemented Cloud Run deployment workflow

## Development Guidelines
1. **Code Organization**
   - Follow Go standard project layout
   - Maintain separation of concerns
   - Use dependency injection
   - Implement clean architecture principles

2. **Testing Strategy**
   - Unit tests for business logic
   - Integration tests with testcontainers
   - API endpoint testing
   - Coverage monitoring

3. **Security Measures**
   - Environment-based configuration
   - Secure headers
   - Rate limiting
   - Input validation

4. **Performance Optimization**
   - Database query optimization
   - Redis caching strategy
   - Connection pooling
   - Goroutine management

## Documentation
- API documentation available in `/api`
- Integration guide in root directory
- README.md with setup instructions
- Deployment configurations in `/deployment`

## Monitoring and Observability
- Prometheus metrics exposed at `/metrics`
- Health checks at `/health/liveness`
- Structured logging with Zap
- Error tracking and reporting

This document will be updated automatically as the project evolves.