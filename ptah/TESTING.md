# Ptah Core Renderer Testing Guide

This directory contains PowerShell scripts for running comprehensive tests of the ptah core renderer, including the newly implemented visitor methods for `DropIndex`, `CreateType`, and `AlterType`.

## Prerequisites

- **Docker Desktop** - Required for running database integration tests
- **Go 1.19+** - For running the tests
- **PowerShell 7+** - For running the test scripts (works on Windows, macOS, Linux)

## Test Scripts

### 🚀 Quick Start

```powershell
# Run all tests (recommended)
.\test-all.ps1

# Run only the new visitor methods tests
.\test-new-methods.ps1

# Run unit tests only (no databases needed)
.\test-all.ps1 -Category unit
```

### 📋 Available Scripts

| Script | Purpose | Database Required |
|--------|---------|-------------------|
| `test-all.ps1` | Comprehensive test runner with multiple options | Conditional |
| `test-new-methods.ps1` | Quick test for new visitor methods | Yes |
| `run-integration-tests.ps1` | Full integration test runner | Yes |

## Script Details

### `test-all.ps1` - Comprehensive Test Runner

The main test script with multiple categories and options:

```powershell
# Run all tests
.\test-all.ps1

# Run only unit tests (fast, no databases)
.\test-all.ps1 -Category unit

# Run integration tests for specific dialect
.\test-all.ps1 -Category integration -Dialect postgresql

# Run essential tests only (faster)
.\test-all.ps1 -Quick

# Test specific dialect
.\test-all.ps1 -Dialect mysql
```

**Categories:**
- `unit` - Unit tests only (AST, renderer logic)
- `integration` - Integration tests with real databases
- `new-methods` - Tests for DropIndex, CreateType, AlterType
- `all` - All test categories (default)

**Dialects:**
- `postgresql` - PostgreSQL tests only
- `mysql` - MySQL tests only  
- `mariadb` - MariaDB tests only
- `all` - All dialects (default)

### `test-new-methods.ps1` - New Visitor Methods

Quick test script focusing on the newly implemented visitor methods:

```powershell
# Test new methods
.\test-new-methods.ps1

# Test and keep databases running for debugging
.\test-new-methods.ps1 -KeepDatabases
```

Tests the following functionality:
- `VisitDropIndex` - DROP INDEX statements
- `VisitCreateType` - CREATE TYPE statements (enums, domains, composite types)
- `VisitAlterType` - ALTER TYPE statements (add values, rename, etc.)

### `run-integration-tests.ps1` - Full Integration Runner

Detailed integration test runner with Docker Compose:

```powershell
# Run all integration tests
.\run-integration-tests.ps1

# Run specific test pattern
.\run-integration-tests.ps1 -TestPattern "TestDropIndex"

# Verbose output and keep databases
.\run-integration-tests.ps1 -Verbose -KeepDatabases

# Skip Docker image building
.\run-integration-tests.ps1 -SkipBuild
```

## Database Setup

The integration tests use Docker Compose to start:

- **PostgreSQL 16** on port 5432
- **MySQL 8.0** on port 3310  
- **MariaDB 10.11** on port 3307

Connection details:
- Database: `ptah_test`
- Username: `ptah_user`
- Password: `ptah_password`

## Test Coverage

### New Visitor Methods Tests

✅ **DropIndex Tests:**
- PostgreSQL: `DROP INDEX [IF EXISTS] name;`
- MySQL/MariaDB: `DROP INDEX name ON table;`

✅ **CreateType Tests:**
- PostgreSQL: Full support for ENUM, DOMAIN, COMPOSITE types
- MySQL/MariaDB: Informative comments (types handled inline)

✅ **AlterType Tests:**
- PostgreSQL: ADD VALUE, RENAME VALUE, RENAME TYPE operations
- MySQL/MariaDB: Informative comments (use ALTER TABLE instead)

### Integration Test Categories

1. **Unit Tests** - No database required
   - AST node functionality
   - Renderer interface compliance
   - SQL generation logic

2. **Integration Tests** - Real database connections
   - Actual SQL execution
   - Database-specific syntax validation
   - Cross-dialect compatibility

3. **Dialect-Specific Tests**
   - PostgreSQL advanced features
   - MySQL/MariaDB compatibility
   - Error handling and edge cases

## Troubleshooting

### Docker Issues

```powershell
# Check Docker status
docker --version
docker-compose --version

# View database logs
docker-compose logs postgres mysql mariadb

# Clean up containers
docker-compose down -v
```

### Test Failures

```powershell
# Run with verbose output
.\run-integration-tests.ps1 -Verbose

# Keep databases for debugging
.\run-integration-tests.ps1 -KeepDatabases

# Test specific pattern
.\run-integration-tests.ps1 -TestPattern "TestDropIndex"
```

### Performance

```powershell
# Quick tests only
.\test-all.ps1 -Quick

# Unit tests only (fastest)
.\test-all.ps1 -Category unit

# Specific dialect only
.\test-all.ps1 -Dialect postgresql
```

## Examples

### Development Workflow

```powershell
# 1. Quick unit tests during development
.\test-all.ps1 -Category unit

# 2. Test new functionality
.\test-new-methods.ps1

# 3. Full integration test before commit
.\test-all.ps1

# 4. Test specific database if issues found
.\test-all.ps1 -Dialect mysql -Verbose
```

### CI/CD Pipeline

```powershell
# Comprehensive test suite for CI
.\test-all.ps1 -Category all -Verbose

# Quick smoke test
.\test-all.ps1 -Quick
```

## Exit Codes

- `0` - All tests passed
- `1` - Test failures or errors
- `2` - Script parameter errors
- `3` - Docker/environment issues

All scripts return appropriate exit codes for CI/CD integration.
