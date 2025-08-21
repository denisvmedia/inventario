import { test, expect } from '@playwright/test';
import { ensureAuthenticated, isAuthenticated, isLoginPage } from './includes/auth.js';

test.describe('Debug Authentication', () => {
  test('should debug authentication flow', async ({ page }) => {
    console.log('🔍 Starting authentication debug...');
    
    // Navigate to home page
    await page.goto('/');
    console.log('📍 Current URL:', page.url());
    
    // Check if we're on login page
    const onLoginPage = await isLoginPage(page);
    console.log('🔐 On login page:', onLoginPage);
    
    // Check if authenticated
    const authenticated = await isAuthenticated(page);
    console.log('✅ Authenticated:', authenticated);
    
    // Take a screenshot
    await page.screenshot({ path: 'debug-before-auth.png', fullPage: true });
    
    // Ensure authentication
    await ensureAuthenticated(page);
    
    // Check status after authentication
    console.log('📍 URL after auth:', page.url());
    const authAfter = await isAuthenticated(page);
    console.log('✅ Authenticated after:', authAfter);
    
    // Take another screenshot
    await page.screenshot({ path: 'debug-after-auth.png', fullPage: true });
    
    // Check page content
    const h1Text = await page.locator('h1').textContent();
    console.log('📝 H1 text:', h1Text);
    
    const pageTitle = await page.title();
    console.log('📄 Page title:', pageTitle);
    
    // Check if navigation is present
    const navVisible = await page.locator('nav').isVisible();
    console.log('🧭 Navigation visible:', navVisible);
    
    if (navVisible) {
      const navText = await page.locator('nav').textContent();
      console.log('🧭 Navigation text:', navText);
    }
    
    // Try to navigate to locations
    console.log('🔄 Attempting to navigate to locations...');
    await page.goto('/locations');
    
    console.log('📍 URL after locations navigation:', page.url());
    await page.screenshot({ path: 'debug-locations.png', fullPage: true });
    
    const locationsH1 = await page.locator('h1').textContent();
    console.log('📝 Locations H1 text:', locationsH1);
  });
});
