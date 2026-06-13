# syntax=docker/dockerfile:1

# ---- Build stage ----
# bookworm (Debian 12) is current LTS; buster (Debian 10) is EOL and no longer
# receives security updates.
FROM golang:1.20-bookworm AS builder

WORKDIR /app

# Download dependencies first so this layer is cached unless go.mod/go.sum change.
COPY go.* ./
RUN go mod download

# Copy the rest of the source and build.
COPY . ./

# -trimpath strips local paths; -s -w drop the symbol/DWARF tables for a smaller
# binary. VERSION can be overridden at build time: --build-arg VERSION=v1.2.3
ARG VERSION=dev
RUN CGO_ENABLED=1 go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /app/qubenode-bundler

# ---- Runtime stage ----
FROM debian:bookworm-slim

# ca-certificates for outbound TLS (RPC providers); wget is used by HEALTHCHECK.
RUN set -eux; \
    apt-get update; \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        ca-certificates \
        wget; \
    rm -rf /var/lib/apt/lists/*

# Run as an unprivileged user. The data directory is owned by that user so
# BadgerDB can persist the mempool across restarts.
RUN groupadd --system --gid 10001 bundler \
    && useradd --system --uid 10001 --gid bundler --home-dir /app --no-create-home bundler \
    && mkdir -p /app /data \
    && chown -R bundler:bundler /app /data

COPY --from=builder --chown=bundler:bundler /app/qubenode-bundler /app/qubenode-bundler

USER bundler
WORKDIR /app

# Persist BadgerDB to a mountable volume rather than ephemeral /tmp.
ENV ERC4337_BUNDLER_DATA_DIRECTORY=/data
VOLUME ["/data"]

EXPOSE 4337

# /live returns 200 once the HTTP server is up (no upstream dependencies).
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://127.0.0.1:4337/live || exit 1

ENTRYPOINT ["/app/qubenode-bundler"]
CMD ["start", "--mode", "private"]
