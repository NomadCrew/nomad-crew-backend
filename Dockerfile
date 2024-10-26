# Stage 1: Build the application
FROM golang:1.21 AS builder
WORKDIR /app
# Copy go mod and sum files
COPY ./user-service/go.mod ./user-service/go.sum ./
# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download
# Copy the source code into the container
COPY ./user-service .
# Build the Go app
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o nomadcrew-backend .

# Stage 2: Setup runtime container
# Use a smaller, more secure base image
FROM alpine:latest  
RUN apk --no-cache add ca-certificates nginx
WORKDIR /root/
# Copy the Pre-built binary file from the previous stage
COPY --from=builder /app/nomadcrew-backend .
# Copy necessary secrets
COPY ./user-service/secrets/serviceAccountKey.json /secrets/serviceAccountKey.json
# Copy mkcert SSL certificates
COPY ./localhost+2.pem /etc/nginx/ssl/localhost+2.pem
COPY ./localhost+2-key.pem /etc/nginx/ssl/localhost+2-key.pem
# Copy nginx configuration file
COPY ./nginx.conf /etc/nginx/nginx.conf
# Expose port 443 to the Docker host, so we can access it from the outside
EXPOSE 443
# Start nginx and the Go app
CMD ["sh", "-c", "nginx && ./nomadcrew-backend"]
