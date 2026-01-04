.PHONY: all build test clean raid-inhibitor jellyfin-inhibitor container-raid container-jellyfin

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)"

all: build

build: raid-inhibitor jellyfin-inhibitor

raid-inhibitor:
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/raid-inhibitor ./cmd/raid-inhibitor

jellyfin-inhibitor:
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/jellyfin-inhibitor ./cmd/jellyfin-inhibitor

test:
	go test -v ./...

clean:
	rm -rf bin/

# Container builds
container-raid:
	podman build -t localhost/raid-inhibitor:latest -f deploy/Containerfile.raid .

container-jellyfin:
	podman build -t localhost/jellyfin-inhibitor:latest -f deploy/Containerfile.jellyfin .

containers: container-raid container-jellyfin
