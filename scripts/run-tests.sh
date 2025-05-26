#!/bin/bash

# Test runner script with automatic cleanup
# Usage: ./scripts/run-tests.sh [test-service-name]
# Examples:
#   ./scripts/run-tests.sh                          # Run all tests
#   ./scripts/run-tests.sh inventario-test          # Run all tests
#   ./scripts/run-tests.sh inventario-test-postgres  # Run only PostgreSQL tests

set -e

TEST_SERVICE="${1:-inventario-test}"
NO_CLEANUP="${NO_CLEANUP:-false}"

# Function to cleanup on exit
cleanup() {
    if [ "$NO_CLEANUP" != "true" ]; then
        echo ""
        echo "🧹 Running cleanup..."
        "$(dirname "$0")/test-cleanup.sh"
    fi
}

# Register cleanup function to run on script exit
trap cleanup EXIT

# Handle interruptions (Ctrl+C, etc.)
trap 'echo -e "\n⚠️  Test interrupted. Running cleanup..."; cleanup; exit 1' INT TERM

echo "🚀 Starting test environment..."

# Ensure clean state before starting
echo "Ensuring clean state..."
"$(dirname "$0")/test-cleanup.sh"

echo "Building and starting test services..."
docker compose --profile test build

echo "Running tests with service: $TEST_SERVICE"

# Run the specified test service
if docker compose --profile test run --rm "$TEST_SERVICE"; then
    echo "✅ Tests completed successfully!"
    exit_code=0
else
    echo "❌ Tests failed with exit code: $?"
    exit_code=1
fi

exit $exit_code
