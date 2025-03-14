# Nomad Crew - Backend

## Overview

This repository is dedicated to the backend services of Nomad Crew. Built using Go, it handles functionalities like user authentication, group management, location tracking, messaging, media handling, and expense management through microservices.

## Services

- User Service
- Group Service
- Location Service
- Messaging Service
- Media Service
- Expense Service

## Technology Stack

- Go with gRPC for service communication
- PostgreSQL and Redis for database management
- Apache Kafka for messaging and event streaming

## Development Guidelines

- Robust error handling and logging (using Zap for Go logging).
- Efficient inter-service communication with gRPC.
- Adherence to RESTful principles where applicable.
- Database normalization and proper indexing for location-based queries.

## Getting Started

Instructions for setting up the backend development environment will be provided here.

## Contribution Guidelines

Follow the [GitHub Flow](https://docs.github.com/en/get-started/quickstart/github-flow). Branches for new features should be named as `feature/your-feature-name`, and bug fixes as `bugfix/bug-name`. Ensure compliance with [OWASP security standards](https://owasp.org/www-project-mobile-app-security/).

## Project Roadmap

- Phase 1: Setup and Basic Functionality
- Phase 2: Advanced Features
- Phase 3: Additional Features
- Phase 4: Testing and Deployment

## Contact and Support

- **Maintainers**: [List of Maintainers]
- **Community**: [Slack Channel](https://join.slack.com/t/slack-les9847/shared_invite/zt-2a0dqjzvk-YLC9TQFBExNnPFsH9yAB6g)

## Environment Variables

### Required Core Variables
- `DB_CONNECTION_STRING`: PostgreSQL connection string
- `REDIS_ADDRESS`: Redis server address
- `JWT_SECRET_KEY`: JWT signing key for generating invitation tokens (min 32 chars)

### Required Supabase Integration
- `SUPABASE_URL`: Your Supabase project URL
- `SUPABASE_ANON_KEY`: Supabase anon key for client operations
- `SUPABASE_SERVICE_KEY`: Supabase service key for admin operations (used by ChatStore)
- `SUPABASE_JWT_SECRET`: Secret used to validate Supabase JWTs

### Other Required Variables
- `GEOAPIFY_KEY`: API key for geolocation services
- `PEXELS_API_KEY`: API key for Pexels image service
- `RESEND_API_KEY`: API key for the Resend email service

### Optional/Configuration Variables
- `SERVER_ENVIRONMENT`: Set to 'development', 'staging', or 'production'
- `ALLOWED_ORIGINS`: CORS allowed origins (comma-separated)
- `LOG_LEVEL`: Logging level (debug, info, warn, error)

### Database Configuration (Can use DB_CONNECTION_STRING instead)
- `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSL_MODE`

### Redis Configuration (Additional options)
- `REDIS_PASSWORD`, `REDIS_DB`, `REDIS_USE_TLS`, `REDIS_POOL_SIZE`, `REDIS_MIN_IDLE_CONNS`

## Deployment Notes

Always:

1. Use HTTPS in production
2. Rotate secrets regularly
3. Monitor database connection pool metrics
4. Enable Redis persistence

## Fly.io Deployment

This project is configured for deployment on Fly.io, a platform that provides a cost-effective way to host applications.

### Prerequisites

1. Install the Fly CLI: [Installation Guide](https://fly.io/docs/hands-on/install-flyctl/)
2. Sign up for a Fly.io account
3. Log in to Fly.io: `flyctl auth login`

### Deployment Steps

1. Set up your secrets:
   ```bash
   flyctl secrets set JWT_SECRET_KEY=your_jwt_secret \
     DB_PASSWORD=your_db_password \
     REDIS_PASSWORD=your_redis_password \
     # Add other required secrets here
   ```

2. Deploy the application:
   ```bash
   flyctl deploy
   ```

3. Check the deployment status:
   ```bash
   flyctl status
   ```

### Database and Redis Setup

This project uses:
- **Database**: Neon.tech PostgreSQL (free tier)
- **Redis Cache**: Upstash Redis (free tier)

Make sure to set up these services and update the connection details in your Fly.io secrets.

### GitHub Actions Integration

The repository includes a GitHub Actions workflow for automatic deployment to Fly.io. To use it:

1. Add your `FLY_API_TOKEN` to your GitHub repository secrets
2. Add all other required environment variables to your GitHub repository secrets
3. Push to the main branch or manually trigger the workflow

---
Generated using GPT-4
## Workflow Consolidation

The GitHub Actions workflows have been consolidated to eliminate redundancy:

1. **deploy.yml** - This is the consolidated workflow that handles all AWS infrastructure deployment using Terraform. It combines the best features of the previous `deploy.yml` and `terraform-deploy.yml` workflows.

2. **main.yml** - This workflow handles CI/CD for the Go backend, including testing, security scanning, and building/pushing Docker images to GitHub Container Registry.

3. **golang-cilint.yml** - This workflow runs Go linting checks.

The redundant `terraform-deploy.yml` workflow has been removed.
