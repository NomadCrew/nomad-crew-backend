FROM golang:1.24-alpine AS builder

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
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy the binary from the builder stage
COPY --from=builder /app/nomadcrew-backend /app/nomadcrew-backend

# Create a non-root user to run the application
RUN adduser -D -g '' appuser
USER appuser

# Explicitly tell Cloud Run the container listens on this port
ENV PORT=8080
EXPOSE 8080

# Start command
CMD ["/app/nomadcrew-backend"]