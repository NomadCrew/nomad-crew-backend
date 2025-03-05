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

Required for operation:

- `SSE_API_KEY`: Secret key for SSE endpoint authentication
- `JWT_SECRET_KEY`: JWT signing key (min 32 chars)
- `DB_CONNECTION_STRING`: Postgres connection string
- `REDIS_ADDRESS`: Redis server address

## Deployment Notes

Always:

1. Use HTTPS in production
2. Rotate secrets regularly
3. Monitor database connection pool metrics
4. Enable Redis persistence

---
Generated using GPT-4
## Workflow Consolidation

The GitHub Actions workflows have been consolidated to eliminate redundancy:

1. **deploy.yml** - This is the consolidated workflow that handles all AWS infrastructure deployment using Terraform. It combines the best features of the previous `deploy.yml` and `terraform-deploy.yml` workflows.

2. **main.yml** - This workflow handles CI/CD for the Go backend, including testing, security scanning, and building/pushing Docker images to GitHub Container Registry.

3. **golang-cilint.yml** - This workflow runs Go linting checks.

The redundant `terraform-deploy.yml` workflow has been removed.
