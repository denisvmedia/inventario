# Multi-stage Dockerfile for Inventario
# Supports both production and testing builds

# Stage 1: Build frontend
FROM node:24.12.0-alpine AS frontend-builder

WORKDIR /app/frontend

# Copy package files
COPY frontend/package*.json ./

# Install dependencies (including devDependencies for build)
RUN npm ci

# Copy frontend source
COPY frontend/ ./

# Build frontend
RUN npm run build

# Stage 2: Base Go environment
FROM golang:1.25.6-alpine AS go-base

# Install common dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    curl

WORKDIR /app

# Copy go mod files
COPY go/go.mod go/go.sum ./go/

# Copy frontend go.mod for dependency resolution
COPY frontend/go.mod frontend/frontend.go ./frontend/

# Download dependencies
WORKDIR /app/go
RUN go mod download

# Copy backend source
COPY go/ ./

# Copy built frontend from previous stage
COPY --from=frontend-builder /app/frontend/dist ../frontend/dist/

# Stage 3: Production builder
FROM go-base AS backend-builder

# Set build arguments for version injection
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# Build the application for production with proper tags and ldflags
WORKDIR /app/go/cmd/inventario
RUN CGO_ENABLED=0 GOOS=linux go build \
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
COPY --from=backend-builder /app/go/cmd/inventario/inventario .

# Change ownership
RUN chown inventario:inventario inventario

# Switch to non-root user
USER inventario

# Expose port
EXPOSE 3333

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:3333/api/v1/settings || exit 1

# Default command
CMD ["./inventario", "run"]
