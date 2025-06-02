# Ptah Core Renderer Testing - Quick Reference

## 🚀 **Quick Start Commands**

```powershell
# Run unit tests only (fast, no databases needed)
.\test-simple.ps1

# Run specific test with detailed output
.\run-integration-tests.ps1 -TestPattern "TestNewVisitorMethods_UnitTests" -Verbose

# Run all new visitor methods tests
.\test-new-methods.ps1

# Run comprehensive test suite
.\test-all.ps1
```

## 📋 **Available Scripts**

| Script | Purpose | Speed | Database Required |
|--------|---------|-------|-------------------|
| `test-simple.ps1` | Unit tests with detailed output | ⚡ Fast | ❌ No |
| `run-integration-tests.ps1` | Full integration tests | 🐌 Slow | ✅ Yes |
| `test-new-methods.ps1` | New visitor methods only | 🚀 Medium | ✅ Yes |
| `test-all.ps1` | Comprehensive test suite | 🐌 Slow | ✅ Yes |

## 🧪 **Test Categories**

### **Unit Tests** (No Database)
```powershell
# AST tests - test the AST node functionality
go test -C core/ast -v

# Core renderer unit tests - test new visitor methods
go test -C core/renderer -run "TestNewVisitorMethods_UnitTests" -v
```

### **Integration Tests** (With Database)
```powershell
# DROP INDEX tests across all dialects
.\run-integration-tests.ps1 -TestPattern "TestDropIndex_Integration" -Verbose

# CREATE TYPE tests across all dialects  
.\run-integration-tests.ps1 -TestPattern "TestCreateType_Integration" -Verbose

# ALTER TYPE tests across all dialects
.\run-integration-tests.ps1 -TestPattern "TestAlterType_Integration" -Verbose
```

## 🎯 **New Visitor Methods Test Results**

### **What Gets Tested:**

#### **DropIndex Tests:**
- ✅ PostgreSQL: `DROP INDEX [IF EXISTS] index_name;`
- ✅ MySQL: `DROP INDEX index_name ON table_name;`
- ✅ MariaDB: `DROP INDEX index_name ON table_name;`

#### **CreateType Tests:**
- ✅ PostgreSQL: `CREATE TYPE name AS ENUM (...)`, `CREATE DOMAIN ...`, `CREATE TYPE ... AS (...)`
- ✅ MySQL/MariaDB: Informative comments explaining inline enum handling

#### **AlterType Tests:**
- ✅ PostgreSQL: `ALTER TYPE name ADD VALUE ...`, `RENAME VALUE ...`, `RENAME TO ...`
- ✅ MySQL/MariaDB: Informative comments explaining `ALTER TABLE MODIFY COLUMN` usage

## 🔧 **Troubleshooting**

### **Common Issues:**

1. **Docker not running:**
   ```powershell
   # Check Docker status
   docker --version
   docker-compose --version
   ```

2. **Tests not showing detailed output:**
   ```powershell
   # Use the simple test script for detailed unit test output
   .\test-simple.ps1
   
   # Use verbose flag for integration tests
   .\run-integration-tests.ps1 -TestPattern "TestName" -Verbose
   ```

3. **Database connection issues:**
   ```powershell
   # Check database health
   docker-compose ps
   
   # View database logs
   docker-compose logs postgres mysql mariadb
   ```

4. **PowerShell coloring issues:**
   - The scripts now use PowerShell native colors (`-ForegroundColor`)
   - If colors don't work, the tests will still run correctly

### **Environment Variables:**
```powershell
$env:POSTGRES_TEST_DSN = "postgres://ptah_user:ptah_password@localhost:5432/ptah_test?sslmode=disable"
$env:MYSQL_TEST_DSN = "ptah_user:ptah_password@tcp(localhost:3310)/ptah_test"
$env:MARIADB_TEST_DSN = "ptah_user:ptah_password@tcp(localhost:3307)/ptah_test"
```

## 📊 **Test Coverage Summary**

### **Implemented Visitor Methods:**
- ✅ `VisitDropIndex` - Complete cross-dialect support
- ✅ `VisitCreateType` - PostgreSQL full support, MySQL/MariaDB compatibility
- ✅ `VisitAlterType` - PostgreSQL full support, MySQL/MariaDB compatibility

### **Test Types:**
- ✅ **Unit Tests** - Logic validation without databases
- ✅ **Integration Tests** - Real database execution
- ✅ **Cross-Dialect Tests** - PostgreSQL, MySQL, MariaDB compatibility
- ✅ **Error Handling Tests** - Edge cases and error conditions

### **Database Versions Tested:**
- ✅ PostgreSQL 16
- ✅ MySQL 8.0
- ✅ MariaDB 10.11

## 🎉 **Success Indicators**

When tests pass, you'll see:
```
=== RUN   TestNewVisitorMethods_UnitTests/postgresql/DropIndex
--- PASS: TestNewVisitorMethods_UnitTests/postgresql/DropIndex (0.00s)
=== RUN   TestNewVisitorMethods_UnitTests/postgresql/CreateType  
--- PASS: TestNewVisitorMethods_UnitTests/postgresql/CreateType (0.00s)
=== RUN   TestNewVisitorMethods_UnitTests/postgresql/AlterType
--- PASS: TestNewVisitorMethods_UnitTests/postgresql/AlterType (0.00s)
```

All tests should show `PASS` status with no failures or errors.

## 🚀 **Development Workflow**

```powershell
# 1. Quick unit tests during development
.\test-simple.ps1

# 2. Test specific functionality
.\run-integration-tests.ps1 -TestPattern "TestDropIndex" -Verbose

# 3. Test all new methods
.\test-new-methods.ps1

# 4. Full test suite before commit
.\test-all.ps1
```
