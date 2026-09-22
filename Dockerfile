# syntax=docker/dockerfile:1
# Multi-stage Dockerfile for Inventario
# Supports both production and testing builds

# Stage 1: Build the React frontend
# Node version pinned to match frontend/package.json's volta.node so the
# in-Docker bundle matches what the macOS e2e lane (which uses the same
# pin via .github/actions/vars) produces for darwin/arm64.
FROM node:26.9-alpine AS frontend-builder

WORKDIR /app/frontend

# .npmrc carries `legacy-peer-deps=true` (openapi-typescript@7's stale TS5
# peer dec — see frontend/.npmrc); without it `npm ci` exits non-zero.
COPY frontend/package*.json frontend/.npmrc ./

# Install dependencies (including devDependencies for build).
# Cache node_modules across builds — invalidated only when package*.json changes
RUN --mount=type=cache,target=/root/.npm \
    npm ci

# Copy frontend source
COPY frontend/ ./

# Build frontend
RUN npm run build

# Stage 2: Base Go environment
FROM golang:1.27.1-alpine AS go-base

# Install common dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    curl

WORKDIR /app

# Copy go mod files
COPY go/go.mod go/go.sum ./go/

# Copy frontend go.mod for dependency resolution. The replace directive in
# go/go.mod points to ../frontend, so the frontend go.mod must be in place
# before `go mod download`.
COPY frontend/go.mod frontend/frontend.go ./frontend/

# Download dependencies into the layer (no cache mount). BuildKit cache
# mounts are daemon-resident and don't persist across CI runners or get
# exported by `cache-to: type=gha`, so a mount here means modules get
# re-downloaded by stage 3 every CI build. Baking /go/pkg/mod into the
# layer lets GHA layer cache reuse it as long as go.mod/go.sum are
# unchanged. The official golang image sets GOPATH=/go.
WORKDIR /app/go
RUN go mod download

# Copy backend source
COPY go/ ./

# Copy built bundle from previous stage
COPY --from=frontend-builder /app/frontend/dist ../frontend/dist/

# Stage 3: Production builder
FROM go-base AS backend-builder

# Set build arguments for version injection
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# Build the application for production with proper tags and ldflags.
# /go/pkg/mod is inherited from go-base (no cache mount, see above).
# /root/.cache/go-build stays a cache mount: it benefits local dev where
# the BuildKit daemon persists; in CI it's a no-op but harmless.
WORKDIR /app/go/cmd/inventario
# The module path comes from go.mod rather than being written out here. The
# linker drops -X for a symbol it cannot find without saying so, so a stale
# path produces a binary that reports "dev" and a release nobody can identify
# — which is how v0.1.0 shipped. Deriving it removes the way to be wrong.
# `version` prints through cobra, which writes to stderr, hence the redirect.
RUN --mount=type=cache,target=/root/.cache/go-build \
    MODULE="$(go list -m)" && \
    CGO_ENABLED=0 GOOS=linux go build \
    -tags with_frontend \
    -ldflags "-X ${MODULE}/internal/version.Version=${VERSION} \
              -X ${MODULE}/internal/version.Commit=${COMMIT} \
              -X ${MODULE}/internal/version.Date=${BUILD_DATE}" \
    -a -installsuffix cgo \
    -o inventario . && \
    ./inventario version 2>&1 | grep -q "^${VERSION} " || \
      { echo "ldflags did not apply: version reports '$(./inventario version 2>&1)', expected '${VERSION}'" >&2; exit 1; }

# Stage 4: Test environment
FROM go-base AS test-runner

# Install additional test dependencies
RUN apk add --no-cache \
    postgresql-client \
    make

# Create test directories
RUN mkdir -p /tmp/test-uploads /app/test-data

# Set working directory for tests
WORKDIR /app/go

# Default command for tests (can be overridden)
CMD ["go", "test", "-v", "./..."]

# Stage 5: Production runtime
# Pinned to an explicit version + multi-arch digest (matches the precision of
# the node:/golang: base pins above) so the runtime base layer is reproducible
# and can't drift to a newly-published `latest`. Renovate's `docker` manager
# keeps the digest current; bump both the tag and the @sha256 together.
FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6 AS production

# Install runtime dependencies. `apk upgrade` first so base-image packages pick
# up security patches at build time even when the pinned digest lags a CVE fix
# — otherwise the image scan (Trivy, #2097) flags a fixed-but-not-yet-repinned
# OS CVE as HIGH and fails the build (e.g. CVE-2026-45447 in libssl3/libcrypto3,
# fixed in openssl 3.5.7-r0). Keeps the digest pin for the base layer while
# staying current on patches; --no-cache leaves no apk index behind.
RUN apk --no-cache upgrade && apk --no-cache add ca-certificates tzdata curl

# Create non-root user
RUN addgroup -g 1001 -S inventario && \
    adduser -u 1001 -S inventario -G inventario

# Create directories
RUN mkdir -p /app/uploads /app/data && \
    chown -R inventario:inventario /app

WORKDIR /app

# Copy binary from builder stage
COPY --chown=inventario:inventario --from=backend-builder /app/go/cmd/inventario/inventario /usr/local/bin/inventario

# Switch to non-root user
USER inventario

# Expose port
EXPOSE 3333

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:3333/api/v1/settings || exit 1

# Default command
ENTRYPOINT ["/usr/local/bin/inventario"]
CMD ["run"]
