FROM golang:1.21 AS builder

WORKDIR /app

COPY ./user-service/go.mod ./user-service/go.sum ./
RUN go mod download

COPY ./user-service .
COPY ./user-service/config.json ./

RUN go build -o nomadcrew-backend

FROM golang:1.21

COPY --from=builder /app/nomadcrew-backend /nomadcrew-backend
COPY --from=builder /app/config.json /config.json

CMD ["/nomadcrew-backend"]
