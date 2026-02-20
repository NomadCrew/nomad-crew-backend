FROM golang:1.24-alpine3.21 AS builder

WORKDIR /app

# Copy go.mod and go.sum first for better caching
COPY ./go.mod ./go.sum ./
RUN go mod download

# Copy the rest of the code
COPY ./ .

# Build the application (Pass build args as needed)
ARG VERSION=dev
ARG SERVER_ENVIRONMENT=development
RUN CGO_ENABLED=0 go build -ldflags "-X main.Version=${VERSION} -X main.Environment=${SERVER_ENVIRONMENT}" -o nomadcrew-backend

# Use a small image for the final container
FROM alpine:3.21

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy the binary from the builder stage
COPY --from=builder /app/nomadcrew-backend /app/nomadcrew-backend

# Create a non-root user to run the application
RUN adduser -D -g '' appuser

# Wallet file storage directory (used in local mode; R2 mode stores remotely)
RUN mkdir -p /var/data/wallet-files && chown appuser:appuser /var/data/wallet-files

USER appuser

# Explicitly tell Cloud Run the container listens on this port
ENV PORT=8080
EXPOSE 8080

# Start command
CMD ["/app/nomadcrew-backend"]