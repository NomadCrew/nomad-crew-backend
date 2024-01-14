FROM golang:1.15

WORKDIR /go/src/app

COPY . .

RUN go get -d -v ./...

RUN go install -v ./...

EXPOSE 8080

ENV GIN_MODE release

CMD ["app"]
