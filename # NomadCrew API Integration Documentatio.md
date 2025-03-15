# NomadCrew API Integration Documentation

## 1. Project Overview
```markdown
### Basic Information
- Project Name: NomadCrew
- Frontend: React Native Expo App
- Backend: Go Gin Backend on Cloud Run
- Environment URLs:
  - Production: https://api.nomadcrew.uk
  - Preview: https://preview.nomadcrew.uk

### Team Contacts
[Note: Team contacts should be filled by the organization]
- Frontend Team Lead: [Name]
- Backend Team Lead: [Name]
- Integration Developer: [Name]
- Project Manager: [Name]

### Timeline
[Note: Timeline should be filled by the project management team]
- Integration Start Date: [Date]
- Expected Completion: [Date]
- Key Milestones: [List]
```

## 2. Technical Specifications

### 2.1 Frontend Configuration
```typescript
// src/api/env.ts
export const API_CONFIG = {
  BASE_URL: process.env.EXPO_PUBLIC_API_URL || (__DEV__ 
    ? 'https://preview.nomadcrew.uk'
    : 'https://api.nomadcrew.uk'),
  VERSION: 'v1',
  TIMEOUT: 10000,
  WEBSOCKET: {
    RECONNECT_INTERVAL: 30000,
    PING_INTERVAL: 30000,
    PONG_TIMEOUT: 60000
  }
}
```

#### Required Headers
```typescript
{
  'Content-Type': 'application/json',
  'Authorization': 'Bearer ${token}',
  'Accept': 'application/json'
}
```

#### Error Handling Implementation
```typescript
interface ApiError {
  type: string;
  message: string;
  detail?: string;
}

const handleApiError = (error: any): ApiError => {
  if (error.response) {
    const { data } = error.response;
    switch (error.response.status) {
      case 401:
        // Handle unauthorized - redirect to login
        break;
      case 403:
        // Handle forbidden - show permission error
        break;
      case 404:
        // Handle not found
        break;
      case 422:
        // Handle validation errors
        break;
      default:
        // Handle other errors
    }
    return {
      type: data.type || 'UNKNOWN_ERROR',
      message: data.message || 'An unknown error occurred',
      detail: data.detail
    };
  }
  return {
    type: 'NETWORK_ERROR',
    message: 'Network error occurred',
    detail: error.message
  };
};
```

### 2.2 Backend Configuration
```go
// CORS Configuration
r.Use(middleware.ErrorHandler())
r.Use(cors.New(cors.Config{
  AllowOrigins: []string{
    "https://preview.nomadcrew.uk",
    "https://app.nomadcrew.uk"
  },
  AllowMethods: []string{
    "GET",
    "POST",
    "PUT",
    "DELETE",
    "OPTIONS",
    "PATCH"
  },
  AllowHeaders: []string{
    "Content-Type",
    "Authorization",
    "Accept"
  },
  AllowCredentials: true,
  MaxAge: 86400
}))
```

#### Required Environment Variables
```bash
# Backend .env template
SERVER_ENVIRONMENT=development
PORT=8080
JWT_SECRET_KEY=your-secret-here
ALLOWED_ORIGINS=https://preview.nomadcrew.uk,https://app.nomadcrew.uk

# Database Configuration
DB_HOST=ep-blue-sun-a8kj1qdc-pooler.eastus2.azure.neon.tech
DB_PORT=5432
DB_USER=neondb_owner
DB_NAME=neondb
DB_PASSWORD=your-password-here
DB_SSL_MODE=require

# Redis Configuration
REDIS_ADDRESS=actual-serval-57447.upstash.io:6379
REDIS_PASSWORD=your-redis-password
REDIS_DB=0
REDIS_USE_TLS=true

# External Services
GEOAPIFY_KEY=your-geoapify-key
PEXELS_API_KEY=your-pexels-key
SUPABASE_ANON_KEY=your-supabase-anon-key
SUPABASE_URL=your-supabase-url
SUPABASE_JWT_SECRET=your-supabase-jwt-secret

# Email Configuration
EMAIL_FROM_ADDRESS=welcome@nomadcrew.uk
EMAIL_FROM_NAME=NomadCrew
RESEND_API_KEY=your-resend-api-key
```

### 2.3 Cloud Run Configuration
```yaml
# service.yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: nomadcrew-api
spec:
  template:
    metadata:
      annotations:
        autoscaling.knative.dev/maxScale: "10"
    spec:
      containerConcurrency: 80
      timeoutSeconds: 300
      containers:
        - image: gcr.io/[PROJECT_ID]/nomadcrew-api
          ports:
            - containerPort: 8080
          env:
            - name: SERVER_ENVIRONMENT
              value: "production"
          resources:
            limits:
              cpu: "1"
              memory: "256Mi"
            requests:
              cpu: "200m"
              memory: "128Mi"
```

