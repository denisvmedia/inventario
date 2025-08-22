# User Isolation Testing Documentation

This document describes the comprehensive user isolation testing framework implemented for the Inventario application. The testing suite validates that users cannot access each other's data across all entities and operations.

## Overview

The user isolation testing framework consists of multiple layers of tests:

1. **Unit Tests** - Test individual components and functions
2. **Integration Tests** - Test user isolation at the registry and API level
3. **End-to-End Tests** - Test complete user workflows in browser contexts
4. **Performance Tests** - Validate performance under load with multiple users
5. **Security Tests** - Test edge cases and malicious input handling

## Test Structure

### Integration Tests

Located in `go/integration_test/`:

- `user_isolation_test.go` - Core user isolation tests for all entities
- `api_user_isolation_test.go` - API endpoint user isolation tests
- `user_isolation_comprehensive_test.go` - Complex scenarios and edge cases
- `user_isolation_performance_test.go` - Performance and load testing

### End-to-End Tests

Located in `e2e/tests/`:

- `user-isolation.spec.ts` - Browser-based user isolation tests
- `includes/multi-user-auth.ts` - Multi-user authentication helpers

### Test Helpers

- `go/registry/test_helpers.go` - Reusable test utilities and patterns

## Running Tests

### Prerequisites

1. **PostgreSQL Database**: Tests require a PostgreSQL database for integration testing
   ```bash
   # Set environment variable
   export POSTGRES_TEST_DSN="postgres://inventario:inventario_password@localhost:5433/inventario?sslmode=disable"
   ```

2. **Node.js and Playwright**: Required for E2E tests
   ```bash
   npm install
   npx playwright install
   ```

### Using Makefile Targets

The Makefile provides convenient targets for running user isolation tests:

```bash
# Run all user isolation tests
make test-user-isolation-all

# Run specific test types
make test-user-isolation              # Integration tests
make test-user-isolation-performance  # Performance benchmarks
make test-user-isolation-e2e          # End-to-end tests

# Set PostgreSQL DSN for integration tests
export POSTGRES_TEST_DSN="postgres://user:pass@localhost:5432/inventario_test?sslmode=disable"
make test-user-isolation
```

### Manual Test Execution

#### Unit Tests
```bash
go test github.com/denisvmedia/inventario/registry -v
go test github.com/denisvmedia/inventario/apiserver -v
```

#### Integration Tests
```bash
export POSTGRES_TEST_DSN="postgres://inventario:inventario_password@localhost:5433/inventario?sslmode=disable"
go test -tags=integration ./go/integration_test/user_isolation_test.go -v
go test -tags=integration ./go/integration_test/api_user_isolation_test.go -v
go test -tags=integration ./go/integration_test/user_isolation_comprehensive_test.go -v
```

#### Performance Tests
```bash
go test -tags=integration ./go/integration_test/user_isolation_performance_test.go -bench=. -benchmem -v
```

#### E2E Tests
```bash
npx playwright test e2e/tests/user-isolation.spec.ts
```

## Test Coverage

### Entities Tested

The user isolation tests cover all major entities:

- **Commodities** - Items in the inventory
- **Locations** - Physical locations
- **Areas** - Areas within locations
- **Files** - File uploads and attachments
- **Exports** - Data export operations
- **Users** - User management (admin operations)

### Operations Tested

For each entity, the following operations are tested:

- **Create** - Users can only create entities for themselves
- **Read** - Users can only read their own entities
- **Update** - Users can only update their own entities
- **Delete** - Users can only delete their own entities
- **List** - Users only see their own entities in lists
- **Search** - Search results are filtered by user

### Security Scenarios

- **SQL Injection** - Malicious user IDs with SQL injection attempts
- **XSS Attempts** - User IDs with script injection attempts
- **Path Traversal** - User IDs with path traversal attempts
- **Buffer Overflow** - Very long user IDs
- **Null/Empty Values** - Empty or null user contexts
- **Invalid Formats** - Malformed user IDs

### Performance Scenarios

- **Concurrent Users** - Multiple users operating simultaneously
- **Load Testing** - High volume of operations per user
- **Scalability** - Performance with large datasets
- **Memory Usage** - Memory consumption under load

## Test Patterns

### Standard User Isolation Pattern

