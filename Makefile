.PHONY: all build test clean lint

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)"

BIN := bin

SIDECARS := jellyfin-sidecar qbittorrent-sidecar raid-sidecar

all: build

build: $(SIDECARS)

$(SIDECARS):
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BIN)/$@ ./cmd/$@

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

# Cross-compile for container builds
build-linux:
	$(foreach sidecar,$(SIDECARS),GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BIN)/$(sidecar) ./cmd/$(sidecar);)
