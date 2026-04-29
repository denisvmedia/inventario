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
- Go (v1.26 or later)

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
- Start the Vue.js frontend (legacy)

To run the React (new) frontend instead, set `INVENTARIO_FRONTEND=new` before bringing up the stack — the Go binary picks the embedded bundle at startup. See [Project layout](#project-layout-legacy-vs-new) below.

2. In a separate terminal, run the tests for a specific frontend × browser combination:

```bash
# From the e2e directory
npx playwright test --project=legacy-chromium    # 22 existing specs against Vue
npx playwright test --project=new-chromium       # @react-only specs against React
```

`npm run test` runs every project, which today means both frontends. If only the legacy stack is up, the `new-*` specs will fail; bring up the matching stack first.

## Project layout (legacy vs new)

The Playwright config defines projects as `<frontend>-<browser>` pairs:

- `legacy-chromium` / `legacy-firefox` / `legacy-webkit` — production Vue frontend. Default home for all 22 existing specs.
- `new-chromium` / `new-firefox` / `new-webkit` — the React rewrite (#1397). Runs only specs tagged `@react-only` (or `@react-ready`); legacy-tagged specs are excluded.

Spec gating uses Playwright tag-grep:

| Tag             | Runs on   |
| --------------- | --------- |
| `@react-only`   | `new-*`   |
| `@legacy-only`  | `legacy-*` |
| _(untagged)_    | `legacy-*` (today). Drops the `@legacy-only` tag and stays untagged once the spec is dual-mode. |

`INVENTARIO_FRONTEND={legacy|new}` env var (read by `docker-compose.e2e.yaml` or directly by the Go binary) decides which embedded bundle the server hosts. CI brings up the matching stack per matrix arm.

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

## Email delivery tests (Mailpit)

`tests/mailpit-email.spec.ts` (issue #1282) asserts the full transactional
email pipeline — verification, password reset, welcome, password-changed —
by fetching delivered mail out of Mailpit via its HTTP API. The helper lives
at `tests/includes/mailpit.ts` and wraps Mailpit's `/api/v1/messages` and
`/api/v1/message/{id}` endpoints.

**Mailpit is not started by this suite.** It piggybacks on whatever stack is
already up. The tests probe `MAILPIT_URL` (default `http://localhost:8025`)
once in `beforeAll`; if it's unreachable, all tests in the spec `test.skip()`
cleanly.

Where Mailpit comes from in each run mode:

| Mode                              | Mailpit source                                               | mailpit-email.spec.ts |
| --------------------------------- | ------------------------------------------------------------ | --------------------- |
| CI Linux (`chromium`, `firefox`)  | Transitive dep of `inventario` in `docker-compose.yaml`      | Runs                  |
| CI macOS (`webkit`)               | Not present — binary runs without docker                     | Skips                 |
| Local `docker compose up`         | Same transitive dep (plus host port `8025:8025`)             | Runs                  |
| Local `npm run stack` (dev mode)  | Not started; `go run` backend uses the stub email provider   | Skips                 |

The Linux CI lane doesn't explicitly name Mailpit in the `docker compose up`
command — `inventario`'s `depends_on` list includes `mailpit` and
`mailpit-sidecar` (a busybox sidecar whose healthcheck probes Mailpit's
`/api/v1/info`), so `--wait` blocks until Mailpit is reachable before any
test starts. The host port mapping `8025:8025` is defined in the base
`docker-compose.yaml`, which is how the Playwright runner sees it at
`http://localhost:8025`.

### Running the email tests locally

The fast path is the same compose stack CI uses:

```bash
# From the project root
INVENTARIO_IMAGE=inventario-inventario:latest \
  docker compose -f docker-compose.yaml -f docker-compose.e2e.yaml \
  up -d --wait --no-build inventario

# From e2e/
USE_PREBUILT=true npx playwright test mailpit-email.spec.ts
```

The `docker-compose.e2e.yaml` override disables the auth + global rate
limits; without it, the suite's parallel `POST /register` calls trip
`429 Too Many Requests` after the first few. If you run without it and see
rate-limit failures, bring the stack up again using both compose files.

Override `MAILPIT_URL` to point at a different Mailpit instance if needed:

```bash
MAILPIT_URL=http://mailpit.internal:8025 USE_PREBUILT=true \
  npx playwright test mailpit-email.spec.ts
```
