# Multi-stage build for homelab sidecars
FROM docker.io/library/golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build
COPY . .
RUN go mod download

# Build all sidecars
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /jellyfin-sidecar ./cmd/jellyfin-sidecar
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /qbittorrent-sidecar ./cmd/qbittorrent-sidecar
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /raid-sidecar ./cmd/raid-sidecar

# Jellyfin sidecar image
FROM scratch AS jellyfin-sidecar
COPY --from=builder /jellyfin-sidecar /sidecar
ENTRYPOINT ["/sidecar"]

# qBittorrent sidecar image
FROM scratch AS qbittorrent-sidecar
COPY --from=builder /qbittorrent-sidecar /sidecar
ENTRYPOINT ["/sidecar"]

# RAID sidecar image (needs /proc/mdstat access - host only)
FROM scratch AS raid-sidecar
COPY --from=builder /raid-sidecar /sidecar
ENTRYPOINT ["/sidecar"]

# Default: all sidecars in one image
FROM alpine:3.20 AS default
COPY --from=builder /jellyfin-sidecar /usr/bin/
COPY --from=builder /qbittorrent-sidecar /usr/bin/
COPY --from=builder /raid-sidecar /usr/bin/
