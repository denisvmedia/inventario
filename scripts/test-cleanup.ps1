# Test cleanup script for Docker Compose (PowerShell)
# This script ensures proper cleanup of test containers and networks

$ErrorActionPreference = "Continue"

Write-Host "🧹 Cleaning up test environment..." -ForegroundColor Yellow

# Stop and remove test containers
Write-Host "Stopping test containers..." -ForegroundColor Blue
docker compose --profile test down --remove-orphans --volumes

# Remove test containers if they still exist
Write-Host "Removing any remaining test containers..." -ForegroundColor Blue
$containers = @("inventario-postgres-test", "inventario-test-runner", "inventario-migrate-runner", "inventario-postgresql-test-runner")
foreach ($container in $containers) {
    try {
        docker container rm -f $container 2>$null
    } catch {
        # Ignore errors if container doesn't exist
    }
}

# Remove test networks
Write-Host "Removing test networks..." -ForegroundColor Blue
try {
    docker network rm inventario_inventario-test-network 2>$null
} catch {
    # Ignore errors if network doesn't exist
}

# Remove test volumes
Write-Host "Removing test volumes..." -ForegroundColor Blue
try {
    $testVolumes = docker volume ls -q | Where-Object { $_ -match "inventario.*test" }
    if ($testVolumes) {
        docker volume rm $testVolumes 2>$null
    }
} catch {
    # Ignore errors if no volumes found
}

# Remove dangling images from test builds
Write-Host "Removing dangling test images..." -ForegroundColor Blue
try {
    docker image prune -f --filter label=stage=test-runner 2>$null
} catch {
    # Ignore errors
}

Write-Host "✅ Test environment cleanup completed!" -ForegroundColor Green
