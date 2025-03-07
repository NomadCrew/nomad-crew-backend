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

# Create minimal config files directly using printf to ensure proper YAML formatting
RUN printf "server:\n\
  environment: %s\n\
  port: \"8080\"\n\
  allowed_origins:\n\
    - \"*\"\n\
  jwt_secret_key: %s\n\
  frontend_url: %s\n\
  log_level: debug\n\
\n\
database:\n\
  host: postgres\n\
  port: 5432\n\
  user: postgres\n\
  password: %s\n\
  name: nomadcrew\n\
  max_connections: 20\n\
  ssl_mode: disable\n\
\n\
redis:\n\
  address: redis:6379\n\
  password: %s\n\
  db: 0\n\
\n\
email:\n\
  from_address: %s\n\
  from_name: %s\n\
  resend_api_key: %s\n\
\n\
external_services:\n\
  geoapify_key: %s\n\
  pexels_api_key: %s\n\
  supabase_anon_key: %s\n\
  supabase_service_key: %s\n\
  supabase_url: %s\n\
  supabase_jwt_secret: %s\n\
  jwt_secret_key: %s\n" \
  "${SERVER_ENVIRONMENT:-development}" \
  "${JWT_SECRET_KEY}" \
  "${FRONTEND_URL:-https://nomadcrew.uk}" \
  "${DB_PASSWORD}" \
  "${REDIS_PASSWORD}" \
  "${EMAIL_FROM_ADDRESS:-welcome@nomadcrew.uk}" \
  "${EMAIL_FROM_NAME:-NomadCrew}" \
  "${RESEND_API_KEY}" \
  "${GEOAPIFY_KEY}" \
  "${PEXELS_API_KEY}" \
  "${SUPABASE_ANON_KEY}" \
  "${SUPABASE_SERVICE_KEY}" \
  "${SUPABASE_URL}" \
  "${SUPABASE_JWT_SECRET}" \
  "${JWT_SECRET_KEY}" > config/config.development.yaml

# Create production config by copying development and updating environment
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