# End-to-End Testing for Inventario

This directory contains end-to-end tests for the Inventario application using Playwright.

## Directory Structure

```
e2e/
├── fixtures/       # Test fixtures and helpers
├── setup/          # Setup scripts for the test environment
├── tests/          # Test files
├── playwright.config.ts  # Playwright configuration
├── tsconfig.json   # TypeScript configuration
└── package.json    # E2E-specific dependencies and scripts
```

## Running Tests

### Prerequisites

- Node.js (v18 or later)
- npm (v8 or later)
- Go (v1.24 or later)

### Installation

```bash
# From the root directory
cd e2e
npm install
npm run install-browsers
```

### Running the Tests

1. Start the application stack (backend + frontend):

```bash
# From the e2e directory
npm run stack
```

This will:
- Start the Go backend server
- Seed the database with test data
- Start the Vue.js frontend

2. In a separate terminal, run the tests:

```bash
# From the e2e directory
npm run test
```

### Running Tests with UI

```bash
# From the e2e directory
npm run ui
```

### Viewing Test Reports

```bash
# From the e2e directory
npm run report
```

## Writing Tests

Tests are written using Playwright's test framework. See the [Playwright documentation](https://playwright.dev/docs/intro) for more information.

Example test:

```typescript
import { test, expect } from '@playwright/test';

test('should navigate to the home page', async ({ page }) => {
  await page.goto('/');
  await expect(page.locator('h1')).toContainText('Welcome to Inventario');
});
```
