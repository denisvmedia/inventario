# PostgreSQL Test Setup Script for Windows
# This script helps set up PostgreSQL for testing the Inventario application

param(
    [string]$PostgreSQLVersion = "15",
    [string]$DatabaseName = "inventario_test",
    [string]$Username = "inventario_test",
    [string]$Password = "test_password",
    [int]$Port = 5432,
    [switch]$UseDocker,
    [switch]$Help
)

if ($Help) {
    Write-Host @"
PostgreSQL Test Setup Script

This script helps set up PostgreSQL for testing the Inventario application.

USAGE:
    .\scripts\setup-postgresql-test.ps1 [OPTIONS]

OPTIONS:
    -PostgreSQLVersion <version>  PostgreSQL version to use (default: 15)
    -DatabaseName <name>          Test database name (default: inventario_test)
    -Username <user>              Test user name (default: inventario_test)
    -Password <pass>              Test user password (default: test_password)
    -Port <port>                  PostgreSQL port (default: 5432)
    -UseDocker                    Use Docker instead of local PostgreSQL
    -Help                         Show this help message

EXAMPLES:
    # Set up using Docker (recommended)
    .\scripts\setup-postgresql-test.ps1 -UseDocker

    # Set up using local PostgreSQL
    .\scripts\setup-postgresql-test.ps1

    # Custom configuration
    .\scripts\setup-postgresql-test.ps1 -UseDocker -DatabaseName "my_test_db" -Username "testuser"

AFTER SETUP:
    Set the environment variable:
    `$env:POSTGRES_TEST_DSN="postgres://$Username`:$Password@localhost:$Port/$DatabaseName`?sslmode=disable"

    Then run PostgreSQL tests:
    make test-go-postgresql
"@
    exit 0
}

$ErrorActionPreference = "Stop"

Write-Host "🐘 PostgreSQL Test Setup for Inventario" -ForegroundColor Cyan
Write-Host "=======================================" -ForegroundColor Cyan

if ($UseDocker) {
    Write-Host "🐳 Setting up PostgreSQL using Docker..." -ForegroundColor Yellow
    
    # Check if Docker is available
    try {
        docker --version | Out-Null
        Write-Host "✅ Docker is available" -ForegroundColor Green
    } catch {
        Write-Host "❌ Docker is not installed or not in PATH" -ForegroundColor Red
        Write-Host "   Please install Docker Desktop from: https://www.docker.com/products/docker-desktop" -ForegroundColor Yellow
        exit 1
    }

    # Stop and remove existing container if it exists
    $containerName = "inventario-postgres-test"
    try {
        docker stop $containerName 2>$null | Out-Null
        docker rm $containerName 2>$null | Out-Null
        Write-Host "🧹 Cleaned up existing container" -ForegroundColor Yellow
    } catch {
        # Container doesn't exist, which is fine
    }

    # Start PostgreSQL container
    Write-Host "🚀 Starting PostgreSQL container..." -ForegroundColor Yellow
    $dockerCmd = @(
        "run", "--name", $containerName,
        "-e", "POSTGRES_DB=$DatabaseName",
        "-e", "POSTGRES_USER=$Username", 
        "-e", "POSTGRES_PASSWORD=$Password",
        "-p", "${Port}:5432",
        "-d", "postgres:$PostgreSQLVersion"
    )
    
    try {
        docker @dockerCmd | Out-Null
        Write-Host "✅ PostgreSQL container started successfully" -ForegroundColor Green
    } catch {
        Write-Host "❌ Failed to start PostgreSQL container" -ForegroundColor Red
        Write-Host "   Error: $_" -ForegroundColor Red
        exit 1
    }

    # Wait for PostgreSQL to be ready
    Write-Host "⏳ Waiting for PostgreSQL to be ready..." -ForegroundColor Yellow
    $maxAttempts = 30
    $attempt = 0
    
    do {
        $attempt++
        Start-Sleep -Seconds 2
        try {
            docker exec $containerName pg_isready -U $Username -d $DatabaseName 2>$null | Out-Null
            $ready = $true
        } catch {
            $ready = $false
        }
    } while (-not $ready -and $attempt -lt $maxAttempts)

    if ($ready) {
        Write-Host "✅ PostgreSQL is ready!" -ForegroundColor Green
    } else {
        Write-Host "❌ PostgreSQL failed to start within expected time" -ForegroundColor Red
        Write-Host "   Check container logs: docker logs $containerName" -ForegroundColor Yellow
        exit 1
    }

} else {
    Write-Host "🏠 Setting up PostgreSQL using local installation..." -ForegroundColor Yellow
    
    # Check if PostgreSQL is available
    try {
        psql --version | Out-Null
        Write-Host "✅ PostgreSQL client is available" -ForegroundColor Green
    } catch {
        Write-Host "❌ PostgreSQL client (psql) is not installed or not in PATH" -ForegroundColor Red
        Write-Host "   Please install PostgreSQL from: https://www.postgresql.org/download/windows/" -ForegroundColor Yellow
        exit 1
    }

    # Create database and user
    Write-Host "🔧 Creating test database and user..." -ForegroundColor Yellow
    
    $createScript = @"
-- Create test database
DROP DATABASE IF EXISTS $DatabaseName;
CREATE DATABASE $DatabaseName;

-- Create test user
DROP USER IF EXISTS $Username;
CREATE USER $Username WITH PASSWORD '$Password';

-- Grant privileges
GRANT ALL PRIVILEGES ON DATABASE $DatabaseName TO $Username;
"@

    try {
        $createScript | psql -U postgres -h localhost -p $Port 2>$null
        Write-Host "✅ Database and user created successfully" -ForegroundColor Green
    } catch {
        Write-Host "❌ Failed to create database and user" -ForegroundColor Red
        Write-Host "   Make sure PostgreSQL is running and you have admin access" -ForegroundColor Yellow
        Write-Host "   You may need to run: psql -U postgres" -ForegroundColor Yellow
        exit 1
    }
}

# Set environment variable
$dsn = "postgres://${Username}:${Password}@localhost:${Port}/${DatabaseName}?sslmode=disable"
$env:POSTGRES_TEST_DSN = $dsn

Write-Host ""
Write-Host "🎉 PostgreSQL test setup completed successfully!" -ForegroundColor Green
Write-Host ""
Write-Host "📋 Configuration:" -ForegroundColor Cyan
Write-Host "   Database: $DatabaseName" -ForegroundColor White
Write-Host "   Username: $Username" -ForegroundColor White
Write-Host "   Password: $Password" -ForegroundColor White
Write-Host "   Port: $Port" -ForegroundColor White
Write-Host "   DSN: $dsn" -ForegroundColor White
Write-Host ""
Write-Host "🔧 Environment variable set for current session:" -ForegroundColor Cyan
Write-Host "   POSTGRES_TEST_DSN=$dsn" -ForegroundColor White
Write-Host ""
Write-Host "💡 To make this permanent, add to your PowerShell profile:" -ForegroundColor Cyan
Write-Host "   `$env:POSTGRES_TEST_DSN=`"$dsn`"" -ForegroundColor White
Write-Host ""
Write-Host "🧪 Now you can run PostgreSQL tests:" -ForegroundColor Cyan
Write-Host "   make test-go-postgresql" -ForegroundColor White
Write-Host "   # or" -ForegroundColor Gray
Write-Host "   cd go && go test -v ./registry/postgresql/..." -ForegroundColor White
Write-Host ""

if ($UseDocker) {
    Write-Host "🐳 Docker container management:" -ForegroundColor Cyan
    Write-Host "   Stop:    docker stop $containerName" -ForegroundColor White
    Write-Host "   Start:   docker start $containerName" -ForegroundColor White
    Write-Host "   Remove:  docker stop $containerName && docker rm $containerName" -ForegroundColor White
    Write-Host "   Logs:    docker logs $containerName" -ForegroundColor White
    Write-Host ""
}
