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

# Copy go.mod and go.sum first for better caching
COPY ./go.mod ./go.sum ./
RUN go mod download

# Copy the rest of the code
COPY ./ .

# Create .env file for the config generator
RUN echo "Creating .env file in container..." && \
    echo "SERVER_ENVIRONMENT=${SERVER_ENVIRONMENT}" > .env && \
    echo "JWT_SECRET_KEY=${JWT_SECRET_KEY}" >> .env && \
    echo "DB_PASSWORD=${DB_PASSWORD}" >> .env && \
    echo "REDIS_PASSWORD=${REDIS_PASSWORD}" >> .env && \
    echo "RESEND_API_KEY=${RESEND_API_KEY}" >> .env && \
    echo "GEOAPIFY_KEY=${GEOAPIFY_KEY}" >> .env && \
    echo "PEXELS_API_KEY=${PEXELS_API_KEY}" >> .env && \
    echo "SUPABASE_ANON_KEY=${SUPABASE_ANON_KEY}" >> .env && \
    echo "SUPABASE_SERVICE_KEY=${SUPABASE_SERVICE_KEY}" >> .env && \
    echo "SUPABASE_URL=${SUPABASE_URL}" >> .env && \
    echo "SUPABASE_JWT_SECRET=${SUPABASE_JWT_SECRET}" >> .env && \
    echo "EMAIL_FROM_ADDRESS=${EMAIL_FROM_ADDRESS}" >> .env && \
    echo "EMAIL_FROM_NAME=${EMAIL_FROM_NAME}" >> .env && \
    echo "FRONTEND_URL=${FRONTEND_URL}" >> .env && \
    echo "ALLOWED_ORIGINS=${ALLOWED_ORIGINS}" >> .env && \
    echo "Contents of .env file (redacted):" && \
    cat .env | sed 's/=.*/=REDACTED/'

# Debug: Print that we created the .env file
RUN echo "Created .env file for config generation"

# Build the config generator
RUN go build -o generate-config ./scripts/generate_config.go

# Generate config files for both environments
RUN ./generate-config development
RUN ./generate-config production

# Verify config files were created
RUN ls -la config/
RUN cat config/config.development.yml | grep -v password | grep -v key
RUN cat config/config.production.yml | grep -v password | grep -v key

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

EXPOSE 8080
CMD ["/app/nomadcrew-backend"]