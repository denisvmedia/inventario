import { Page } from '@playwright/test';

/**
 * Check if the page shows "Settings Required" message and fail fast if found.
 * This indicates that the system database is not properly seeded/set up.
 * 
 * @param page - The Playwright page object
 * @throws Error if "Settings Required" message is found
 */
export async function checkSettingsRequired(page: Page): Promise<void> {
  const settingsRequiredElement = page.locator('h2:has-text("Settings Required")');
  const isVisible = await settingsRequiredElement.isVisible();
  
  if (isVisible) {
    throw new Error('Test failed: "Settings Required" message found. The system database is not properly seeded/set up.');
  }
}

/**
 * Navigate to a URL and check for "Settings Required" message.
 * This is a convenience function that combines navigation with the settings check.
 * 
 * @param page - The Playwright page object
 * @param url - The URL to navigate to
 * @throws Error if "Settings Required" message is found after navigation
 */
export async function navigateAndCheckSettings(page: Page, url: string): Promise<void> {
  await page.goto(url);
  await checkSettingsRequired(page);
}
