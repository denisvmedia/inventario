import { defineConfig, devices } from '@playwright/test';
import { BASE_URL } from './setup/urls.js';

/**
 * Playwright config — see https://playwright.dev/docs/test-configuration.
 *
 * Project naming convention is the browser name; the React frontend is the
 * single supported stack:
 *
 *     npx playwright test --project=chromium
 *     npx playwright test --project=firefox
 *     npx playwright test --project=webkit
 *
 * Run via the `inventario-e2e` skill (see e2e/README.md), which takes care
 * of starting the stack before invoking Playwright.
 */

const projects = [
  { name: 'chromium', use: devices['Desktop Chrome'] },
  { name: 'firefox', use: devices['Desktop Firefox'] },
  { name: 'webkit', use: devices['Desktop Safari'] },
];

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
