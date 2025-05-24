# Test runner script with automatic cleanup
# Usage: .\scripts\run-tests.ps1 [test-service-name]
# Examples:
#   .\scripts\run-tests.ps1                          # Run all tests
#   .\scripts\run-tests.ps1 inventario-test          # Run all tests
#   .\scripts\run-tests.ps1 inventario-test-postgresql  # Run only PostgreSQL tests

param(
    [string]$TestService = "inventario-test",
    [switch]$NoCleanup = $false
)

$ErrorActionPreference = "Stop"

# Function to cleanup on exit
function Cleanup {
    if (-not $NoCleanup) {
        Write-Host "`n🧹 Running cleanup..." -ForegroundColor Yellow
        & "$PSScriptRoot\test-cleanup.ps1"
    }
}

# Register cleanup function to run on script exit
Register-EngineEvent PowerShell.Exiting -Action { Cleanup }

# Trap Ctrl+C and other interruptions
trap {
    Write-Host "`n⚠️  Test interrupted. Running cleanup..." -ForegroundColor Red
    Cleanup
    exit 1
}

try {
    Write-Host "🚀 Starting test environment..." -ForegroundColor Green
    
    # Ensure clean state before starting
    Write-Host "Ensuring clean state..." -ForegroundColor Blue
    & "$PSScriptRoot\test-cleanup.ps1"
    
    Write-Host "Building and starting test services..." -ForegroundColor Blue
    docker compose --profile test build
    
    Write-Host "Running tests with service: $TestService" -ForegroundColor Blue
    $exitCode = 0
    
    # Run the specified test service
    docker compose --profile test run --rm $TestService
    $exitCode = $LASTEXITCODE
    
    if ($exitCode -eq 0) {
        Write-Host "✅ Tests completed successfully!" -ForegroundColor Green
    } else {
        Write-Host "❌ Tests failed with exit code: $exitCode" -ForegroundColor Red
    }
    
} catch {
    Write-Host "❌ Error running tests: $_" -ForegroundColor Red
    $exitCode = 1
} finally {
    # Always cleanup unless explicitly disabled
    Cleanup
}

exit $exitCode
