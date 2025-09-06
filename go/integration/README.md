# CLI Workflow Integration Test

This directory contains comprehensive integration tests that validate the complete workflow from fresh database setup through CLI operations to API access, simulating real CI pipeline scenarios.

## Overview

The CLI workflow integration test (`cli_workflow_integration_test.go`) validates the following scenario:

1. **Fresh Database Setup**: Run bootstrap operations and migrations on a clean database
2. **Authentication Failure Test**: Attempt to log in with non-existent user (should fail)
3. **CLI Tenant Creation**: Create a new tenant using the CLI command
4. **CLI User Creation**: Create a new user using the CLI command
5. **Authentication Success Test**: Attempt to log in with the created user (should succeed)
6. **API Access Test**: Access the system info API with a valid token (should succeed)

This test ensures that the complete workflow from CLI setup to API access works correctly in production environments.

## Test Structure

### Test File: `cli_workflow_integration_test.go`

The test is structured as a single comprehensive integration test that validates the entire workflow:

```go
func TestCLIWorkflowIntegration(t *testing.T)
```

### Key Components

#### Database Setup
- `setupFreshDatabase()`: Runs bootstrap and migration commands
- Uses the actual CLI commands to set up the database
- Validates that migrations complete successfully

#### CLI Command Testing
- `createTenantViaCLI()`: Creates tenants using the actual CLI command
- `createUserViaCLI()`: Creates users using the actual CLI command
- Commands are executed programmatically with output capture
- Validates command success and error handling

#### API Testing
- `setupAPIServer()`: Creates a test API server with proper authentication
- `attemptLogin()`: Tests authentication endpoints
- `getAuthToken()`: Retrieves JWT tokens for API access
- `getSystemInfo()`: Tests protected API endpoints

## Running the Tests

### Prerequisites

1. **PostgreSQL Database**: A PostgreSQL database must be available for testing
2. **Environment Variable**: Set `POSTGRES_TEST_DSN` with the database connection string

### Local Testing

#### Using Go Test Directly

```bash
# Set the PostgreSQL DSN
export POSTGRES_TEST_DSN="postgres://user:password@localhost:5432/test_db?sslmode=disable"

# Run the integration test
cd go
go test -tags=integration ./integration_test/cli_workflow_integration_test.go -v
```

#### Using the Test Scripts

**Linux/macOS:**
```bash
export POSTGRES_TEST_DSN="postgres://user:password@localhost:5432/test_db?sslmode=disable"
./scripts/run-integration-tests.sh
```

**Windows (PowerShell):**
```powershell
$env:POSTGRES_TEST_DSN="postgres://user:password@localhost:5432/test_db?sslmode=disable"
.\scripts\run-integration-tests.ps1
```

### CI/CD Integration

The test is designed to run in CI pipelines with the provided GitHub Actions workflow:

```yaml
# .github/workflows/cli-integration-test.yml
```

The workflow:
- Sets up a PostgreSQL service
- Configures the test environment
- Runs the integration test
- Uploads test results as artifacts

## Test Scenarios

### 1. Fresh Database Setup
- Validates that bootstrap operations complete successfully
- Ensures database migrations run without errors
- Verifies that the database is in a clean, ready state

### 2. Authentication Flow
- **Negative Test**: Confirms that login fails for non-existent users
- **Positive Test**: Validates that login succeeds for created users
- **Token Validation**: Ensures JWT tokens are properly generated and valid

### 3. CLI Operations
- **Tenant Creation**: Tests the complete tenant creation workflow
- **User Creation**: Tests the complete user creation workflow
- **Command Integration**: Validates that CLI commands work with the database
- **Error Handling**: Ensures proper error messages and validation

### 4. API Access
- **Protected Endpoints**: Tests that authentication is required
- **System Information**: Validates that system info API returns correct data
- **Token Usage**: Confirms that JWT tokens provide proper API access

## Expected Outcomes

### Successful Test Run

When the test passes, it validates:

✅ **Database Setup**: Bootstrap and migrations complete successfully  
✅ **CLI Commands**: Tenant and user creation work correctly  
✅ **Authentication**: Login flow works with created users  
✅ **API Access**: Protected endpoints are accessible with valid tokens  
✅ **End-to-End Flow**: Complete workflow from setup to API access  

### Test Output Example

```
=== RUN   TestCLIWorkflowIntegration
    cli_workflow_integration_test.go:35: 🔧 Setting up fresh database with bootstrap and migrations...
    cli_workflow_integration_test.go:40: 🔐 Testing login with non-existent user (should fail)...
    cli_workflow_integration_test.go:45: 🏢 Creating tenant via CLI...
    cli_workflow_integration_test.go:49: 👤 Creating user via CLI...
    cli_workflow_integration_test.go:54: 🔐 Testing login with created user (should succeed)...
    cli_workflow_integration_test.go:59: 🎫 Getting authentication token...
    cli_workflow_integration_test.go:64: 📊 Testing system info API access with valid token...
    cli_workflow_integration_test.go:68: ✅ CLI workflow integration test completed successfully!
--- PASS: TestCLIWorkflowIntegration (2.34s)
PASS
```

## Troubleshooting

### Common Issues

#### Database Connection Errors
```
Error: failed to connect to database: dial tcp: connection refused
```
**Solution**: Ensure PostgreSQL is running and the DSN is correct

#### Migration Failures
```
Error: migration failed: relation already exists
```
**Solution**: Use a fresh database or ensure proper cleanup between test runs

#### Authentication Failures
```
Error: login failed with status: 401
```
**Solution**: Check that user creation completed successfully and credentials are correct

#### API Access Errors
```
Error: system info request failed with status: 403
```
**Solution**: Verify that JWT token is valid and properly formatted

### Debug Mode

To enable verbose output and debugging:

```bash
go test -tags=integration ./integration_test/cli_workflow_integration_test.go -v -test.v
```

## Integration with CI/CD

This test is designed to be the definitive validation for CI/CD pipelines, ensuring that:

1. **Deployment Readiness**: The system can be set up from scratch
2. **CLI Functionality**: All administrative commands work correctly
3. **API Compatibility**: The API layer functions properly with authentication
4. **End-to-End Validation**: The complete user journey works as expected

The test serves as a comprehensive smoke test for production deployments and can be used to validate that all components of the system are working together correctly.

## Maintenance

### Updating the Test

When adding new CLI commands or API endpoints:

1. Add corresponding test functions following the existing patterns
2. Update the main test flow to include new validations
3. Ensure proper cleanup and error handling
4. Update this documentation with new test scenarios

### Performance Considerations

The test is designed to be comprehensive but efficient:
- Uses in-memory file storage for uploads
- Minimizes database operations
- Reuses API server instances where possible
- Includes appropriate timeouts for CI environments
