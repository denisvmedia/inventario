# Docker Test Script for Inventario (PowerShell)
# This script tests the Docker setup on Windows

$ErrorActionPreference = "Stop"

Write-Host "🐳 Testing Inventario Docker Setup" -ForegroundColor Cyan
Write-Host "==================================" -ForegroundColor Cyan

# Check if Docker is available
try {
    docker --version | Out-Null
    Write-Host "✅ Docker is available" -ForegroundColor Green
} catch {
    Write-Host "❌ Docker is not installed or not in PATH" -ForegroundColor Red
    exit 1
}

# Check if Docker Compose is available
try {
    docker-compose --version | Out-Null
    Write-Host "✅ Docker Compose is available" -ForegroundColor Green
} catch {
    Write-Host "❌ Docker Compose is not installed or not in PATH" -ForegroundColor Red
    exit 1
}

# Check if .env file exists
if (-not (Test-Path .env)) {
    Write-Host "📝 Creating .env file from .env.example" -ForegroundColor Yellow
    Copy-Item .env.example .env
}

Write-Host "✅ Environment file is ready" -ForegroundColor Green

# Build the Docker image
Write-Host "🔨 Building Docker image..." -ForegroundColor Yellow
docker build -t inventario:test .

Write-Host "✅ Docker image built successfully" -ForegroundColor Green

# Start services
Write-Host "🚀 Starting services..." -ForegroundColor Yellow
docker-compose up -d

# Wait for services to be healthy
Write-Host "⏳ Waiting for services to be healthy..." -ForegroundColor Yellow
Start-Sleep -Seconds 30

# Check if services are running
$services = docker-compose ps
if ($services -notmatch "Up") {
    Write-Host "❌ Services are not running properly" -ForegroundColor Red
    docker-compose logs
    exit 1
}

Write-Host "✅ Services are running" -ForegroundColor Green

# Test API endpoint
Write-Host "🧪 Testing API endpoint..." -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri "http://localhost:3333/api/v1/settings" -UseBasicParsing
    if ($response.StatusCode -eq 200) {
        Write-Host "✅ API is responding" -ForegroundColor Green
    } else {
        throw "API returned status code: $($response.StatusCode)"
    }
} catch {
    Write-Host "❌ API is not responding: $_" -ForegroundColor Red
    docker-compose logs inventario
    exit 1
}

# Seed the database
Write-Host "🌱 Seeding database..." -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri "http://localhost:3333/api/v1/seed" -Method POST -UseBasicParsing
    Write-Host "✅ Database seeded successfully" -ForegroundColor Green
} catch {
    Write-Host "⚠️  Database seeding failed (this might be expected if already seeded)" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "🎉 Docker setup test completed successfully!" -ForegroundColor Green
Write-Host ""
Write-Host "📋 Next steps:" -ForegroundColor Cyan
Write-Host "   • Access the application at: http://localhost:3333" -ForegroundColor White
Write-Host "   • Stop services with: docker-compose down" -ForegroundColor White
Write-Host "   • View logs with: docker-compose logs -f" -ForegroundColor White
Write-Host ""
