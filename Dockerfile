FROM golang:1.24 AS builder

WORKDIR /app

COPY ./go.mod ./go.sum ./
RUN go mod download

COPY ./ .
COPY config.development.yaml ./config/config.development.yaml
COPY config.production.yaml ./config/config.production.yaml
RUN go build -o nomadcrew-backend

FROM golang:1.23

COPY --from=builder /app/nomadcrew-backend /nomadcrew-backend
COPY --from=builder /app/config ./config

# Set default values for non-sensitive configurations
ENV PORT=8080

CMD ["/nomadcrew-backend"]