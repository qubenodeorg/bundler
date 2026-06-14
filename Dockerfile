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

# Prepare a writable data directory for the distroless nonroot user (uid 65532).
RUN mkdir -p /data && chown -R 65532:65532 /data

# ---- Runtime stage ----
# distroless removes the package-manager step (and its cache) and ships with
# ca-certificates already installed. The cc variant includes the C libraries
# required by the CGO-enabled binary. The nonroot tag runs as uid 65532 and
# contains no shell or utilities, so the HEALTHCHECK uses a built-in command.
FROM gcr.io/distroless/cc-debian12:nonroot

WORKDIR /app

COPY --from=builder --chown=65532:65532 /app/qubenode-bundler /app/qubenode-bundler
COPY --from=builder --chown=65532:65532 /data /data

USER nonroot:nonroot

# Persist BadgerDB to a mountable volume rather than ephemeral /tmp.
ENV ERC4337_BUNDLER_DATA_DIRECTORY=/data
VOLUME ["/data"]

EXPOSE 4337

# /live returns 200 once the HTTP server is up (no upstream dependencies).
# A built-in healthcheck subcommand is used because the distroless image has no
# shell or wget.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/app/qubenode-bundler", "healthcheck"]

ENTRYPOINT ["/app/qubenode-bundler"]
CMD ["start", "--mode", "private"]
