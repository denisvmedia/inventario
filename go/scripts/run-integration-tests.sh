#!/bin/bash

# CI Integration Test Runner
# This script runs the CLI workflow integration test that validates the complete
# workflow from fresh database setup through CLI operations to API access.

set -e

echo "🚀 Starting CLI Workflow Integration Test"
echo "=========================================="

# Check if PostgreSQL DSN is provided
if [ -z "$POSTGRES_TEST_DSN" ]; then
    echo "❌ Error: POSTGRES_TEST_DSN environment variable is required"
    echo "   Example: export POSTGRES_TEST_DSN='postgres://user:password@localhost/test_db?sslmode=disable'"
    exit 1
fi

echo "📊 Database: $POSTGRES_TEST_DSN"
echo ""

# Run the integration test
echo "🧪 Running CLI workflow integration test..."
go test -tags=integration ./integration_test/cli_workflow_integration_test.go -v -timeout=5m

echo ""
echo "✅ CLI workflow integration test completed successfully!"
echo "🎉 The complete workflow from CLI setup to API access is working correctly."
