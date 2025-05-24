# Docker Test Runner Script for Windows
# This script helps run tests using Docker Compose

param(
    [string]$TestType = "all",
    [switch]$Build,
    [switch]$Clean,
    [switch]$Logs,
    [switch]$Help
)

if ($Help) {
    Write-Host @"
Docker Test Runner Script

This script helps run tests for the Inventario application using Docker Compose.

USAGE:
    .\scripts\docker-test-run.ps1 [OPTIONS]

OPTIONS:
    -TestType <type>    Type of tests to run: 'all', 'postgresql', 'go' (default: all)
    -Build              Build the test Docker image before running tests
    -Clean              Clean up test containers and images after running
    -Logs               Show logs from test containers
    -Help               Show this help message

EXAMPLES:
    # Run all tests
    .\scripts\docker-test-run.ps1

    # Run only PostgreSQL tests
    .\scripts\docker-test-run.ps1 -TestType postgresql

    # Build and run tests, then clean up
    .\scripts\docker-test-run.ps1 -Build -Clean

    # Run tests and show logs
    .\scripts\docker-test-run.ps1 -Logs

REQUIREMENTS:
    - Docker and Docker Compose must be installed
    - PowerShell 5.0 or later

"@
    exit 0
}

# Check if Docker is available
if (!(Get-Command docker -ErrorAction SilentlyContinue)) {
    Write-Host "❌ Docker is not installed or not in PATH" -ForegroundColor Red
    exit 1
}

# Check if Docker Compose is available
if (!(Get-Command docker-compose -ErrorAction SilentlyContinue)) {
    Write-Host "❌ Docker Compose is not installed or not in PATH" -ForegroundColor Red
    exit 1
}

Write-Host "🐳 Docker Test Runner for Inventario" -ForegroundColor Cyan
Write-Host "====================================" -ForegroundColor Cyan

# Build test image if requested
if ($Build) {
    Write-Host "🔨 Building test Docker image..." -ForegroundColor Yellow
    docker build --target test-runner -t inventario:test .
    if ($LASTEXITCODE -ne 0) {
        Write-Host "❌ Failed to build test image" -ForegroundColor Red
        exit 1
    }
    Write-Host "✅ Test image built successfully" -ForegroundColor Green
}

# Start test database
Write-Host "🚀 Starting test database..." -ForegroundColor Yellow
docker-compose --profile test up -d postgres-test

if ($LASTEXITCODE -ne 0) {
    Write-Host "❌ Failed to start test database" -ForegroundColor Red
    exit 1
}

# Wait for database to be ready
Write-Host "⏳ Waiting for database to be ready..." -ForegroundColor Yellow
$maxAttempts = 30
$attempt = 0

do {
    $attempt++
    Start-Sleep -Seconds 2
    $healthStatus = docker-compose --profile test ps postgres-test --format json | ConvertFrom-Json | Select-Object -ExpandProperty Health
    if ($healthStatus -eq "healthy") {
        break
    }
    if ($attempt -ge $maxAttempts) {
        Write-Host "❌ Database failed to become ready within timeout" -ForegroundColor Red
        docker-compose --profile test logs postgres-test
        exit 1
    }
} while ($true)

Write-Host "✅ Database is ready" -ForegroundColor Green

# Run tests based on type
switch ($TestType.ToLower()) {
    "postgresql" {
        Write-Host "🧪 Running PostgreSQL tests..." -ForegroundColor Yellow
        docker-compose --profile test run --rm inventario-test-postgresql
    }
    "go" {
        Write-Host "🧪 Running Go tests..." -ForegroundColor Yellow
        docker-compose --profile test run --rm inventario-test
    }
    "all" {
        Write-Host "🧪 Running all tests..." -ForegroundColor Yellow
        docker-compose --profile test run --rm inventario-test
    }
    default {
        Write-Host "❌ Invalid test type: $TestType. Valid options: all, postgresql, go" -ForegroundColor Red
        exit 1
    }
}

$testExitCode = $LASTEXITCODE

# Show logs if requested
if ($Logs) {
    Write-Host "📋 Test logs:" -ForegroundColor Yellow
    docker-compose --profile test logs
}

# Clean up if requested
if ($Clean) {
    Write-Host "🧹 Cleaning up test containers..." -ForegroundColor Yellow
    docker-compose --profile test down -v
    docker rmi inventario:test 2>$null
    Write-Host "✅ Cleanup completed" -ForegroundColor Green
} else {
    # Just stop the services
    docker-compose --profile test down
}

# Report results
if ($testExitCode -eq 0) {
    Write-Host "🎉 Tests completed successfully!" -ForegroundColor Green
} else {
    Write-Host "❌ Tests failed with exit code: $testExitCode" -ForegroundColor Red
}

exit $testExitCode
