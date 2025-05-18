# CRUD Operation E2E Tests

This directory contains end-to-end tests that perform Create, Read, Update, and Delete (CRUD) operations on the main entities in the Inventario application.

## Test Files

### Original CRUD Tests (Complex)

- `location-crud.spec.ts`: Tests CRUD operations for locations
- `area-crud.spec.ts`: Tests CRUD operations for areas
- `commodity-crud.spec.ts`: Tests CRUD operations for commodities

### Simplified CRUD Tests (Recommended)

- `location-simple-crud.spec.ts`: Simplified CRUD operations for locations
- `area-simple-crud.spec.ts`: Simplified CRUD operations for areas
- `commodity-simple-crud.spec.ts`: Simplified CRUD operations for commodities
- `basic-crud.spec.ts`: Basic create operations for all entities

## Running the Tests

### Prerequisites

Make sure you have the application stack running:

```bash
# From the e2e directory
npm run stack
```

### Running All CRUD Tests

```bash
# From the e2e directory
# Run original CRUD tests
npm run test:crud

# Run simplified CRUD tests (recommended)
npm run test:simple-crud
```

### Running Individual CRUD Tests

```bash
# Run only location CRUD tests
npm run test:location-crud

# Run only area CRUD tests
npm run test:area-crud

# Run only commodity CRUD tests
npm run test:commodity-crud

# Run only basic CRUD tests
npm run test:basic-crud
```

### Running with UI Mode

```bash
# From the e2e directory
npx playwright test location-crud.spec.ts --ui
```

## Test Artifacts

The tests automatically take screenshots at key points during test execution. These screenshots are saved in the `test-results/screenshots` directory.

## Test Cleanup

Each test suite cleans up after itself by deleting the entities it creates. However, if a test fails before cleanup, you may need to manually delete test entities or reset the database.

## Notes

- The tests are designed to be run in sequence within each file, as later tests depend on entities created in earlier tests.
- Each test suite is independent of the others, so you can run them in any order or individually.
- The tests use timestamps in entity names to ensure uniqueness across test runs.
