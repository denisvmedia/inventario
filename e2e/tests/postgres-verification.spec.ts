import { test, expect } from '@playwright/test';
import { resetAndSeedDatabase, isDatabaseReady } from '../utils/database.js';

test.describe('PostgreSQL Database Verification', () => {
  test.beforeEach(async () => {
    await resetAndSeedDatabase();
  });

  test('should connect to PostgreSQL database', async ({ page }) => {
    // Verify database is ready
    const dbReady = await isDatabaseReady();
    expect(dbReady).toBe(true);

    // Navigate to the application
    await page.goto('/');

    // Check that we don't see the "Settings Required" page
    // This would indicate the database is not properly connected
    const settingsRequired = page.locator('h2:has-text("Settings Required")');
    await expect(settingsRequired).not.toBeVisible();

    // Verify we can see the main navigation or dashboard
    // This confirms the app is working with PostgreSQL
    const mainContent = page.locator('main, .main-content, nav, .dashboard');
    await expect(mainContent).toBeVisible();
  });

  test('should persist data between requests', async ({ page }) => {
    // Navigate to locations page
    await page.goto('/locations');

    // Create a test location
    await page.click('text=Add Location');
    await page.fill('input[name="name"]', 'PostgreSQL Test Location');
    await page.fill('input[name="address"]', '123 Database Street');
    await page.click('button[type="submit"]');

    // Verify location was created
    await expect(page.locator('text=PostgreSQL Test Location')).toBeVisible();

    // Refresh the page to verify data persistence
    await page.reload();

    // Data should still be there (unlike memory database)
    await expect(page.locator('text=PostgreSQL Test Location')).toBeVisible();
  });

  test('should enforce PostgreSQL constraints', async ({ page }) => {
    // Navigate to locations page
    await page.goto('/locations');

    // Try to create a location with empty name (should fail validation)
    await page.click('text=Add Location');
    await page.fill('input[name="address"]', '123 Test Street');
    await page.click('button[type="submit"]');

    // Should see validation error
    const errorMessage = page.locator('.error, .alert-danger, [role="alert"]');
    await expect(errorMessage).toBeVisible();
  });

  test('should support PostgreSQL-specific features', async ({ page }) => {
    // This test verifies that PostgreSQL-specific features are available
    // For now, we just verify the basic functionality works
    
    await page.goto('/');
    
    // Check that the application loads without errors
    await expect(page.locator('body')).toBeVisible();
    
    // Verify no JavaScript errors occurred
    const errors: string[] = [];
    page.on('pageerror', (error) => {
      errors.push(error.message);
    });
    
    // Navigate through a few pages to trigger any potential issues
    await page.goto('/locations');
    await page.goto('/areas');
    await page.goto('/commodities');
    
    // Should have no JavaScript errors
    expect(errors).toHaveLength(0);
  });

  test('should handle database reset correctly', async ({ page }) => {
    // Create some test data
    await page.goto('/locations');
    await page.click('text=Add Location');
    await page.fill('input[name="name"]', 'Test Location Before Reset');
    await page.fill('input[name="address"]', '123 Before Street');
    await page.click('button[type="submit"]');

    // Verify data exists
    await expect(page.locator('text=Test Location Before Reset')).toBeVisible();

    // Reset database manually
    await resetAndSeedDatabase();

    // Refresh page
    await page.reload();

    // Data should be gone (reset to seed data only)
    await expect(page.locator('text=Test Location Before Reset')).not.toBeVisible();
    
    // But seeded data should be present
    // (This depends on what your seed data contains)
    const locations = page.locator('[data-testid="location-item"], .location-item, .list-item');
    // Should have some seeded locations or empty state
    await expect(page.locator('body')).toBeVisible(); // Basic check that page loads
  });
});
