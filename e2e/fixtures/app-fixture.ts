import { test as base, expect } from '@playwright/test';
import { TestRecorder } from '../utils/test-recorder.js';
import waitOn from 'wait-on';

// Define the type for our custom fixtures
type AppFixtures = {
  recorder: TestRecorder;
};

/**
 * Check if the page shows "Settings Required" message and fail fast if found
 */
async function checkSettingsRequired(page: any) {
  const settingsRequiredElement = page.locator('h2:has-text("Settings Required")');
  const isVisible = await settingsRequiredElement.isVisible();

  if (isVisible) {
    throw new Error('Test failed: "Settings Required" message found. The system database is not properly seeded/set up.');
  }
}

/**
 * Custom fixture that ensures the application stack is running
 */
export const test = base.extend<AppFixtures>({
  // Setup the application stack before tests
  page: async ({ page }, use) => {
    await waitOn({
      resources: ['http://localhost:5173'],
      timeout: 15000,
      interval: 250,
      window: 1000,
      tcpTimeout: 1000,
    });

    // The stack should already be running via the e2e:stack command
    // We just need to navigate to the base URL
    await page.goto('/');

    // Check for "Settings Required" message and fail fast if found
    await checkSettingsRequired(page);

    // Use the page with the application loaded
    await use(page);
  },

  // Create a TestRecorder for each test
  recorder: async ({ page }, use, testInfo) => {
    // Create a recorder with the test name
    const recorder = new TestRecorder(page, testInfo.title);

    // Use the recorder in the test
    await use(recorder);
  },
});
