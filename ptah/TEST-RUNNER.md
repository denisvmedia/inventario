# Ptah Test Runner - Complete Guide

## 🎯 **Single Unified Test Runner**

All testing is now handled by **one comprehensive test runner** with three convenient wrappers:

| File | Platform | Usage |
|------|----------|-------|
| `test-ptah.ps1` | PowerShell (Windows/Linux/macOS) | Main test runner |
| `test-ptah.cmd` | Windows CMD | Simple wrapper |
| `test-ptah.sh` | Bash (Linux/macOS) | Bash wrapper |

## 🚀 **Quick Commands**

### **Windows (CMD)**
```cmd
REM Run all tests with databases
test-ptah.cmd

REM Unit tests only (fast, no databases)
test-ptah.cmd unit

REM Specific test pattern
test-ptah.cmd pattern TestDropIndex

REM Specific package
test-ptah.cmd package core/renderer

REM Keep databases for debugging
test-ptah.cmd keep
```

### **PowerShell (Cross-platform)**
```powershell
# Run all tests with databases
.\test-ptah.ps1

# Unit tests only (fast, no databases)
.\test-ptah.ps1 -UnitOnly

# Specific test pattern
.\test-ptah.ps1 -Pattern "TestDropIndex"

# Specific package
.\test-ptah.ps1 -Package "core/renderer"

# Keep databases for debugging
.\test-ptah.ps1 -KeepDatabases
```

### **Bash (Linux/macOS)**
```bash
# Run all tests with databases
./test-ptah.sh

# Unit tests only (fast, no databases)
./test-ptah.sh unit

# Specific test pattern
./test-ptah.sh pattern TestDropIndex

# Specific package
./test-ptah.sh package core/renderer

# Keep databases for debugging
./test-ptah.sh keep
```

## 📋 **What Gets Tested**

### **Recursive Test Coverage**
- ✅ **All Go packages** in ptah directory recursively
- ✅ **Core packages**: ast, astbuilder, renderer, parser, lexer, etc.
- ✅ **Migration packages**: planner, schemadiff, migrator, etc.
- ✅ **Database packages**: dbschema, executor, etc.
- ✅ **Schema packages**: differ, parser, transform, etc.
- ✅ **Integration tests** (optional, can be skipped)

### **New Visitor Methods Tested**
- ✅ **DropIndex**: PostgreSQL, MySQL, MariaDB implementations
- ✅ **CreateType**: ENUM, DOMAIN, COMPOSITE types (PostgreSQL)
- ✅ **AlterType**: ADD VALUE, RENAME VALUE, RENAME TO operations

## 🔧 **Features**

### **Automatic Database Management**
- ✅ **Docker Compose**: Automatically starts PostgreSQL, MySQL, MariaDB
- ✅ **Health Checks**: Waits for databases to be ready
- ✅ **Environment Setup**: Sets connection strings automatically
- ✅ **Cleanup**: Stops and removes containers after tests

### **Verbose Output**
- ✅ **Detailed Test Names**: Shows exactly which tests are running
- ✅ **Progress Indicators**: Real-time test execution feedback
- ✅ **Colored Output**: PowerShell native colors for better readability
- ✅ **Timing Information**: Test duration and performance metrics

### **Flexible Options**
- ✅ **Unit Tests Only**: Skip database setup for fast testing
- ✅ **Pattern Matching**: Run specific tests by name pattern
- ✅ **Package Filtering**: Test specific packages only
- ✅ **Database Persistence**: Keep databases running for debugging
- ✅ **Integration Skipping**: Exclude integration folder if needed

## 📊 **Example Output**

```
======================================================================
  Ptah Comprehensive Test Runner
======================================================================

Mode: Full Integration Tests
Pattern: All tests
Package: All packages
Timeout: 10 minutes

[STEP] Checking prerequisites...
[OK] Go found: go version go1.24.3 windows/amd64
[OK] Docker and docker-compose found

[STEP] Starting databases (PostgreSQL, MySQL, MariaDB)...
[STEP] Waiting for databases to be healthy...
[OK] All databases are healthy!

[STEP] Setting up test environment variables...
  POSTGRES_TEST_DSN = postgres://ptah_user:ptah_password@localhost:5432/ptah_test?sslmode=disable
  MYSQL_TEST_DSN = ptah_user:ptah_password@tcp(localhost:3310)/ptah_test
  MARIADB_TEST_DSN = ptah_user:ptah_password@tcp(localhost:3307)/ptah_test

--------------------------------------------------
  Running Go Tests
--------------------------------------------------

Testing all packages recursively

[STEP] Executing: go test ./... -v -timeout 10m

=== RUN   TestNewVisitorMethods_UnitTests
=== RUN   TestNewVisitorMethods_UnitTests/postgresql
=== RUN   TestNewVisitorMethods_UnitTests/postgresql/DropIndex
=== RUN   TestNewVisitorMethods_UnitTests/postgresql/CreateType
=== RUN   TestNewVisitorMethods_UnitTests/postgresql/AlterType
--- PASS: TestNewVisitorMethods_UnitTests/postgresql/DropIndex (0.00s)
--- PASS: TestNewVisitorMethods_UnitTests/postgresql/CreateType (0.00s)
--- PASS: TestNewVisitorMethods_UnitTests/postgresql/AlterType (0.00s)
...

Test execution completed in 02:15

======================================================================
  Test Results
======================================================================

[OK] All tests passed!
Total duration: 02:30
```

## 🛠 **Troubleshooting**

### **Common Issues**

1. **Docker not running:**
   ```bash
   # Check Docker status
   docker --version
   docker-compose --version
   ```

2. **Permission issues (Linux/macOS):**
   ```bash
   chmod +x test-ptah.sh
   ```

3. **PowerShell execution policy (Windows):**
   ```powershell
   Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
   ```

### **Environment Variables**
The test runner automatically sets these for integration tests:
```
POSTGRES_TEST_DSN=postgres://ptah_user:ptah_password@localhost:5432/ptah_test?sslmode=disable
MYSQL_TEST_DSN=ptah_user:ptah_password@tcp(localhost:3310)/ptah_test
MARIADB_TEST_DSN=ptah_user:ptah_password@tcp(localhost:3307)/ptah_test
```

## 🎯 **Development Workflow**

```bash
# 1. Quick unit tests during development (30 seconds)
test-ptah.cmd unit

# 2. Test specific functionality you're working on
test-ptah.cmd pattern TestDropIndex

# 3. Test specific package
test-ptah.cmd package core/renderer

# 4. Full test suite before commit (2-3 minutes)
test-ptah.cmd

# 5. Keep databases for debugging
test-ptah.cmd keep
```

## ✅ **Success Indicators**

When all tests pass, you'll see:
- ✅ All individual test names with `PASS` status
- ✅ No `FAIL` or `ERROR` messages
- ✅ Final `[OK] All tests passed!` message
- ✅ Clean database startup and shutdown

## 🎉 **Benefits**

- **Single Command**: One command runs everything
- **Cross-Platform**: Works on Windows, Linux, macOS
- **Verbose Output**: See exactly what's being tested
- **Fast Unit Tests**: Skip databases when not needed
- **Automatic Setup**: No manual database configuration
- **Clean Cleanup**: No leftover containers
- **Flexible Filtering**: Test exactly what you need
