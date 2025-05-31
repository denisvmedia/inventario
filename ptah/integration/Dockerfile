# Multi-stage build for Ptah integration tests
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /src

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the integration test binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/ptah-integration-test ./cmd/integration-test

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates

# Create app directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/ptah-integration-test .

# Create directories for reports and fixtures
RUN mkdir -p /app/reports /app/fixtures

# Set executable permissions
RUN chmod +x /app/ptah-integration-test

# Default command
ENTRYPOINT ["/app/ptah-integration-test"]