## 3. API Endpoints Documentation

### 3.1 Authentication
```markdown
#### Refresh Token
- Endpoint: `/v1/auth/refresh`
- Method: POST
- Request Body:
  ```json
  {
    "refresh_token": "string"
  }
  ```
- Response:
  ```json
  {
    "access_token": "string",
    "refresh_token": "string",
    "expires_in": number,
    "token_type": "bearer"
  }
  ```

### 3.2 Trips
```markdown
#### Create Trip
- Endpoint: `/v1/trips`
- Method: POST
- Auth: Required
- Request Body:
  ```json
  {
    "name": "string",
    "description": "string",
    "destination": {
      "address": "string",
      "latitude": number,
      "longitude": number
    },
    "startDate": "ISO8601 date",
    "endDate": "ISO8601 date",
    "status": "string"
  }
  ```

#### Get Trip
- Endpoint: `/v1/trips/:id`
- Method: GET
- Auth: Required
- Response: Trip object

#### List User Trips
- Endpoint: `/v1/trips/list`
- Method: GET
- Auth: Required
- Response: Array of Trip objects

#### Search Trips
- Endpoint: `/v1/trips/search`
- Method: POST
- Auth: Required
- Request Body: TripSearchCriteria object

#### Update Trip Status
- Endpoint: `/v1/trips/:id/status`
- Method: PATCH
- Auth: Required
- Request Body:
  ```json
  {
    "status": "string"
  }
  ```

### 3.3 Trip Members
```markdown
#### Add Member
- Endpoint: `/v1/trips/:id/members`
- Method: POST
- Auth: Required (Owner only)
- Request Body:
  ```json
  {
    "userId": "string",
    "role": "string"
  }
  ```

#### Update Member Role
- Endpoint: `/v1/trips/:id/members/:userId/role`
- Method: PUT
- Auth: Required (Owner only)

#### Remove Member
- Endpoint: `/v1/trips/:id/members/:userId`
- Method: DELETE
- Auth: Required (Owner or self)

#### Get Trip Members
- Endpoint: `/v1/trips/:id/members`
- Method: GET
- Auth: Required (Any member)
```

### 3.4 Location Services
```markdown
#### Update Location
- Endpoint: `/v1/location/update`
- Method: POST
- Auth: Required
- Request Body: LocationUpdate object

#### Save Offline Locations
- Endpoint: `/v1/location/offline`
- Method: POST
- Auth: Required
- Request Body: Array of offline location updates

#### Get Trip Member Locations
- Endpoint: `/v1/trips/:id/locations`
- Method: GET
- Auth: Required (Member)
- Response: Array of member locations (last 24 hours)
```

### 3.5 Chat System
```markdown
#### Create Chat Group
- Endpoint: `/v1/chats/groups`
- Method: POST
- Auth: Required

#### List Chat Groups
- Endpoint: `/v1/chats/groups`
- Method: GET
- Auth: Required

#### Get Chat Group
- Endpoint: `/v1/chats/groups/:groupID`
- Method: GET
- Auth: Required

#### Update Chat Group
- Endpoint: `/v1/chats/groups/:groupID`
- Method: PUT
- Auth: Required

#### Get Chat Messages
- Endpoint: `/v1/chats/groups/:groupID/messages`
- Method: GET
- Auth: Required

#### Update Last Read Message
- Endpoint: `/v1/chats/groups/:groupID/read`
- Method: PUT
- Auth: Required
```

## 4. Integration Testing

### 4.1 Test Cases
```markdown
#### CORS Verification
1. Preflight Request Test
```bash
curl -X OPTIONS -H "Origin: https://preview.nomadcrew.uk" \
     -H "Access-Control-Request-Method: GET" \
     -H "Access-Control-Request-Headers: Content-Type" \
     -v https://api.nomadcrew.uk/health
```

2. Authentication Flow Test
   - Token refresh
   - Invalid token handling
   - Expired token handling

3. Data Flow Tests
   - Trip creation
   - Member management
   - Location updates
   - Chat functionality

4. Error Handling Tests
   - Invalid requests
   - Unauthorized access
   - Rate limiting
   - Network failures
```

### 4.2 Test Environments
```markdown
#### Preview Environment
- Frontend URL: https://preview.nomadcrew.uk
- Backend URL: https://preview-api.nomadcrew.uk
- Test User Credentials: [Secure Location]

#### Production Environment
- Frontend URL: https://app.nomadcrew.uk
- Backend URL: https://api.nomadcrew.uk
```

