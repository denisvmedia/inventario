import { test, expect } from '@playwright/test';

test.describe('Minimal PostgreSQL Test', () => {
  test('should be able to access the application', async ({ page }) => {
    console.log('Starting minimal PostgreSQL test...');
    
    // Just try to access the home page
    await page.goto('/');
    
    // Check that the page loads (any content is fine)
    await expect(page.locator('body')).toBeVisible();
    
    console.log('Minimal test completed successfully');
  });
});
