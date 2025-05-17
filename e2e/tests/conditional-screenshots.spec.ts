import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';
import { TestRecorder } from '../utils/test-recorder.js';
import path from 'path';

test.describe('Conditional Screenshots', () => {
  test('should take screenshots based on application state', async ({ page }) => {
    const recorder = new TestRecorder(page, 'Conditional-Screenshots');

    // Navigate to home page
    await page.goto('/');

    // Check if the value summary is visible
    const valueSummary = page.locator('.value-summary');
    if (await valueSummary.isVisible()) {
      await recorder.takeElementScreenshot('.value-summary', 'value-summary');

      // Check which state the value summary is in
      const hasValue = await page.locator('.value-amount').isVisible();
      const isLoading = await page.locator('.value-loading').isVisible();
      const isEmpty = await page.locator('.value-empty').isVisible();

      if (hasValue) {
        await recorder.takeElementScreenshot('.value-amount', 'value-amount');
      } else if (isLoading) {
        await recorder.takeElementScreenshot('.value-loading', 'value-loading');
      } else if (isEmpty) {
        await recorder.takeElementScreenshot('.value-empty', 'value-empty');
      }
    }

    // Navigate to locations
    await page.locator('.navigation-cards .card', { hasText: 'Locations' }).click();
    await expect(page).toHaveURL(/\/locations/);

    // Take a screenshot of the locations page
    await recorder.takeScreenshot('locations-page');

    // Check if there are any locations
    const locationCards = page.locator('.location-card');
    const locationCount = await locationCards.count();

    if (locationCount > 0) {
      // Take a screenshot of each location card
      for (let i = 0; i < Math.min(locationCount, 3); i++) { // Limit to first 3 cards
        await recorder.takeElementScreenshot('.location-card', `location-card-${i + 1}`, i);
      }
    } else {
      // Take a screenshot of the empty state
      await recorder.takeScreenshot('locations-empty');
    }
  });
});