## 5. Monitoring and Debugging

### 5.1 Logging Setup
```go
// Backend logging implementation
log := logger.GetLogger()
log.Infow("Operation completed",
    "userID", userID,
    "action", action,
    "status", status,
)
```

### 5.2 Metrics
```markdown
#### Key Metrics to Monitor
1. API Response Times
2. Error Rates
3. WebSocket Connections
   - Active connections
   - Message rates
   - Error rates
4. Database Operations
   - Connection pool status
   - Query performance
5. Redis Operations
   - Connection status
   - Cache hit rates

#### Monitoring Tools
- Cloud Run Metrics
- Prometheus Metrics
  - websocket_active_connections
  - websocket_messages_received_total
  - websocket_messages_sent_total
  - websocket_errors_total
```

## 6. Security Considerations

### 6.1 Security Checklist
```markdown
- [x] SSL/TLS Configuration
- [x] CORS Policy Implementation
- [x] JWT Authentication
- [x] Rate Limiting Implementation
- [x] Input Validation
- [x] Error Handling Security
- [x] WebSocket Security
  - Authentication
  - Message validation
  - Rate limiting
- [x] Database Security
  - Connection encryption
  - Query parameterization
- [x] Redis Security
  - TLS encryption
  - Password protection
```

## 7. Deployment Process

### 7.1 Deployment Checklist
```markdown
#### Pre-Deployment
- [ ] Environment variables configured
- [ ] Database migrations ready
- [ ] CORS settings verified
- [ ] SSL certificates valid
- [ ] API documentation updated

#### Deployment Steps
1. Backend Deployment
   ```bash
   # Build Docker image
   docker build -t gcr.io/[PROJECT_ID]/nomadcrew-api:$VERSION .
   
   # Push to Container Registry
   docker push gcr.io/[PROJECT_ID]/nomadcrew-api:$VERSION
   
   # Deploy to Cloud Run
   gcloud run deploy nomadcrew-api \
     --image gcr.io/[PROJECT_ID]/nomadcrew-api:$VERSION \
     --platform managed \
     --region [REGION] \
     --allow-unauthenticated
   ```

2. Verify Deployment
   - Health check endpoints
   - Database connections
   - Redis connections
   - External service integrations

3. Post-Deployment
   - Monitor error rates
   - Check metrics
   - Verify WebSocket connections
```

## 8. Troubleshooting Guide

### 8.1 Common Issues
```markdown
#### Authentication Issues
1. Symptom: Token refresh failing
   Solution: Verify JWT secret and token expiration settings

2. Symptom: CORS errors
   Solution: Check allowed origins configuration

#### WebSocket Issues
1. Symptom: Connection drops
   Solution: Check network stability and firewall settings

2. Symptom: High latency
   Solution: Monitor server resources and connection count

#### Database Issues
1. Symptom: Connection pool exhaustion
   Solution: Adjust pool settings and check for connection leaks

2. Symptom: Slow queries
   Solution: Review query performance and indexes

#### Redis Issues
1. Symptom: Cache misses
   Solution: Verify TTL settings and cache invalidation logic

2. Symptom: Connection failures
   Solution: Check Redis credentials and network access
```

## 9. Change Management

### 9.1 Change Log
```markdown
| Date | Change | Author | Version |
|------|---------|---------|----------|
| [Current Date] | Initial Documentation | [System] | 1.0.0 |
```


## 10. Infrastructure and CI/CD

### 10.1 Infrastructure Overview
```markdown
#### Cloud Infrastructure
- **Platform**: Google Cloud Platform (GCP)
- **Primary Services**:
  - Cloud Run (Container hosting)
  - Cloud Build (Container building)
  - Artifact Registry (Container registry)
  - Secret Manager (Secrets management)
  - Cloud Logging (Centralized logging)
  - Cloud Monitoring (Metrics and alerts)

#### Database Infrastructure
- **Primary Database**: PostgreSQL on Neon
  - Host: ep-blue-sun-a8kj1qdc-pooler.eastus2.azure.neon.tech
  - SSL Mode: Required
  - Connection Pooling: Enabled
  - Auto-scaling: Enabled

#### Cache Infrastructure
- **Redis**: Upstash Redis
  - TLS Encryption: Enabled
  - Connection Pooling: Enabled
  - Persistence: Enabled

#### External Services Integration
- Supabase (Authentication)
- Resend (Email delivery)
- Geoapify (Location services)
- Pexels (Image services)
```

