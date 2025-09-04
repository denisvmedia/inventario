# CI Integration Test Runner (PowerShell)
# This script runs the CLI workflow integration test that validates the complete
# workflow from fresh database setup through CLI operations to API access.

param(
    [string]$PostgresDSN = $env:POSTGRES_TEST_DSN
)

Write-Host "🚀 Starting CLI Workflow Integration Test" -ForegroundColor Green
Write-Host "==========================================" -ForegroundColor Green

# Check if PostgreSQL DSN is provided
if (-not $PostgresDSN) {
    Write-Host "❌ Error: POSTGRES_TEST_DSN environment variable is required" -ForegroundColor Red
    Write-Host "   Example: `$env:POSTGRES_TEST_DSN='postgres://user:password@localhost/test_db?sslmode=disable'" -ForegroundColor Yellow
    exit 1
}

Write-Host "📊 Database: $PostgresDSN" -ForegroundColor Cyan
Write-Host ""

# Set environment variable for the test
$env:POSTGRES_TEST_DSN = $PostgresDSN

# Run the integration test
Write-Host "🧪 Running CLI workflow integration test..." -ForegroundColor Blue
try {
    go test -tags=integration ./integration_test/cli_workflow_integration_test.go -v -timeout=5m
    
    Write-Host ""
    Write-Host "✅ CLI workflow integration test completed successfully!" -ForegroundColor Green
    Write-Host "🎉 The complete workflow from CLI setup to API access is working correctly." -ForegroundColor Green
}
catch {
    Write-Host ""
    Write-Host "❌ Integration test failed!" -ForegroundColor Red
    Write-Host $_.Exception.Message -ForegroundColor Red
    exit 1
}
