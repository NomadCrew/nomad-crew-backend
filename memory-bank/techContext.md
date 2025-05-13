# NomadCrew Backend - Technical Context

## Project Architecture
- Follows standard Go project structure
- Layered architecture: handlers  services  models  db
- RESTful API design with Gin
- PostgreSQL database with migrations
- Redis for caching and real-time features
- WebSockets for real-time communication

## Key Components
- User authentication with JWT and Supabase
- Trip management system
- Location tracking and management
- Real-time chat functionality
- Email notification service
- Weather data integration

## Development Environment
- Go 1.24.x
- Docker and docker-compose for local development
- GitHub Actions for CI/CD
- Deployed on Google Cloud Run
