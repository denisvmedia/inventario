import { Page, expect, BrowserContext } from '@playwright/test';

/**
 * Test user interface for user isolation testing
 */
export interface TestUser {
  email: string;
  password: string;
  name: string;
  context?: BrowserContext;
  page?: Page;
}

/**
 * Pre-seeded test users for isolation testing
 */
export const SEEDED_TEST_USERS: TestUser[] = [
  {
    email: 'admin@test-org.com',
    password: 'testpassword123',
    name: 'Test Administrator'
  },
  {
    email: 'user2@test-org.com', 
    password: 'testpassword123',
    name: 'Test User 2'
  }
];

/**
 * Logs in a user with email and password
 */
export async function loginUser(page: Page, email: string, password: string): Promise<void> {
  await page.goto('/login');

  // Check if page contains 'Settings Required' and fail fast if found
  const settingsRequired = page.locator('h2:has-text("Settings Required")');
  if (await settingsRequired.isVisible()) {
    throw new Error('Settings Required page found - test environment not properly configured');
  }

  // Wait for login form to be visible
  await page.waitForSelector('input[type="email"]', { timeout: 10000 });

  // Fill in credentials using the same selectors as working auth.ts
  await page.fill('input[type="email"]', email);
  await page.fill('input[type="password"]', password);

  // Submit the form
  await page.click('button[type="submit"]');

  // Wait for login to complete and redirect (same logic as auth.ts)
  await page.waitForFunction(
    () => {
      return window.location.pathname !== '/login' ||
             (document.querySelector('h1')?.textContent?.includes('Welcome to Inventario') === true);
    },
    { timeout: 10000 }
  );
}

/**
 * Logs out the current user
 */
export async function logoutUser(page: Page): Promise<void> {
  try {
    // Use the same logout logic as working auth.ts
    const userMenu = page.locator('[data-testid="user-menu"]').or(page.locator('button:has-text("Logout")'));

    if (await userMenu.isVisible()) {
      await userMenu.click();

      // If it's a dropdown menu, look for logout option
      const logoutButton = page.locator('button:has-text("Logout")').or(page.locator('a:has-text("Logout")'));
      if (await logoutButton.isVisible()) {
        await logoutButton.click();
      }
    }

    // Wait for logout to complete
    await page.waitForFunction(
      () => window.location.pathname === '/login',
      { timeout: 5000 }
    );
  } catch (error) {
    console.warn('Logout failed or not needed:', error);
  }
}

/**
 * Gets pre-seeded test users for isolation testing
 */
export async function getTestUsers(testName: string, count: number = 2): Promise<TestUser[]> {
  if (count > SEEDED_TEST_USERS.length) {
    throw new Error(`Requested ${count} users but only ${SEEDED_TEST_USERS.length} are available`);
  }
  
  return SEEDED_TEST_USERS.slice(0, count);
}

/**
 * Sets up isolated browser contexts for multiple users
 */
export async function setupUserContexts(browser: any, users: TestUser[]): Promise<TestUser[]> {
  const updatedUsers: TestUser[] = [];
  
  for (const user of users) {
    const context = await browser.newContext();
    const page = await context.newPage();
    
    updatedUsers.push({
      ...user,
      context,
      page
    });
  }
  
  return updatedUsers;
}

/**
 * Logs in all users in their respective contexts
 */
export async function loginAllUsers(users: TestUser[]): Promise<void> {
  for (const user of users) {
    if (user.page) {
      await loginUser(user.page, user.email, user.password);
    }
  }
}

/**
 * Cleans up all user contexts
 */
export async function cleanupUserContexts(users: TestUser[]): Promise<void> {
  for (const user of users) {
    if (user.context) {
      await user.context.close();
    }
  }
}

/**
 * Verifies that a user is logged in by checking for authenticated content
 */
export async function verifyUserLoggedIn(page: Page): Promise<void> {
  // Use the same authentication check logic as working auth.ts
  await page.waitForLoadState('networkidle', { timeout: 5000 });

  // Check for authenticated content - if we see "Welcome to Inventario" we're authenticated
  const hasWelcomeMessage = await page.locator('h1:has-text("Welcome to Inventario")').isVisible({ timeout: 2000 });
  if (hasWelcomeMessage) {
    return;
  }

  // Check for authenticated navigation elements
  const nav = page.locator('nav');
  const hasNav = await nav.isVisible();

  if (!hasNav) {
    throw new Error('User does not appear to be logged in - no navigation found');
  }

  // Check for typical authenticated navigation items
  const hasLocations = await nav.locator('text=Locations').isVisible({ timeout: 2000 });
  const hasCommodities = await nav.locator('text=Commodities').isVisible({ timeout: 2000 });

  // Check that we're not seeing login-specific elements
  const hasLoginForm = await page.locator('form').filter({ hasText: 'Login' }).isVisible({ timeout: 1000 });

  if (!hasLocations || !hasCommodities || hasLoginForm) {
    throw new Error('User does not appear to be logged in - missing authenticated navigation or login form present');
  }
}

