# syntax=docker/dockerfile:1
# Multi-stage Dockerfile for Inventario
# Supports both production and testing builds

# Stage 1: Build legacy Vue frontend
# Node version pinned to match frontend/package.json's volta.node so the
# in-Docker bundle matches what the macOS e2e lane (which uses the same
# pin via .github/actions/vars) produces for darwin/arm64.
FROM node:24.14.1-alpine AS frontend-builder

WORKDIR /app/frontend

# Copy package files
COPY frontend/package*.json ./

# Install dependencies (including devDependencies for build)
# Cache node_modules across builds — invalidated only when package*.json changes
RUN --mount=type=cache,target=/root/.npm \
    npm ci

# Copy frontend source
COPY frontend/ ./

# Build frontend
RUN npm run build

# Stage 1b: Build the new React frontend
# Both bundles ship in the image during the dual-frontend migration window
# (see epic #1397). The active bundle is selected at runtime by the env var
# INVENTARIO_FRONTEND={legacy|new} once the dual-bundle handler lands (#1401).
FROM node:24.14.1-alpine AS frontend-react-builder

WORKDIR /app/frontend-react

# .npmrc carries `legacy-peer-deps=true` (openapi-typescript@7's stale TS5
# peer dec — see frontend-react/.npmrc); without it `npm ci` exits non-zero.
COPY frontend-react/package*.json frontend-react/.npmrc ./

RUN --mount=type=cache,target=/root/.npm \
    npm ci

COPY frontend-react/ ./

RUN npm run build

# Stage 2: Base Go environment
FROM golang:1.26.2-alpine AS go-base

# Install common dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    curl

WORKDIR /app

# Copy go mod files
COPY go/go.mod go/go.sum ./go/

# Copy frontend go.mod files for dependency resolution (legacy + React).
# Replace directives in go/go.mod point to ../frontend and ../frontend-react,
# so both go.mod files must be in place before `go mod download`.
COPY frontend/go.mod frontend/frontend.go ./frontend/
COPY frontend-react/go.mod frontend-react/frontend.go ./frontend-react/

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

# Copy built bundles from previous stages
COPY --from=frontend-builder /app/frontend/dist ../frontend/dist/
COPY --from=frontend-react-builder /app/frontend-react/dist ../frontend-react/dist/

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
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
    -tags with_frontend \
    -ldflags "-X github.com/denisvmedia/inventario/internal/version.Version=${VERSION} \
              -X github.com/denisvmedia/inventario/internal/version.Commit=${COMMIT} \
              -X github.com/denisvmedia/inventario/internal/version.Date=${BUILD_DATE}" \
    -a -installsuffix cgo \
    -o inventario .

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
FROM alpine:latest AS production

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata curl

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
