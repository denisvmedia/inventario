# Multi-stage Dockerfile for Inventario

# Stage 1: Build frontend
FROM node:22.16.0-alpine AS frontend-builder

WORKDIR /app/frontend

# Copy package files
COPY frontend/package*.json ./

# Install dependencies (including devDependencies for build)
RUN npm ci

# Copy frontend source
COPY frontend/ ./

# Build frontend
RUN npm run build

# Stage 2: Build backend
FROM golang:1.24.1-alpine AS backend-builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

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

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o inventario .

# Stage 3: Runtime
FROM alpine:latest

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
COPY --from=backend-builder /app/go/inventario .

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
