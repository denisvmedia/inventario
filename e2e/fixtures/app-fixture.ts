import { test as base } from '@playwright/test';
import { TestRecorder } from '../utils/test-recorder';

// Define the type for our custom fixtures
type AppFixtures = {
  recorder: TestRecorder;
};

/**
 * Custom fixture that ensures the application stack is running
 */
export const test = base.extend<AppFixtures>({
  // Setup the application stack before tests
  page: async ({ page }, use) => {
    // The stack should already be running via the e2e:stack command
    // We just need to navigate to the base URL
    await page.goto('/');

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
