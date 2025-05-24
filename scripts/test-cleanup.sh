#!/bin/bash

# Test cleanup script for Docker Compose
# This script ensures proper cleanup of test containers and networks

set -e

echo "🧹 Cleaning up test environment..."

# Stop and remove test containers
echo "Stopping test containers..."
docker compose --profile test down --remove-orphans --volumes

# Remove test containers if they still exist
echo "Removing any remaining test containers..."
docker container rm -f inventario-postgres-test inventario-test-runner inventario-migrate-runner inventario-postgresql-test-runner 2>/dev/null || true

# Remove test networks
echo "Removing test networks..."
docker network rm inventario_inventario-test-network 2>/dev/null || true

# Remove test volumes
echo "Removing test volumes..."
docker volume rm $(docker volume ls -q | grep inventario.*test) 2>/dev/null || true

# Remove dangling images from test builds
echo "Removing dangling test images..."
docker image prune -f --filter label=stage=test-runner 2>/dev/null || true

echo "✅ Test environment cleanup completed!"
