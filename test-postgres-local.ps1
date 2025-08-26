#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Local PostgreSQL testing script that replicates the GitHub Actions workflow
.DESCRIPTION
    This script sets up PostgreSQL using Docker, configures databases and users,
    and runs the same tests as the GitHub Actions workflow for local development.
.PARAMETER SkipSetup
    Skip PostgreSQL container setup (useful if container is already running)
.PARAMETER SkipCleanup
    Skip cleanup of PostgreSQL container after tests
.PARAMETER Verbose
    Enable verbose output
.EXAMPLE
    .\test-postgres-local.ps1
.EXAMPLE
    .\test-postgres-local.ps1 -SkipSetup -SkipCleanup
#>

param(
    [switch]$SkipSetup,
    [switch]$SkipCleanup,
    [switch]$Verbose
)

# Set error action preference
$ErrorActionPreference = "Stop"

# Add PostgreSQL bin directory to PATH if it exists
$PostgreSQLBinPath = "C:\Program Files\PostgreSQL\17\bin"
if (Test-Path $PostgreSQLBinPath) {
    $env:PATH = "$PostgreSQLBinPath;$env:PATH"
    Write-Verbose "Added PostgreSQL bin directory to PATH: $PostgreSQLBinPath"
}

# Configuration
$CONTAINER_NAME = "inventario-test-postgres"
$POSTGRES_VERSION = "17"
$POSTGRES_DB = "inventario_test"
$POSTGRES_USER = "inventario_test"
$POSTGRES_PASSWORD = "test_password"
$POSTGRES_PORT = "55432"
$GO_VERSION = "1.24.3"

# Environment variables for tests
$env:POSTGRES_TEST_DSN = "postgres://inventario_test:test_password@localhost:5432/inventario_test?sslmode=disable"
$env:POSTGRES_LIMITED_TEST_DSN = "postgres://limited_user:limited_password@localhost:5432/inventario?sslmode=disable"

function Write-Info {
    param([string]$Message)
    Write-Host "INFO: $Message" -ForegroundColor Green
}

function Write-Error-Custom {
    param([string]$Message)
    Write-Host "ERROR: $Message" -ForegroundColor Red
}

function Write-Warning-Custom {
    param([string]$Message)
    Write-Host "WARNING: $Message" -ForegroundColor Yellow
}

function Test-DockerAvailable {
    try {
        docker --version | Out-Null
        return $true
    }
    catch {
        return $false
    }
}

function Test-PostgreSQLClient {
    try {
        psql --version | Out-Null
        return $true
    }
    catch {
        return $false
    }
}

function Wait-ForPostgreSQL {
    param([int]$MaxAttempts = 30)

    Write-Info "Waiting for PostgreSQL to be ready..."

    for ($i = 1; $i -le $MaxAttempts; $i++) {
        try {
            # Use docker exec to check if PostgreSQL is ready inside the container
            docker exec $CONTAINER_NAME pg_isready -U $POSTGRES_USER -d $POSTGRES_DB | Out-Null
            if ($LASTEXITCODE -eq 0) {
                Write-Info "PostgreSQL is ready!"
                return $true
            }
        }
        catch {
            # Ignore errors and continue trying
        }

        if ($Verbose) {
            Write-Warning-Custom "Attempt $i/$MaxAttempts failed, waiting 2 seconds..."
        }
        Start-Sleep -Seconds 2
    }

    Write-Error-Custom "PostgreSQL failed to become ready after $MaxAttempts attempts"
    return $false
}

function Setup-PostgreSQL {
    Write-Info "Setting up PostgreSQL container..."
    
    # Check if container already exists
    $existingContainer = docker ps -a --filter "name=$CONTAINER_NAME" --format "{{.Names}}"
    if ($existingContainer -eq $CONTAINER_NAME) {
        Write-Info "Removing existing container: $CONTAINER_NAME"
        docker rm -f $CONTAINER_NAME | Out-Null
    }
    
    # Start PostgreSQL container with trust authentication for all connections
    Write-Info "Starting PostgreSQL $POSTGRES_VERSION container..."
    docker run -d `
        --name $CONTAINER_NAME `
        -e POSTGRES_DB=$POSTGRES_DB `
        -e POSTGRES_USER=$POSTGRES_USER `
        -e POSTGRES_PASSWORD=$POSTGRES_PASSWORD `
        -e POSTGRES_HOST_AUTH_METHOD=trust `
        -e POSTGRES_INITDB_ARGS="--auth-host=trust --auth-local=trust" `
        -p "${POSTGRES_PORT}:5432" `
        --health-cmd="pg_isready" `
        --health-interval=10s `
        --health-timeout=5s `
        --health-retries=5 `
        postgres:$POSTGRES_VERSION | Out-Null
    
    if ($LASTEXITCODE -ne 0) {
        Write-Error-Custom "Failed to start PostgreSQL container"
        exit 1
    }
    
    Write-Info "PostgreSQL container started successfully"
}