/**
 * Verifies that a user is logged out by checking for login form
 */
export async function verifyUserLoggedOut(page: Page): Promise<void> {
  await expect(page.locator('input[type="email"]')).toBeVisible();
  await expect(page.locator('input[type="password"]')).toBeVisible();
}

/**
 * Switches between users by logging out current user and logging in new user
 */
export async function switchUser(page: Page, newUser: TestUser): Promise<void> {
  await logoutUser(page);
  await loginUser(page, newUser.email, newUser.password);
}

/**
 * Creates a commodity as a specific user
 * Returns an object with the unique commodity name and ID
 */
export async function createCommodityAsUser(user: TestUser, commodityName: string, description?: string): Promise<{ name: string; id: string }> {
  if (!user.page) {
    throw new Error('User page not available');
  }

  // Make commodity name unique by adding timestamp
  const uniqueCommodityName = `${commodityName}-${Date.now()}`;

  // Navigate to locations first (commodities are created within areas)
  await user.page.goto('/locations');

  // Create a location first if none exists
  const hasLocations = await user.page.locator('.location-card').count() > 0;
  if (!hasLocations) {
    await user.page.click('button:has-text("New")');
    await user.page.fill('#name', 'Test Location');
    await user.page.click('button:has-text("Create Location")');
    await user.page.waitForSelector('.location-card:has-text("Test Location")');
  }

  // Click on the first location to expand it
  const firstLocation = user.page.locator('.location-card').first();
  await firstLocation.click();

  // Create an area if none exists
  const hasAreas = await user.page.locator('.area-card').count() > 0;
  if (!hasAreas) {
    await user.page.click('button:has-text("Add Area")');
    await user.page.fill('input[placeholder="Area name"]', 'Test Area');
    await user.page.click('button:has-text("Create")');
    await user.page.waitForSelector('.area-card:has-text("Test Area")');
  }

  // Click on the first area to go to commodities (this navigates to the area's commodity page)
  const firstArea = user.page.locator('.area-card').first();
  await firstArea.click();

  // Wait for the area commodities page to load
  await user.page.waitForLoadState('networkidle');

  // Now we should be on the commodities page for this area
  // Wait for either "Add Commodity" link (when no commodities) or "New" button with icon (when commodities exist)
  try {
    await user.page.waitForSelector('a:has-text("Add Commodity"), a:has-text("New"):has(svg)', { timeout: 10000 });
    const createButton = user.page.locator('a:has-text("Add Commodity"), a:has-text("New"):has(svg)').first();
    await createButton.click();
  } catch (error) {
    throw new Error('Neither "Add Commodity" link nor "New" button with icon found on area commodities page');
  }

  // Fill in the commodity form using the same selectors as working tests
  await user.page.fill('#name', uniqueCommodityName);
  await user.page.fill('#shortName', uniqueCommodityName);

  // Select type from dropdown
  await user.page.click('.p-select[id="type"]');
  await user.page.click('.p-select-option-label:has-text("Other")');

  // Fill in required count field
  await user.page.fill('#count', '1');

  // Submit the form
  await user.page.click('button:has-text("Create Commodity")');

  // Wait for creation and get the ID from URL
  await user.page.waitForURL(/\/commodities\/[0-9a-fA-F-]{36}/, { timeout: 10000 });
  const commodityUrl = user.page.url();
  const commodityId = commodityUrl.split('/').pop() || '';

  // Return both the unique commodity name and ID
  return {
    name: uniqueCommodityName,
    id: commodityId
  };
}

/**
 * Creates a location as a specific user
 */