```go
func TestEntityUserIsolation(t *testing.T) {
    c := qt.New(t)
    registrySet, cleanup := setupTestDatabase(t)
    defer cleanup()

    // Create two users
    user1 := createTestUser(c, registrySet, "user1@example.com")
    user2 := createTestUser(c, registrySet, "user2@example.com")

    ctx1 := withUserContext(context.Background(), user1.ID)
    ctx2 := withUserContext(context.Background(), user2.ID)

    // User1 creates entity
    entity := createTestEntity(user1.ID)
    created, err := registry.CreateWithUser(ctx1, entity)
    c.Assert(err, qt.IsNil)

    // User2 cannot access User1's entity
    _, err = registry.GetWithUser(ctx2, created.ID)
    c.Assert(err, qt.IsNotNil)

    // User2 cannot see User1's entity in list
    entities, err := registry.ListWithUser(ctx2)
    c.Assert(err, qt.IsNil)
    c.Assert(len(entities), qt.Equals, 0)
}
```

### API Isolation Pattern

```go
func testAPIIsolation(c *qt.C, server *httptest.Server, user1, user2 *models.User, jwtSecret string) {
    // Generate tokens
    token1, _ := generateJWTToken(user1, jwtSecret)
    token2, _ := generateJWTToken(user2, jwtSecret)

    // User1 creates entity
    resp, _ := makeAuthenticatedRequest("POST", server.URL+"/api/v1/entities", data, token1)
    c.Assert(resp.StatusCode, qt.Equals, http.StatusCreated)

    // User2 cannot see User1's entity
    resp, _ = makeAuthenticatedRequest("GET", server.URL+"/api/v1/entities", nil, token2)
    c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)
    
    var entities []map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&entities)
    c.Assert(len(entities), qt.Equals, 0)
}
```

### E2E Isolation Pattern

```typescript
test('Users cannot access each other\'s data', async ({ browser, page }) => {
    const users = await createTestUsers(page, 'test-name', 2);
    const userContexts = await setupUserContexts(browser, users);
    
    try {
        await loginAllUsers(userContexts);
        const [user1, user2] = userContexts;
        
        // User1 creates entity
        const entityId = await createEntityAsUser(user1, 'Test Entity');
        
        // User2 cannot see User1's entity
        await verifyUserCannotSeeContent(user2, 'Test Entity');
        
        // User1 can see their own entity
        await verifyUserCanSeeContent(user1, 'Test Entity');
        
    } finally {
        await cleanupUserContexts(userContexts);
    }
});
```

## Troubleshooting

### Common Issues

1. **Database Connection Failures**
   - Ensure PostgreSQL is running
   - Verify the DSN is correct
   - Check database permissions

2. **Test Timeouts**
   - Increase timeout values for slow systems
   - Check for database locks
   - Verify test data cleanup

3. **E2E Test Failures**
   - Ensure the application is running
   - Check browser compatibility
   - Verify test selectors are correct

### Debug Mode

Enable verbose logging for debugging:

```bash
# Go tests
go test -v -tags=integration ./go/integration_test/... 

# E2E tests
npx playwright test --reporter=verbose e2e/tests/user-isolation.spec.ts
```

## Continuous Integration

The user isolation tests are designed to run in CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Run User Isolation Tests
  run: |
    export POSTGRES_TEST_DSN="${{ secrets.POSTGRES_TEST_DSN }}"
    make test-user-isolation-all
```

## Contributing

When adding new entities or operations:

1. Add user isolation tests for all CRUD operations
2. Include API endpoint tests if applicable
3. Add E2E tests for user-facing features
4. Update this documentation

### Test Naming Conventions

- Unit tests: `TestEntityName_Operation_UserIsolation`
- Integration tests: `TestUserIsolation_EntityName`
- E2E tests: `Users cannot access each other's entity_name`
- Performance tests: `BenchmarkUserIsolation_Scenario`

## Security Considerations

The user isolation tests help ensure:

- **Data Privacy** - Users cannot access other users' data
- **Data Integrity** - Users cannot modify other users' data
- **System Security** - Malicious input is handled safely
- **Performance** - User isolation doesn't significantly impact performance

Regular execution of these tests helps maintain the security posture of the application and ensures that user data remains properly isolated.