function Configure-PostgreSQLUsers {
    Write-Info "Configuring PostgreSQL users and databases..."

    # Use docker exec to run psql commands inside the container
    # This avoids authentication issues from the host

    # Check and fix pg_hba.conf to ensure trust authentication
    Write-Info "Configuring pg_hba.conf for trust authentication..."
    docker exec $CONTAINER_NAME bash -c "cat > /var/lib/postgresql/data/pg_hba.conf << 'EOF'
# Trust authentication for all connections
local   all             all                                     trust
host    all             all             127.0.0.1/32            trust
host    all             all             ::1/128                 trust
host    all             all             0.0.0.0/0               trust
host    all             all             ::/0                    trust
EOF"

    # Show the pg_hba.conf content for debugging
    if ($Verbose) {
        Write-Info "Current pg_hba.conf content:"
        docker exec $CONTAINER_NAME cat /var/lib/postgresql/data/pg_hba.conf
    }

    # Restart the entire container to ensure pg_hba.conf changes take effect
    Write-Info "Restarting PostgreSQL container to apply authentication changes..."
    docker restart $CONTAINER_NAME | Out-Null

    # Wait for container to restart
    Start-Sleep -Seconds 5

    # Verify PostgreSQL is ready after restart
    Write-Info "Verifying PostgreSQL is ready after restart..."
    if (-not (Wait-ForPostgreSQL -MaxAttempts 15)) {
        Write-Error-Custom "PostgreSQL failed to restart properly"
        return $false
    }

    # Create the main test database for bootstrap tests
    Write-Info "Creating inventario database..."
    docker exec $CONTAINER_NAME psql -U $POSTGRES_USER -d $POSTGRES_DB -c "CREATE DATABASE inventario;"

    # Create the main application users
    Write-Info "Creating application users..."
    docker exec $CONTAINER_NAME psql -U $POSTGRES_USER -d $POSTGRES_DB -c "CREATE USER inventario WITH PASSWORD 'inventario_password'; CREATE USER inventario_migrator WITH PASSWORD 'inventario_migrator_password';"

    # Grant necessary privileges to the main users
    Write-Info "Granting privileges to main users..."
    docker exec $CONTAINER_NAME psql -U $POSTGRES_USER -d $POSTGRES_DB -c "GRANT ALL PRIVILEGES ON DATABASE inventario_test TO inventario; GRANT ALL PRIVILEGES ON DATABASE inventario_test TO inventario_migrator;"

    # Grant superuser privileges to inventario_test user for bootstrap tests
    Write-Info "Granting superuser privileges..."
    docker exec $CONTAINER_NAME psql -U $POSTGRES_USER -d $POSTGRES_DB -c "ALTER USER inventario_test WITH SUPERUSER;"

    # Create a limited privilege user for testing transaction rollback
    Write-Info "Creating limited privilege user..."
    docker exec $CONTAINER_NAME psql -U $POSTGRES_USER -d $POSTGRES_DB -c "CREATE USER limited_user WITH PASSWORD 'limited_password' NOSUPERUSER NOCREATEDB NOCREATEROLE; GRANT CONNECT ON DATABASE inventario TO limited_user;"

    # Set up schema permissions on inventario database
    Write-Info "Setting up schema permissions..."
    docker exec $CONTAINER_NAME psql -U $POSTGRES_USER -d inventario -c "GRANT ALL ON SCHEMA public TO inventario; GRANT ALL ON SCHEMA public TO inventario_migrator; GRANT USAGE ON SCHEMA public TO limited_user;"

    Write-Info "PostgreSQL users and databases configured successfully"
}

function Test-GoVersion {
    try {
        $goVersion = go version
        if ($Verbose) {
            Write-Info "Go version: $goVersion"
        }
        return $true
    }
    catch {
        Write-Error-Custom "Go is not installed or not in PATH"
        return $false
    }
}

function Run-PostgreSQLRegistryTests {
    Write-Info "Running PostgreSQL registry tests..."

    Push-Location "go"
    try {
        # Set environment variable for this session (using trust auth, no password needed)
        $env:POSTGRES_TEST_DSN = "postgres://inventario_test@localhost:55432/inventario_test?sslmode=disable"
        $env:CGO_ENABLED = "0"

        # Set environment variables for the current PowerShell session
        $env:POSTGRES_TEST_DSN = "postgres://inventario_test@localhost:55432/inventario_test?sslmode=disable"

        if ($Verbose) {
            Write-Host "--- PostgreSQL Registry Test Output ---" -ForegroundColor Cyan
            Write-Info "Using DSN: $env:POSTGRES_TEST_DSN"
            go test -v ./registry/postgres/...
            $exitCode = $LASTEXITCODE
            Write-Host "--- End PostgreSQL Registry Test Output ---" -ForegroundColor Cyan
        } else {
            go test ./registry/postgres/...
            $exitCode = $LASTEXITCODE
        }

        if ($exitCode -ne 0) {
            Write-Error-Custom "PostgreSQL registry tests failed"
            return $false
        }

        Write-Info "PostgreSQL registry tests passed"
        return $true
    }
    finally {
        Pop-Location
    }
}

