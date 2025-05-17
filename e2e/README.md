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

### Running Tests with Screenshots and Videos

```bash
# From the e2e directory
npm run test:record
```

This will run all tests with video and screenshot recording enabled. The artifacts will be saved in the `test-results` directory.

### Running Specific Test Examples

```bash
npm run test:screenshots   # Run screenshot examples
npm run test:recorder      # Run recorder fixture examples
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

## Working with Screenshots and Videos

### Automatic Screenshots and Videos

The Playwright configuration (`playwright.config.ts`) is set up to automatically:
- Take screenshots for all tests (`screenshot: 'on'`)
- Record videos for all tests (`video: 'on'`)

These artifacts are saved in the `test-results` directory.

### Manual Screenshots

You can take manual screenshots in your tests using the `page.screenshot()` method:

```typescript
await page.screenshot({
  path: 'test-results/screenshots/my-screenshot.png',
  fullPage: true
});
```

### Using the TestRecorder Helper

We've created a `TestRecorder` helper class to make it easier to take screenshots:

```typescript
import { TestRecorder } from '../utils/test-recorder';

// Create a recorder
const recorder = new TestRecorder(page, 'My Test');

// Take a full page screenshot
await recorder.takeScreenshot('page-name');

// Take a screenshot of a specific element
await recorder.takeElementScreenshot('h1', 'header');
```

### Using the Recorder Fixture

The `recorder` fixture is available in all tests:

```typescript
test('my test', async ({ page, recorder }) => {
  await page.goto('/');
  await recorder.takeScreenshot('home-page');
});
```

### Cleaning Up Artifacts

To clean up all test artifacts:

```bash
npm run clean:artifacts
```

### Example Files

- `screenshots-example.spec.ts` - Examples of taking manual screenshots
- `recorder-example.spec.ts` - Examples of using the TestRecorder helper
- `fixture-recorder.spec.ts` - Examples of using the recorder fixture
- `conditional-screenshots.spec.ts` - Examples of taking screenshots based on conditions