export async function createLocationAsUser(user: TestUser, locationName: string, address?: string): Promise<string> {
  if (!user.page) {
    throw new Error('User page not available');
  }

  // Navigate to locations page first
  await user.page.goto('/locations');

  // Click the New button (same as working locations.ts)
  await user.page.click('button:has-text("New")');

  // Fill in the location form using the same selectors as working tests
  await user.page.fill('#name', locationName);
  if (address) {
    await user.page.fill('#address', address);
  }

  // Submit the form
  await user.page.click('button:has-text("Create Location")');

  // Wait for the location to be created and displayed
  await user.page.waitForSelector(`.location-card:has-text("${locationName}")`, { timeout: 10000 });

  // Find the location card and click the edit button (with edit icon) to get the real ID from the URL
  const locationCard = user.page.locator(`.location-card:has-text("${locationName}")`).first();
  const editButton = locationCard.locator('button[title="Edit"]');
  await editButton.click();

  // Wait for the edit page to load and extract ID from URL
  await user.page.waitForURL(/\/locations\/[0-9a-fA-F-]{36}\/edit/, { timeout: 10000 });
  const editUrl = user.page.url();
  const locationId = editUrl.split('/')[editUrl.split('/').length - 2]; // Get the ID part before '/edit'

  // Navigate back to locations list
  await user.page.goto('/locations');

  return locationId;
}

/**
 * Verifies that a user cannot see specific content
 * Navigates to the area where commodities are displayed to check visibility
 */
export async function verifyUserCannotSeeContent(user: TestUser, contentText: string): Promise<void> {
  if (!user.page) {
    throw new Error('User page not available');
  }

  // Navigate to locations page and try to find the content
  await user.page.goto('/locations');
  await user.page.waitForLoadState('networkidle', { timeout: 5000 });

  // Try to find and click on the first location to expand it
  const firstLocation = user.page.locator('.location-card').first();
  if (await firstLocation.isVisible({ timeout: 2000 })) {
    await firstLocation.click();

    // Try to find and click on the first area to see commodities
    const firstArea = user.page.locator('.area-card').first();
    if (await firstArea.isVisible({ timeout: 2000 })) {
      await firstArea.click();
      await user.page.waitForLoadState('networkidle', { timeout: 5000 });
    }
  }

  // Check that the content is not visible anywhere on the page
  await expect(user.page.locator(`text=${contentText}`)).not.toBeVisible();
}

/**
 * Verifies that a user can see specific content
 * Navigates to the area where commodities are displayed to check visibility
 */
export async function verifyUserCanSeeContent(user: TestUser, contentText: string): Promise<void> {
  if (!user.page) {
    throw new Error('User page not available');
  }

  // Navigate to locations page and try to find the content
  await user.page.goto('/locations');
  await user.page.waitForLoadState('networkidle', { timeout: 5000 });

  // Try to find and click on the first location to expand it
  const firstLocation = user.page.locator('.location-card').first();
  if (await firstLocation.isVisible({ timeout: 2000 })) {
    await firstLocation.click();

    // Try to find and click on the first area to see commodities
    const firstArea = user.page.locator('.area-card').first();
    if (await firstArea.isVisible({ timeout: 2000 })) {
      await firstArea.click();
      await user.page.waitForLoadState('networkidle', { timeout: 5000 });
    }
  }

  await expect(user.page.locator(`text=${contentText}`).first()).toBeVisible();
}

/**
 * Attempts to access a URL and verifies the response
 */
export async function attemptDirectAccess(user: TestUser, url: string, shouldSucceed: boolean = false): Promise<void> {
  if (!user.page) {
    throw new Error('User page not available');
  }

  await user.page.goto(url);
  await user.page.waitForLoadState('networkidle', { timeout: 5000 });

  if (shouldSucceed) {
    await user.page.waitForSelector('.header')
  } else {
    await user.page.waitForSelector('.resource-not-found')
  }
}

/**
 * Verifies that search results are properly isolated
 * This navigates to the area where commodities are displayed to check visibility
 */
export async function verifySearchIsolation(user: TestUser, searchTerm: string, shouldFind: boolean = false): Promise<void> {
  if (!user.page) {
    throw new Error('User page not available');
  }

  // Navigate to locations page
  await user.page.goto('/locations');
  await user.page.waitForLoadState('networkidle', { timeout: 5000 });

  // Try to find and click on the first location to expand it
  const firstLocation = user.page.locator('.location-card').first();
  if (await firstLocation.isVisible({ timeout: 2000 })) {
    await firstLocation.click();

    // Try to find and click on the first area to see commodities
    const firstArea = user.page.locator('.area-card').first();
    if (await firstArea.isVisible({ timeout: 2000 })) {
      await firstArea.click();
      await user.page.waitForLoadState('networkidle', { timeout: 5000 });
    }
  }

  if (shouldFind) {
    // User should be able to see their own content
    await expect(user.page.locator(`text=${searchTerm}`).first()).toBeVisible({ timeout: 5000 });
  } else {
    // User should NOT be able to see other user's content
    const searchResults = user.page.locator(`text=${searchTerm}`);
    await expect(searchResults).not.toBeVisible();
  }
}
