FROM golang:1.21 AS builder

WORKDIR /app

COPY ./user-service/go.mod ./user-service/go.sum ./
RUN go mod download

COPY ./user-service .
RUN go build -o nomadcrew-backend

FROM golang:1.21

COPY --from=builder /app/nomadcrew-backend /nomadcrew-backend

# Set default values for non-sensitive configurations
ENV PORT=8080

CMD ["/nomadcrew-backend"]