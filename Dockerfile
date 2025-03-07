FROM golang:1.24 AS builder

WORKDIR /app

# Add build arguments
ARG SERVER_ENVIRONMENT
ARG JWT_SECRET_KEY
ARG DB_PASSWORD
ARG REDIS_PASSWORD
ARG RESEND_API_KEY
ARG GEOAPIFY_KEY
ARG PEXELS_API_KEY
ARG SUPABASE_ANON_KEY
ARG SUPABASE_SERVICE_KEY
ARG SUPABASE_URL
ARG SUPABASE_JWT_SECRET
ARG EMAIL_FROM_ADDRESS
ARG EMAIL_FROM_NAME
ARG FRONTEND_URL
ARG ALLOWED_ORIGINS

# Set environment variables from build args
ENV SERVER_ENVIRONMENT=${SERVER_ENVIRONMENT}
ENV JWT_SECRET_KEY=${JWT_SECRET_KEY}
ENV DB_PASSWORD=${DB_PASSWORD}
ENV REDIS_PASSWORD=${REDIS_PASSWORD}
ENV RESEND_API_KEY=${RESEND_API_KEY}
ENV GEOAPIFY_KEY=${GEOAPIFY_KEY}
ENV PEXELS_API_KEY=${PEXELS_API_KEY}
ENV SUPABASE_ANON_KEY=${SUPABASE_ANON_KEY}
ENV SUPABASE_SERVICE_KEY=${SUPABASE_SERVICE_KEY}
ENV SUPABASE_URL=${SUPABASE_URL}
ENV SUPABASE_JWT_SECRET=${SUPABASE_JWT_SECRET}
ENV EMAIL_FROM_ADDRESS=${EMAIL_FROM_ADDRESS}
ENV EMAIL_FROM_NAME=${EMAIL_FROM_NAME}
ENV FRONTEND_URL=${FRONTEND_URL}
ENV ALLOWED_ORIGINS=${ALLOWED_ORIGINS}
# Explicitly set CI=true for the container build environment
ENV CI=true

# Copy go.mod and go.sum first for better caching
COPY ./go.mod ./go.sum ./
RUN go mod download

# Copy the rest of the code
COPY ./ .

# Build the config generator
RUN go build -o generate-config ./scripts/generate_config.go

# Create config directory
RUN mkdir -p config

# Instead of running the generate-config script, create minimal config files directly
RUN echo "Creating minimal config files for CI build" && \
    cat > config/config.development.yaml << EOF
server:
  environment: ${SERVER_ENVIRONMENT:-development}
  port: "8080"
  allowed_origins:
    - "*"
  jwt_secret_key: ${JWT_SECRET_KEY}
  frontend_url: ${FRONTEND_URL:-https://nomadcrew.uk}
  log_level: debug

database:
  host: postgres
  port: 5432
  user: postgres
  password: ${DB_PASSWORD}
  name: nomadcrew
  max_connections: 20
  ssl_mode: disable

redis:
  address: redis:6379
  password: ${REDIS_PASSWORD}
  db: 0

email:
  from_address: ${EMAIL_FROM_ADDRESS:-welcome@nomadcrew.uk}
  from_name: ${EMAIL_FROM_NAME:-NomadCrew}
  resend_api_key: ${RESEND_API_KEY}

external_services:
  geoapify_key: ${GEOAPIFY_KEY}
  pexels_api_key: ${PEXELS_API_KEY}
  supabase_anon_key: ${SUPABASE_ANON_KEY}
  supabase_service_key: ${SUPABASE_SERVICE_KEY}
  supabase_url: ${SUPABASE_URL}
  supabase_jwt_secret: ${SUPABASE_JWT_SECRET}
  jwt_secret_key: ${JWT_SECRET_KEY}
EOF

RUN cp config/config.development.yaml config/config.production.yaml && \
    sed -i 's/development/production/g' config/config.production.yaml

# Verify config files were created
RUN ls -la config/
RUN cat config/config.development.yaml | grep -v password | grep -v key
RUN cat config/config.production.yaml | grep -v password | grep -v key

# Build the main application
RUN go build -o nomadcrew-backend

FROM golang:1.23

WORKDIR /app

# Copy the build artifacts
COPY --from=builder /app/nomadcrew-backend /app/nomadcrew-backend
COPY --from=builder /app/config /app/config

# Set environment variables for runtime
ENV SERVER_ENVIRONMENT=${SERVER_ENVIRONMENT}
ENV JWT_SECRET_KEY=${JWT_SECRET_KEY}
ENV DB_PASSWORD=${DB_PASSWORD}
ENV REDIS_PASSWORD=${REDIS_PASSWORD}
ENV RESEND_API_KEY=${RESEND_API_KEY}
ENV GEOAPIFY_KEY=${GEOAPIFY_KEY}
ENV PEXELS_API_KEY=${PEXELS_API_KEY}
ENV SUPABASE_ANON_KEY=${SUPABASE_ANON_KEY}
ENV SUPABASE_SERVICE_KEY=${SUPABASE_SERVICE_KEY}
ENV SUPABASE_URL=${SUPABASE_URL}
ENV SUPABASE_JWT_SECRET=${SUPABASE_JWT_SECRET}
ENV EMAIL_FROM_ADDRESS=${EMAIL_FROM_ADDRESS}
ENV EMAIL_FROM_NAME=${EMAIL_FROM_NAME}
ENV FRONTEND_URL=${FRONTEND_URL}
ENV ALLOWED_ORIGINS=${ALLOWED_ORIGINS}
ENV PORT=8080
# Keep CI environment variable in runtime container
ENV CI=true
# Add AWS secrets path for production environment
ENV AWS_SECRETS_PATH=${AWS_SECRETS_PATH:-/nomadcrew/secrets}

EXPOSE 8080
CMD ["/app/nomadcrew-backend"]