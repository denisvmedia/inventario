# PostgreSQL-Centric E2E Tests

This document describes the PostgreSQL-centric setup for end-to-end tests in the Inventario project.

## Overview

The e2e tests are now configured to use PostgreSQL as the primary database backend, ensuring that tests run against the same database type used in production. This provides better test coverage and catches PostgreSQL-specific issues early.

## Prerequisites

- **Docker**: Required to run the PostgreSQL container
- **Node.js**: For running the test framework
- **Go**: For building and running the backend

## Architecture

The e2e test setup consists of:

1. **PostgreSQL Container**: Automatically started before tests
2. **Backend Server**: Runs with PostgreSQL connection
3. **Frontend Server**: Serves the Vue.js application
4. **Test Runner**: Playwright tests with database utilities

## Configuration

### Environment Variables

You can customize the PostgreSQL configuration using these environment variables:

```bash
# Database configuration (optional - defaults provided)
export E2E_POSTGRES_DB=inventario_e2e
export E2E_POSTGRES_USER=inventario_e2e
export E2E_POSTGRES_PASSWORD=inventario_e2e_password
export E2E_POSTGRES_PORT=5433  # Different from production to avoid conflicts
```

### Default Configuration

If no environment variables are set, the following defaults are used:

- **Database**: `inventario_e2e`
- **User**: `inventario_e2e`
- **Password**: `inventario_e2e_password`
- **Port**: `5433` (to avoid conflicts with production PostgreSQL on 5432)

## Running Tests

### Local Development

```bash
cd e2e
npm test
```

This will:
1. Start a PostgreSQL container (Docker required)
2. Start the backend with PostgreSQL connection
3. Start the frontend server
4. Run all tests
5. Clean up containers and processes

### GitHub Actions (CI)

The tests automatically detect CI environment and use the PostgreSQL service instead of starting a container:

1. PostgreSQL service runs as a GitHub Actions service
2. Tests connect to the service directly
3. No Docker container management needed
4. Automatic cleanup by GitHub Actions

### Development Mode

For development, you can start the stack manually and run tests against it:

```bash
# Terminal 1: Start the stack
cd e2e
npm run start-stack

# Terminal 2: Run tests (in another terminal)
cd e2e
npm run test:headed  # or any other test command
```

### Individual Test Files

```bash
cd e2e
npx playwright test tests/commodity-simple-crud.spec.ts
```

## Database Management

### Automatic Database Reset

Each test automatically resets the database to ensure a clean state:

```typescript
import { resetAndSeedDatabase } from '../utils/database.js';

test.beforeEach(async () => {
  await resetAndSeedDatabase();
});
```

### Manual Database Operations

You can also use database utilities manually in tests:

```typescript
import { cleanDatabase, seedTestData, isDatabaseReady } from '../utils/database.js';

// Clean database without seeding
await cleanDatabase();

// Seed test data
await seedTestData();

// Check if database is ready
const ready = await isDatabaseReady();
```

## Container Management

### Automatic Management

The test framework automatically:
- Starts a unique PostgreSQL container for each test run
- Waits for PostgreSQL to be ready
- Cleans up containers after tests complete

### Manual Container Management

If you need to manually manage containers:

```bash
# List running containers
docker ps | grep inventario-e2e-postgres

# Stop a specific container
docker stop inventario-e2e-postgres-<timestamp>
docker rm inventario-e2e-postgres-<timestamp>

# Clean up all test containers
docker ps -a | grep inventario-e2e-postgres | awk '{print $1}' | xargs docker rm -f
```

## Troubleshooting

### Docker Not Available

If you get "Docker is not available" error:
1. Install Docker Desktop or Docker Engine
2. Ensure Docker daemon is running
3. Verify with: `docker --version`

### Port Conflicts

If port 5433 is already in use:
1. Set a different port: `export E2E_POSTGRES_PORT=5434`
2. Or stop the conflicting service

### Container Startup Issues

If PostgreSQL container fails to start:
1. Check Docker logs: `docker logs inventario-e2e-postgres-<timestamp>`
2. Ensure sufficient disk space
3. Check for port conflicts

### Database Connection Issues

If backend can't connect to PostgreSQL:
1. Verify container is running: `docker ps | grep postgres`
2. Check container health: `docker exec <container> pg_isready -U inventario_e2e`
3. Review backend logs for connection errors

## GitHub Actions Integration

The e2e tests are fully integrated with GitHub Actions and will automatically:

### Service Configuration

The workflow uses a PostgreSQL service container:

```yaml
services:
  postgres:
    image: postgres:17-alpine
    env:
      POSTGRES_DB: inventario_e2e
      POSTGRES_USER: inventario_e2e
      POSTGRES_PASSWORD: inventario_e2e_password
    options: >-
      --health-cmd pg_isready
      --health-interval 10s
      --health-timeout 5s
      --health-retries 5
    ports:
      - 5433:5432
```

### Automatic Detection

The test setup automatically detects CI environment:
- **Local**: Starts Docker container
- **CI**: Uses GitHub Actions PostgreSQL service
- **No configuration changes needed**

### Workflow Steps

1. Set up PostgreSQL service
2. Install PostgreSQL client tools
3. Set environment variables
4. Run tests with PostgreSQL backend

## Benefits of PostgreSQL-Centric Testing

1. **Production Parity**: Tests run against the same database type as production
2. **Feature Coverage**: Tests PostgreSQL-specific features like JSONB, full-text search
3. **Performance Testing**: Identifies PostgreSQL-specific performance issues
4. **Migration Testing**: Validates database migrations work correctly
5. **Constraint Testing**: Tests PostgreSQL constraints and triggers
6. **CI/CD Integration**: Seamless integration with GitHub Actions

## Migration from Memory Database

Previous tests used in-memory database. Key changes:

1. **Database Persistence**: Data persists between requests (until reset)
2. **Transaction Behavior**: PostgreSQL transaction semantics apply
3. **Constraint Enforcement**: PostgreSQL constraints are enforced
4. **Performance Characteristics**: Different performance profile than memory

## Best Practices

1. **Always Reset Database**: Use `resetAndSeedDatabase()` in `beforeEach`
2. **Unique Test Data**: Use timestamps or UUIDs for test data
3. **Clean Assertions**: Don't rely on data from other tests
4. **Resource Cleanup**: Let the framework handle container cleanup
5. **Error Handling**: Check for "Settings Required" page in tests

## File Structure

```
e2e/
├── setup/
│   ├── setup-stack.ts      # PostgreSQL + backend + frontend setup
│   ├── global-setup.ts     # Test environment initialization
│   └── global-teardown.ts  # Cleanup after all tests
├── utils/
│   └── database.ts         # Database utility functions
├── tests/
│   └── *.spec.ts          # Test files with PostgreSQL support
└── README-POSTGRES.md     # This documentation
```
