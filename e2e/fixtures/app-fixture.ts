import { test as base } from '@playwright/test';

/**
 * Custom fixture that ensures the application stack is running
 */
export const test = base.extend({
  // Setup the application stack before tests
  page: async ({ page }, use) => {
    // The stack should already be running via the e2e:stack command
    // We just need to navigate to the base URL
    await page.goto('/');

    // Use the page with the application loaded
    await use(page);
  },
});
