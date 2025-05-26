# Test Scripts

This directory contains scripts for running and managing tests with automatic database cleanup.

## Scripts Overview

### `run-tests.ps1` / `run-tests.sh`
Main test runner scripts that automatically handle database creation and cleanup.

**Usage:**
```powershell
# Windows PowerShell
.\scripts\run-tests.ps1                          # Run all tests
.\scripts\run-tests.ps1 inventario-test          # Run all tests (explicit)
.\scripts\run-tests.ps1 inventario-test-postgres  # Run only PostgreSQL tests

# Linux/macOS Bash
./scripts/run-tests.sh                          # Run all tests
./scripts/run-tests.sh inventario-test          # Run all tests (explicit)
./scripts/run-tests.sh inventario-test-postgres  # Run only PostgreSQL tests
```

**Features:**
- Automatically creates fresh test database
- Runs migrations before tests
- Executes specified test suite
- Automatically cleans up containers, networks, and volumes after completion
- Handles interruptions (Ctrl+C) gracefully with cleanup
- Provides colored output for better readability

**Options:**
- `-NoCleanup` (PowerShell) / `NO_CLEANUP=true` (Bash): Skip cleanup for debugging

### `test-cleanup.ps1` / `test-cleanup.sh`
Standalone cleanup scripts for manual cleanup of test environment.

**Usage:**
```powershell
# Windows PowerShell
.\scripts\test-cleanup.ps1

# Linux/macOS Bash
./scripts/test-cleanup.sh
```

**What it cleans:**
- Test containers (postgres-test, test runners)
- Test networks
- Test volumes
- Dangling test images

## Docker Compose Changes

The `docker-compose.yaml` has been updated with the following improvements for testing:

1. **Automatic Container Removal**: All test services have `restart: "no"` to prevent automatic restarts
2. **Ephemeral Database**: PostgreSQL test database uses `tmpfs` for faster tests and automatic cleanup
3. **Isolated Test Network**: Tests run in a separate network to avoid conflicts
4. **Health Checks**: Proper health checks ensure database is ready before running tests

## Test Database Lifecycle

1. **Creation**: Fresh PostgreSQL container starts with empty database
2. **Migration**: Database schema is applied via `inventario-migrate` service
3. **Testing**: Tests run against the prepared database
4. **Cleanup**: All containers, networks, and data are automatically removed

## Manual Docker Compose Usage

If you prefer to use Docker Compose directly:

```bash
# Start test environment
docker compose --profile test up --build

# Run specific test service
docker compose --profile test run --rm inventario-test

# Clean up manually
docker compose --profile test down --remove-orphans --volumes
```

## Troubleshooting

### Tests fail to connect to database
- Ensure no other PostgreSQL instance is running on port 5433
- Check if previous test containers are still running: `docker ps`
- Run cleanup script manually

### Cleanup fails
- Check for running containers: `docker ps -a`
- Manually remove containers: `docker container rm -f <container-name>`
- Check for networks: `docker network ls`
- Manually remove networks: `docker network rm <network-name>`

### Permission issues (Linux/macOS)
- Make scripts executable: `chmod +x scripts/*.sh`
- Ensure Docker daemon is running and accessible

## Environment Variables

You can customize test behavior using environment variables:

```bash
# Test database configuration
POSTGRES_TEST_DB=my_test_db
POSTGRES_TEST_USER=test_user
POSTGRES_TEST_PASSWORD=test_pass
POSTGRES_TEST_PORT=5434

# Test timeouts
GO_TEST_TIMEOUT=15m
GO_TEST_VERBOSE=false
```
