.PHONY: test build clean lint

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=nomadcrew-backend

# Linting
GOLINT=golangci-lint

all: test build

build:
	$(GOBUILD) -o $(BINARY_NAME) -v .

test:
	$(GOTEST) -v -race ./...

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

run:
	$(GOBUILD) -o $(BINARY_NAME) -v ./...
	./$(BINARY_NAME)

lint:
	$(GOLINT) run

deps:
	$(GOMOD) download

tidy:
	$(GOMOD) tidy

test-coverage:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Integration tests specifically
test-integration:
	$(GOTEST) -v -tags=integration ./tests/integration/...

# Development tasks
dev:
	air

.PHONY: migrate-up migrate-down
migrate-up:
	migrate -path db/migrations -database "$(DB_CONNECTION_STRING)" up

migrate-down:
	migrate -path db/migrations -database "$(DB_CONNECTION_STRING)" down