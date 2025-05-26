#!/bin/bash

# Docker Test Runner Script for Unix/Linux/macOS
# This script helps run tests using Docker Compose

set -e

# Default values
TEST_TYPE="all"
BUILD=false
CLEAN=false
LOGS=false

# Function to show help
show_help() {
    cat << EOF
Docker Test Runner Script

This script helps run tests for the Inventario application using Docker Compose.

USAGE:
    ./scripts/docker-test-run.sh [OPTIONS]

OPTIONS:
    -t, --type <type>   Type of tests to run: 'all', 'postgresql', 'go' (default: all)
    -b, --build         Build the test Docker image before running tests
    -c, --clean         Clean up test containers and images after running
    -l, --logs          Show logs from test containers
    -h, --help          Show this help message

EXAMPLES:
    # Run all tests
    ./scripts/docker-test-run.sh

    # Run only PostgreSQL tests
    ./scripts/docker-test-run.sh --type postgresql

    # Build and run tests, then clean up
    ./scripts/docker-test-run.sh --build --clean

    # Run tests and show logs
    ./scripts/docker-test-run.sh --logs

REQUIREMENTS:
    - Docker and Docker Compose must be installed
    - Bash 4.0 or later

EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -t|--type)
            TEST_TYPE="$2"
            shift 2
            ;;
        -b|--build)
            BUILD=true
            shift
            ;;
        -c|--clean)
            CLEAN=true
            shift
            ;;
        -l|--logs)
            LOGS=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            echo "❌ Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed or not in PATH"
    exit 1
fi

# Check if Docker Compose is available
if ! command -v docker-compose &> /dev/null; then
    echo "❌ Docker Compose is not installed or not in PATH"
    exit 1
fi

echo "🐳 Docker Test Runner for Inventario"
echo "===================================="

# Build test image if requested
if [ "$BUILD" = true ]; then
    echo "🔨 Building test Docker image..."
    docker build --target test-runner -t inventario:test .
    echo "✅ Test image built successfully"
fi

# Start test database
echo "🚀 Starting test database..."
docker-compose --profile test up -d postgres-test

# Wait for database to be ready
echo "⏳ Waiting for database to be ready..."
max_attempts=30
attempt=0

while [ $attempt -lt $max_attempts ]; do
    attempt=$((attempt + 1))
    sleep 2

    if docker-compose --profile test ps postgres-test --format json | jq -r '.[0].Health' | grep -q "healthy"; then
        break
    fi

    if [ $attempt -ge $max_attempts ]; then
        echo "❌ Database failed to become ready within timeout"
        docker-compose --profile test logs postgres-test
        exit 1
    fi
done

echo "✅ Database is ready"

# Run tests based on type
case "${TEST_TYPE,,}" in
    postgresql)
        echo "🧪 Running PostgreSQL tests..."
        docker-compose --profile test run --rm inventario-test-postgresql
        ;;
    go)
        echo "🧪 Running Go tests..."
        docker-compose --profile test run --rm inventario-test
        ;;
    all)
        echo "🧪 Running all tests..."
        docker-compose --profile test run --rm inventario-test
        ;;
    *)
        echo "❌ Invalid test type: $TEST_TYPE. Valid options: all, postgres, go"
        exit 1
        ;;
esac

test_exit_code=$?

# Show logs if requested
if [ "$LOGS" = true ]; then
    echo "📋 Test logs:"
    docker-compose --profile test logs
fi

# Clean up if requested
if [ "$CLEAN" = true ]; then
    echo "🧹 Cleaning up test containers..."
    docker-compose --profile test down -v
    docker rmi inventario:test 2>/dev/null || true
    echo "✅ Cleanup completed"
else
    # Just stop the services
    docker-compose --profile test down
fi

# Report results
if [ $test_exit_code -eq 0 ]; then
    echo "🎉 Tests completed successfully!"
else
    echo "❌ Tests failed with exit code: $test_exit_code"
fi

exit $test_exit_code
