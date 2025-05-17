import { test as base } from '@playwright/test';
import { startStack, stopStack } from '../setup/setup-stack';

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

/**
 * Start the application stack for all tests
 */
export async function globalSetup() {
  // Only start the stack if it's not already running
  // This is useful for local development where you might want to start the stack manually
  if (process.env.START_STACK === 'true') {
    await startStack();
  }
}

/**
 * Stop the application stack after all tests
 */
export async function globalTeardown() {
  // Only stop the stack if we started it
  if (process.env.START_STACK === 'true') {
    await stopStack();
  }
}
