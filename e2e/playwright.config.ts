import { defineConfig, devices } from '@playwright/test';
import { BASE_URL } from './setup/urls.js';

/**
 * Playwright config — see https://playwright.dev/docs/test-configuration.
 *
 * Project naming convention is `<frontend>-<browser>` so the user can target
 * a stack + browser combination explicitly:
 *
 *     npx playwright test --project=legacy-chromium  # legacy Vue bundle
 *     npx playwright test --project=new-chromium     # React bundle
 *
 * The project itself does NOT bring up the stack — the `INVENTARIO_FRONTEND`
 * env var (read by docker-compose.e2e.yaml or by the Go binary directly)
 * decides which bundle the server hosts. Run via the `inventario-e2e` skill
 * (see e2e/README.md), which takes care of starting the right stack before
 * invoking Playwright.
 *
 * Spec gating uses Playwright tag-grep:
 *   - `@react-only` — only runs against the React stack (`new-*` projects).
 *     Legacy projects skip via `grepInvert`.
 *   - `@legacy-only` — only runs against the legacy stack (`legacy-*`
 *     projects). New projects skip via `grepInvert`. Most of the existing
 *     22 specs are tagged this way during the migration window; each
 *     per-page React PR drops the tag once the corresponding feature ships
 *     on the new bundle.
 *   - untagged — runs everywhere. Stack-agnostic specs live here once
 *     their selectors are dual-mode.
 */

const browsers = [
  { id: 'chromium', use: devices['Desktop Chrome'] },
  { id: 'firefox', use: devices['Desktop Firefox'] },
  { id: 'webkit', use: devices['Desktop Safari'] },
];

interface FrontendVariant {
  id: 'legacy' | 'new';
  // Specs to include. Combined with grepInvert via Playwright's
  // intersection semantics — both must be satisfied.
  grep?: RegExp;
  grepInvert?: RegExp;
}

const frontends: FrontendVariant[] = [
  {
    id: 'legacy',
    // Legacy bundle is the production default; everything except `@react-only`
    // runs here.
    grepInvert: /@react-only/,
  },
  {
    id: 'new',
    // React bundle today only has the shell + placeholder pages, so we
    // run *only* specs that opted into `@react-only`. As feature pages
    // port (#1407–#1417), the corresponding spec drops `@legacy-only`
    // and may pick up `@react-only` (or stay untagged for stack-agnostic
    // coverage).
    grep: /@react-only|@react-ready/,
    grepInvert: /@legacy-only/,
  },
];

const projects = frontends.flatMap((frontend) =>
  browsers.map((browser) => ({
    name: `${frontend.id}-${browser.id}`,
    use: { ...browser.use },
    metadata: { frontend: frontend.id },
    ...(frontend.grep ? { grep: frontend.grep } : {}),
    ...(frontend.grepInvert ? { grepInvert: frontend.grepInvert } : {}),
  }))
);

export default defineConfig({
  testDir: './tests',
  // timeout: 60 * 1000,
  timeout: 120 * 1000,
  expect: {
    /**
     * Maximum time expect() should wait for the condition to be met.
     * For example in `await expect(locator).toHaveText();`
     */
    // timeout: 10000
    timeout: 10000,
  },
  /* Run tests in files in parallel */
  fullyParallel: true,
  /* Fail the build on CI if you accidentally left test.only in the source code. */
  forbidOnly: !!process.env.CI,
  /* Retry on CI only */
  retries: process.env.CI ? 2 : 0,
  /* Opt out of parallel tests on CI. */
  workers: process.env.CI ? 1 : undefined,
  /* Reporter to use. See https://playwright.dev/docs/test-reporters */
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'html',
  /* Output directory for test artifacts */
  outputDir: './test-results/',
  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    /* Base URL to use in actions like `await page.goto('/')`. */
    baseURL: BASE_URL,

    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: 'on-first-retry',

    /* Take screenshot for all tests */
    screenshot: 'on',

    /* Record video for all tests */
    video: 'on',
  },

  projects,

  /* Run your local dev server before starting the tests */
  // webServer: {
  //   // We'll handle this separately in our setup scripts
  //   port: 5173,
  //   reuseExistingServer: true,
  //   timeout: 60 * 1000,
  // },

  /* Global setup and teardown */
  globalSetup: './setup/global-setup.ts',
  globalTeardown: './setup/global-teardown.ts',
});
