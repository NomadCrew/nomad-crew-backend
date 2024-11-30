FROM golang:1.21 AS builder

WORKDIR /app

COPY ./go.mod ./go.sum ./
RUN go mod download

COPY ./ .
RUN go build -o nomadcrew-backend

FROM golang:1.23

COPY --from=builder /app/nomadcrew-backend /nomadcrew-backend

# Set default values for non-sensitive configurations
ENV PORT=8080

CMD ["/nomadcrew-backend"]