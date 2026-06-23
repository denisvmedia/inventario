# CRUD Operation E2E Tests

This directory contains end-to-end tests that perform Create, Read, Update, and Delete (CRUD) operations on the main entities in the Inventario application.

## Test Files

The CRUD coverage that remains as standalone "simple CRUD" specs is:

- `commodity-simple-crud.spec.ts`: CRUD operations for commodities
- `locations-crud.spec.ts`: CRUD operations for locations

> Earlier iterations of this suite also shipped `location-crud`,
> `area-crud`, `commodity-crud`, `location-simple-crud`,
> `area-simple-crud`, and `basic-crud` specs. Those files have been
> removed; their scenarios are now covered by the broader feature specs
> in this directory (e.g. `commodity-bulk-and-filter.spec.ts`,
> `bulk-actions.spec.ts`, `groups.spec.ts`). Don't expect the old
> filenames to exist.

## Running the Tests

### Prerequisites

Make sure you have the application stack running:

```bash
# From the e2e directory
npm run stack
```

### Running the CRUD specs

```bash
# From the e2e directory

# Commodity CRUD (chromium only)
npm run test:commodity-crud

# Or run any spec directly
npx playwright test commodity-simple-crud.spec.ts --project=chromium
npx playwright test locations-crud.spec.ts --project=chromium
```

### Running with UI Mode

```bash
# From the e2e directory
npx playwright test commodity-simple-crud.spec.ts --ui
```

## Test Artifacts

The tests automatically take screenshots at key points during test execution. These screenshots are saved in the `test-results/screenshots` directory.

## Test Cleanup

Each test suite cleans up after itself by deleting the entities it creates. However, if a test fails before cleanup, you may need to manually delete test entities or reset the database.

## Notes

- The tests are designed to be run in sequence within each file, as later tests depend on entities created in earlier tests.
- Each test suite is independent of the others, so you can run them in any order or individually.
- The tests use timestamps in entity names to ensure uniqueness across test runs.
