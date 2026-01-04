.PHONY: all build test clean health-check health-inhibitor lint

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)"

# Output directory
BIN := bin

all: build

build: health-check health-inhibitor

health-check:
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BIN)/health-check ./cmd/health-check

health-inhibitor:
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BIN)/health-inhibitor ./cmd/health-inhibitor

test:
	go test -v -race ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	go vet ./...
	@which staticcheck >/dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed, skipping"

clean:
	rm -rf $(BIN)/ coverage.out coverage.html

# Install to system (for local development)
install: build
	sudo cp $(BIN)/health-check /usr/local/bin/
	sudo cp $(BIN)/health-inhibitor /usr/local/bin/

# Cross-compile for container builds
build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BIN)/health-check ./cmd/health-check
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BIN)/health-inhibitor ./cmd/health-inhibitor