### 10.2 CI/CD Pipeline
```markdown
#### GitHub Actions Workflows

1. **Main Deployment (`deploy-cloud-run.yml`)**
   - Trigger: Push to main branch
   - Steps:
     ```yaml
     jobs:
       test:
         # Run unit tests with PostgreSQL and Redis
         services:
           postgres:
             image: postgres:14
           redis:
             image: redis:latest
         steps:
           - Run tests
           - Generate coverage report

       security-scan:
         # Security scanning
         steps:
           - Run Gosec scanner
           - Run Trivy vulnerability scanner
           - Upload SARIF results

       deploy:
         # Deploy to Cloud Run
         needs: [test, security-scan]
         steps:
           - Google Cloud authentication
           - Build container
           - Push to Artifact Registry
           - Deploy to Cloud Run
           - Configure environment variables
           - Set up secrets
     ```

2. **PR Preview Environment (`pr-preview-cloud-run.yml`)**
   - Trigger: Pull request to develop/main
   - Features:
     - Dedicated preview environment per PR
     - Automatic deployment
     - PR comment with preview URL
     - Cleanup on PR close

3. **PR Cleanup (`pr-cleanup-cloud-run.yml`)**
   - Trigger: PR close/merge
   - Actions:
     - Remove preview environment
     - Clean up resources
     - Delete preview URL
```

### 10.3 Environment Configuration
```markdown
#### Production Environment
```yaml
environment:
  name: production
  url: https://api.nomadcrew.uk
  config:
    scaling:
      minInstances: 1
      maxInstances: 10
    resources:
      cpu: 1
      memory: 256Mi
    timeouts:
      http: 300s
```

#### Preview Environment
```yaml
environment:
  name: preview
  url: https://preview.nomadcrew.uk
  config:
    scaling:
      minInstances: 0
      maxInstances: 2
    resources:
      cpu: 0.5
      memory: 128Mi
    timeouts:
      http: 300s
```

### 10.4 Infrastructure Security
```markdown
#### Security Measures
1. **Container Security**
   - Vulnerability scanning (Trivy)
   - Code security scanning (Gosec)
   - Minimal base image
   - Non-root user execution

2. **Network Security**
   - TLS encryption
   - Private VPC
   - Cloud Run IAM
   - Service account least privilege

3. **Secrets Management**
   - GCP Secret Manager integration
   - Environment-specific secrets
   - Automated rotation (where applicable)

4. **Access Control**
   - IAM roles and permissions
   - Service account keys
   - RBAC implementation
```

### 10.5 Monitoring and Alerting
```markdown
#### Monitoring Setup
1. **Infrastructure Metrics**
   - Container instance count
   - CPU/Memory usage
   - Request latency
   - Error rates

2. **Application Metrics**
   - Active WebSocket connections
   - Message throughput
   - Database connection pool
   - Redis cache hit rate

3. **Alert Configurations**
   - High error rate
   - Excessive latency
   - Resource exhaustion
   - Certificate expiration
```

### 10.6 Disaster Recovery
```markdown
#### Backup Strategy
1. **Database Backups**
   - Daily automated backups
   - Point-in-time recovery
   - Cross-region replication

2. **Configuration Backups**
   - Infrastructure as Code (IaC)
   - Environment configurations
   - Secret versioning

#### Recovery Procedures
1. **Service Disruption**
   ```bash
   # Quick rollback to last known good version
   gcloud run services rollback nomadcrew-backend \
     --to-revision=<revision-id> \
     --region=us-east1
   ```

2. **Database Recovery**
   ```bash
   # Restore from backup
   neon database restore \
     --source-timestamp="2024-03-21 10:00:00" \
     --target-database=restored_db
   ```

3. **Cache Recovery**
   ```bash
   # Clear and rebuild cache
   redis-cli -h $REDIS_HOST -p $REDIS_PORT -a $REDIS_PASSWORD FLUSHALL
   ```
```

### 10.7 Scaling Strategy
```markdown
#### Horizontal Scaling
- **Cloud Run**
  - Auto-scaling based on request load
  - Concurrency: 80 requests per instance
  - Maximum instances: 10

- **Database**
  - Connection pooling
  - Read replicas (when needed)
  - Auto-scaling compute units

- **Redis**
  - Connection pooling
  - Cluster mode (when needed)
  - Memory optimization

#### Vertical Scaling
- **Resource Allocation**
  ```yaml
  resources:
    limits:
      cpu: 1
      memory: 256Mi
    requests:
      cpu: 200m
      memory: 128Mi
  ```

- **Performance Tuning**
  - Database query optimization
  - Cache strategy adjustment
  - Connection pool sizing
```
