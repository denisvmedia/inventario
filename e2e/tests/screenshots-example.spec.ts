import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';
import path from 'path';

test.describe('Screenshots and Videos Example', () => {
  test('should take manual screenshots during test execution', async ({ page }) => {
    // Navigate to home page
    await page.goto('/');
    
    // Take a screenshot of the home page
    await page.screenshot({ 
      path: path.join('test-results', 'screenshots', 'home-page.png'),
      fullPage: true 
    });
    
    // Click on the Locations card
    await page.locator('.navigation-cards .card', { hasText: 'Locations' }).click();
    
    // Verify we're on the locations page
    await expect(page).toHaveURL(/\/locations/);
    
    // Take a screenshot of the locations page
    await page.screenshot({ 
      path: path.join('test-results', 'screenshots', 'locations-page.png'),
      fullPage: true 
    });
    
    // Click on the Commodities link in the navigation
    await page.click('nav >> text=Commodities');
    
    // Verify we're on the commodities page
    await expect(page).toHaveURL(/\/commodities/);
    
    // Take a screenshot of the commodities page
    await page.screenshot({ 
      path: path.join('test-results', 'screenshots', 'commodities-page.png'),
      fullPage: true 
    });
    
    // Take a screenshot of a specific element
    const header = page.locator('h1');
    await expect(header).toBeVisible();
    await header.screenshot({ 
      path: path.join('test-results', 'screenshots', 'commodities-header.png') 
    });
  });
  
  test('should take screenshots with timestamps', async ({ page }) => {
    // Function to generate timestamp for filenames
    const getTimestampFilename = (baseName: string) => {
      const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
      return `${baseName}-${timestamp}.png`;
    };
    
    // Navigate to home page
    await page.goto('/');
    
    // Take a screenshot with timestamp
    await page.screenshot({ 
      path: path.join('test-results', 'screenshots', getTimestampFilename('home')),
      fullPage: true 
    });
    
    // Navigate to settings
    await page.click('nav >> text=Settings');
    await expect(page).toHaveURL(/\/settings/);
    
    // Take another screenshot with timestamp
    await page.screenshot({ 
      path: path.join('test-results', 'screenshots', getTimestampFilename('settings')),
      fullPage: true 
    });
  });
});
