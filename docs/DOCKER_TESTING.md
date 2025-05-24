# Docker Testing Guide

This document explains how to run tests for the Inventario application using the unified Docker setup. This approach provides a consistent testing environment across different platforms and eliminates the need to install PostgreSQL locally.

## Overview

The unified Docker setup includes:

- **Multi-target Dockerfile**: Single Dockerfile with production and test targets
- **Profile-based Compose**: Single docker-compose.yaml with production, development, and test profiles
- **PostgreSQL Test Database**: Isolated test database with temporary storage
- **Test Runner Container**: Go environment with all dependencies for running tests
- **Automated Scripts**: Helper scripts for easy test execution
- **Make Targets**: Integration with the project's Makefile

## Quick Start

### Prerequisites

- Docker and Docker Compose installed
- No local PostgreSQL installation required

### Run All Tests

```bash
# Using Make (recommended)
make docker-test-go

# Using Docker Compose directly
docker-compose --profile test run --rm inventario-test

# Using helper script (Windows)
.\scripts\docker-test-run.ps1

# Using helper script (Unix/Linux/macOS)
./scripts/docker-test-run.sh
```

### Run PostgreSQL Tests Only

```bash
# Using Make
make docker-test-go-postgresql

# Using Docker Compose directly
docker-compose --profile test run --rm inventario-test-postgresql

# Using helper script
.\scripts\docker-test-run.ps1 -TestType postgresql
./scripts/docker-test-run.sh --type postgresql
```

## Available Commands

### Make Targets

| Command | Description |
|---------|-------------|
| `make docker-test-build` | Build the test Docker image |
| `make docker-test-up` | Start the test database |
| `make docker-test-down` | Stop test services |
| `make docker-test-clean` | Clean up test containers and images |
| `make docker-test-go` | Run all Go tests in Docker |
| `make docker-test-go-postgresql` | Run PostgreSQL tests in Docker |
| `make docker-test-logs` | View test container logs |

### Docker Compose Commands

```bash
# Build test image
docker build --target test-runner -t inventario:test .

# Start test database
docker-compose --profile test up -d postgres-test

# Run all tests
docker-compose --profile test run --rm inventario-test

# Run PostgreSQL tests only
docker-compose --profile test run --rm inventario-test-postgresql

# View logs
docker-compose --profile test logs

# Stop and clean up
docker-compose --profile test down -v
```

## Helper Scripts

### Windows PowerShell Script

```powershell
# Run all tests
.\scripts\docker-test-run.ps1

# Run specific test type
.\scripts\docker-test-run.ps1 -TestType postgresql

# Build image and run tests
.\scripts\docker-test-run.ps1 -Build

# Run tests and show logs
.\scripts\docker-test-run.ps1 -Logs

# Run tests and clean up afterward
.\scripts\docker-test-run.ps1 -Clean

# Get help
.\scripts\docker-test-run.ps1 -Help
```

### Unix/Linux/macOS Bash Script

```bash
# Run all tests
./scripts/docker-test-run.sh

# Run specific test type
./scripts/docker-test-run.sh --type postgresql

# Build image and run tests
./scripts/docker-test-run.sh --build

# Run tests and show logs
./scripts/docker-test-run.sh --logs

# Run tests and clean up afterward
./scripts/docker-test-run.sh --clean

# Get help
./scripts/docker-test-run.sh --help
```

## Configuration

### Environment Variables

The test setup uses these environment variables:

| Variable | Value | Description |
|----------|-------|-------------|
| `POSTGRES_TEST_DSN` | `postgres://inventario_test:test_password@postgres-test:5432/inventario_test?sslmode=disable` | PostgreSQL connection string for tests |
| `GO_TEST_TIMEOUT` | `10m` | Test timeout duration |
| `GO_TEST_VERBOSE` | `true` | Enable verbose test output |

### Test Database Configuration

- **Database**: `inventario_test`
- **User**: `inventario_test`
- **Password**: `test_password`
- **Port**: `5433` (mapped to avoid conflicts with local PostgreSQL)
- **Storage**: Temporary (tmpfs) for faster tests

## Architecture

### Files Structure

```
├── docker-compose.test.yml    # Test services configuration
├── Dockerfile.test           # Test environment image
├── scripts/
│   ├── docker-test-run.ps1   # Windows helper script
│   └── docker-test-run.sh    # Unix helper script
└── docs/
    └── DOCKER_TESTING.md     # This documentation
```

### Test Services

1. **postgres-test**: PostgreSQL 15 Alpine with test database
2. **inventario-test**: Full test runner for all Go tests
3. **inventario-test-postgresql**: Specialized runner for PostgreSQL tests only

## Advantages

### Consistency
- Same environment across all platforms
- No local PostgreSQL installation required
- Isolated test database with clean state

### Performance
- Temporary storage (tmpfs) for faster database operations
- Parallel test execution with race detection
- Optimized Docker layers for faster builds

### Convenience
- One-command test execution
- Automatic database setup and teardown
- Comprehensive logging and error reporting

## Troubleshooting

### Common Issues

**Docker not found**
```
❌ Docker is not installed or not in PATH
```
Solution: Install Docker Desktop or Docker Engine

**Port conflicts**
```
Error: port 5433 already in use
```
Solution: Stop other PostgreSQL instances or change the port in `docker-compose.test.yml`

**Build failures**
```
❌ Failed to build test image
```
Solution: Check Docker daemon is running and you have sufficient disk space

**Test timeouts**
```
❌ Database failed to become ready within timeout
```
Solution: Increase timeout in scripts or check Docker resource limits

### Debugging

```bash
# View detailed logs
make docker-test-logs

# Check container status
docker-compose -f docker-compose.test.yml ps

# Debug database connection
docker-compose -f docker-compose.test.yml exec postgres-test psql -U inventario_test -d inventario_test

# Run tests with custom command
docker-compose -f docker-compose.test.yml run --rm inventario-test go test -v ./registry/postgresql/... -run TestSpecificFunction
```

## Integration with CI/CD

This Docker testing setup can be easily integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Run Docker Tests
  run: |
    make docker-test-build
    make docker-test-go
    make docker-test-clean
```

## Performance Tips

1. **Use tmpfs**: Database runs in memory for faster I/O
2. **Layer caching**: Docker layers are optimized for build speed
3. **Parallel execution**: Tests run with race detection enabled
4. **Resource limits**: Adjust Docker resource limits for better performance

## Security Considerations

- Test database uses temporary storage (no data persistence)
- Test credentials are hardcoded (safe for testing)
- Containers run with minimal privileges
- Network isolation between test and production services
