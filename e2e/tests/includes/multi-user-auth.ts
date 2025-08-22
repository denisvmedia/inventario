import { Page, expect, BrowserContext } from '@playwright/test';

/**
 * Multi-user authentication helpers for user isolation testing
 */

export interface TestUser {
  email: string;
  password: string;
  name: string;
  context?: BrowserContext;
  page?: Page;
}

/**
 * Creates a new user account through the registration process
 */
export async function createUser(page: Page, email: string, password: string, name: string): Promise<void> {
  await page.goto('/register');
  
  // Check if page contains 'Settings Required' and fail fast if found
  const settingsRequired = page.locator('h2:has-text("Settings Required")');
  if (await settingsRequired.isVisible()) {
    throw new Error('Settings Required page found - test environment not properly configured');
  }
  
  // Fill registration form
  await page.fill('[data-testid="email"]', email);
  await page.fill('[data-testid="password"]', password);
  await page.fill('[data-testid="confirm-password"]', password);
  await page.fill('[data-testid="name"]', name);
  
  // Submit registration
  await page.click('[data-testid="register-button"]');
  
  // Wait for successful registration
  await expect(page.locator('[data-testid="success-message"]')).toBeVisible();
}

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
  
  // Fill login form
  await page.fill('[data-testid="email"]', email);
  await page.fill('[data-testid="password"]', password);
  
  // Submit login
  await page.click('[data-testid="login-button"]');
  
  // Wait for successful login - should redirect to dashboard or home
  await page.waitForFunction(
    () => {
      return window.location.pathname !== '/login' ||
             (document.querySelector('h1')?.textContent?.includes('Welcome') === true);
    },
    { timeout: 10000 }
  );
}

/**
 * Logs out the current user
 */
export async function logoutUser(page: Page): Promise<void> {
  try {
    // Look for user menu or logout button
    const userMenu = page.locator('[data-testid="user-menu"]');
    
    if (await userMenu.isVisible()) {
      await userMenu.click();
      
      // Click logout option
      const logoutButton = page.locator('[data-testid="logout"]');
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
 * Creates multiple test users for isolation testing
 */
export async function createTestUsers(page: Page, testName: string, count: number = 2): Promise<TestUser[]> {
  const users: TestUser[] = [];
  const timestamp = Date.now();
  
  for (let i = 1; i <= count; i++) {
    const user: TestUser = {
      email: `user${i}-${testName}-${timestamp}@test.com`,
      password: 'password123',
      name: `Test User ${i} for ${testName}`
    };
    
    // Create the user account
    await createUser(page, user.email, user.password, user.name);
    
    users.push(user);
  }
  
  return users;
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
  // Check for authenticated content
  const hasWelcomeMessage = await page.locator('h1:has-text("Welcome")').isVisible({ timeout: 5000 });
  const hasNavigation = await page.locator('nav').isVisible({ timeout: 5000 });
  
  if (!hasWelcomeMessage && !hasNavigation) {
    throw new Error('User does not appear to be logged in');
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
 */
export async function createCommodityAsUser(user: TestUser, commodityName: string, description?: string): Promise<string> {
  if (!user.page) {
    throw new Error('User page not available');
  }
  
  await user.page.goto('/commodities/create');
  await user.page.fill('[data-testid="commodity-name"]', commodityName);
  
  if (description) {
    await user.page.fill('[data-testid="commodity-description"]', description);
  }
  
  await user.page.click('[data-testid="save-button"]');
  
  // Wait for creation and get the ID from URL
  await user.page.waitForURL(/\/commodities\/[a-zA-Z0-9-]+$/);
  const commodityUrl = user.page.url();
  return commodityUrl.split('/').pop() || '';
}

/**
 * Creates a location as a specific user
 */
export async function createLocationAsUser(user: TestUser, locationName: string, address?: string): Promise<string> {
  if (!user.page) {
    throw new Error('User page not available');
  }
  
  await user.page.goto('/locations/create');
  await user.page.fill('[data-testid="location-name"]', locationName);
  
  if (address) {
    await user.page.fill('[data-testid="location-address"]', address);
  }
  
  await user.page.click('[data-testid="save-button"]');
  
  // Wait for creation and get the ID from URL
  await user.page.waitForURL(/\/locations\/[a-zA-Z0-9-]+$/);
  const locationUrl = user.page.url();
  return locationUrl.split('/').pop() || '';
}

/**
 * Verifies that a user cannot see specific content
 */
export async function verifyUserCannotSeeContent(user: TestUser, contentText: string): Promise<void> {
  if (!user.page) {
    throw new Error('User page not available');
  }
  
  await expect(user.page.locator(`text=${contentText}`)).not.toBeVisible();
}

/**
 * Verifies that a user can see specific content
 */
export async function verifyUserCanSeeContent(user: TestUser, contentText: string): Promise<void> {
  if (!user.page) {
    throw new Error('User page not available');
  }
  
  await expect(user.page.locator(`text=${contentText}`)).toBeVisible();
}

/**
 * Attempts to access a URL and verifies the response
 */
export async function attemptDirectAccess(user: TestUser, url: string, shouldSucceed: boolean = false): Promise<void> {
  if (!user.page) {
    throw new Error('User page not available');
  }
  
  await user.page.goto(url);
  
  if (shouldSucceed) {
    // Should not see "Not Found" or error messages
    await expect(user.page.locator('text=Not Found')).not.toBeVisible();
    await expect(user.page.locator('text=Error')).not.toBeVisible();
  } else {
    // Should see "Not Found" or be redirected
    const hasNotFound = await user.page.locator('text=Not Found').isVisible({ timeout: 5000 });
    const isRedirected = !user.page.url().includes(url.split('/').pop() || '');
    
    if (!hasNotFound && !isRedirected) {
      throw new Error(`User was able to access ${url} when they should not have been able to`);
    }
  }
}

/**
 * Verifies that search results are properly isolated
 */
export async function verifySearchIsolation(user: TestUser, searchTerm: string, shouldFind: boolean = false): Promise<void> {
  if (!user.page) {
    throw new Error('User page not available');
  }
  
  await user.page.goto('/search');
  await user.page.fill('[data-testid="search-input"]', searchTerm);
  await user.page.click('[data-testid="search-button"]');
  
  if (shouldFind) {
    await expect(user.page.locator(`text=${searchTerm}`)).toBeVisible();
  } else {
    await expect(user.page.locator('text=No results found')).toBeVisible();
    await expect(user.page.locator(`text=${searchTerm}`)).not.toBeVisible();
  }
}