function Run-BootstrapIntegrationTests {
    Write-Info "Running Bootstrap integration tests..."

    Push-Location "go"
    try {
        # Set environment variables (using trust auth, no password needed)
        $env:POSTGRES_TEST_DSN = "postgres://inventario_test@localhost:55432/inventario?sslmode=disable"
        $env:POSTGRES_LIMITED_TEST_DSN = "postgres://limited_user@localhost:55432/inventario?sslmode=disable"

        # Set environment variables for the current PowerShell session
        $env:POSTGRES_TEST_DSN = "postgres://inventario_test@localhost:55432/inventario?sslmode=disable"
        $env:POSTGRES_LIMITED_TEST_DSN = "postgres://limited_user@localhost:55432/inventario?sslmode=disable"

        if ($Verbose) {
            Write-Info "Main DSN: $env:POSTGRES_TEST_DSN"
            Write-Info "Limited DSN: $env:POSTGRES_LIMITED_TEST_DSN"
            Write-Host "--- Bootstrap Integration Test Output ---" -ForegroundColor Cyan
            go test -v ./schema/bootstrap
            $exitCode = $LASTEXITCODE
            Write-Host "--- End Bootstrap Integration Test Output ---" -ForegroundColor Cyan
        } else {
            go test ./schema/bootstrap
            $exitCode = $LASTEXITCODE
        }

        if ($exitCode -ne 0) {
            Write-Error-Custom "Bootstrap integration tests failed"
            return $false
        }

        Write-Info "Bootstrap integration tests passed"
        return $true
    }
    finally {
        Pop-Location
    }
}

function Cleanup-PostgreSQL {
    if (-not $SkipCleanup) {
        Write-Info "Cleaning up PostgreSQL container..."
        docker rm -f $CONTAINER_NAME | Out-Null
        Write-Info "Cleanup completed"
    } else {
        Write-Info "Skipping cleanup (container $CONTAINER_NAME is still running)"
    }
}

# Main execution
try {
    Write-Info "Starting local PostgreSQL testing script..."
    
    # Check prerequisites
    if (-not (Test-DockerAvailable)) {
        Write-Error-Custom "Docker is not available. Please install Docker and ensure it's running."
        exit 1
    }
    
    if (-not (Test-PostgreSQLClient)) {
        Write-Error-Custom "PostgreSQL client (psql) is not available. Please install PostgreSQL client tools."
        exit 1
    }
    
    if (-not (Test-GoVersion)) {
        exit 1
    }
    
    # Setup PostgreSQL if not skipped
    if (-not $SkipSetup) {
        Setup-PostgreSQL
        
        if (-not (Wait-ForPostgreSQL)) {
            exit 1
        }
        
        Configure-PostgreSQLUsers
    } else {
        Write-Info "Skipping PostgreSQL setup"
        if (-not (Wait-ForPostgreSQL)) {
            Write-Error-Custom "PostgreSQL is not ready. Remove -SkipSetup flag to set up the container."
            exit 1
        }
    }
    
    # Install Go dependencies
    Write-Info "Installing Go dependencies..."
    Push-Location "go"
    try {
        go mod download
        if ($LASTEXITCODE -ne 0) {
            Write-Error-Custom "Failed to download Go dependencies"
            exit 1
        }
    }
    finally {
        Pop-Location
    }
    
    # Run tests
    $registryTestsSuccess = Run-PostgreSQLRegistryTests
    $bootstrapTestsSuccess = Run-BootstrapIntegrationTests
    
    # Summary
    Write-Info "Test Summary:"
    Write-Host "  PostgreSQL Registry Tests: $(if ($registryTestsSuccess) { 'PASSED' } else { 'FAILED' })" -ForegroundColor $(if ($registryTestsSuccess) { 'Green' } else { 'Red' })
    Write-Host "  Bootstrap Integration Tests: $(if ($bootstrapTestsSuccess) { 'PASSED' } else { 'FAILED' })" -ForegroundColor $(if ($bootstrapTestsSuccess) { 'Green' } else { 'Red' })
    
    if ($registryTestsSuccess -and $bootstrapTestsSuccess) {
        Write-Info "All tests passed successfully!"
        $exitCode = 0
    } else {
        Write-Error-Custom "Some tests failed"
        $exitCode = 1
    }
    
    Cleanup-PostgreSQL
    exit $exitCode
}
catch {
    Write-Error-Custom "Script execution failed: $($_.Exception.Message)"
    if ($Verbose) {
        Write-Host $_.ScriptStackTrace -ForegroundColor Red
    }
    Cleanup-PostgreSQL
    exit 1
}
