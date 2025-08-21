import { Page, expect } from '@playwright/test';

/**
 * Test credentials for e2e tests
 * These match the seeded user in the database
 */
export const TEST_CREDENTIALS = {
  email: 'admin@test-org.com',
  password: 'testpassword123'
};

/**
 * Check if the current page is the login page
 */
export async function isLoginPage(page: Page): Promise<boolean> {
  try {
    // Check URL first
    const currentUrl = page.url();
    if (currentUrl.includes('/login')) {
      return true;
    }

    // Check for login form elements
    const loginForm = page.locator('form').filter({ hasText: 'Login' });
    const emailField = page.locator('input[type="email"]');
    const passwordField = page.locator('input[type="password"]');

    const hasLoginForm = await loginForm.isVisible();
    const hasEmailField = await emailField.isVisible();
    const hasPasswordField = await passwordField.isVisible();

    return hasLoginForm && hasEmailField && hasPasswordField;
  } catch (error) {
    return false;
  }
}

/**
 * Check if the user is currently authenticated
 * This checks for the presence of authenticated UI elements and content
 */
export async function isAuthenticated(page: Page): Promise<boolean> {
  try {
    // Wait for page to load
    await page.waitForLoadState('networkidle', { timeout: 5000 });

    // Check for authenticated content - if we see "Welcome to Inventario" we're authenticated
    const hasWelcomeMessage = await page.locator('h1:has-text("Welcome to Inventario")').isVisible({ timeout: 2000 });
    if (hasWelcomeMessage) {
      return true;
    }

    // Check for authenticated navigation elements
    const nav = page.locator('nav');
    const hasNav = await nav.isVisible();

    if (!hasNav) {
      return false;
    }

    // Check for typical authenticated navigation items with timeout
    const hasLocations = await nav.locator('text=Locations').isVisible({ timeout: 2000 });
    const hasCommodities = await nav.locator('text=Commodities').isVisible({ timeout: 2000 });

    // Check that we're not seeing login-specific elements
    const hasLoginForm = await page.locator('form').filter({ hasText: 'Login' }).isVisible({ timeout: 1000 });

    // Also check for authenticated page content (not just navigation)
    const hasAuthenticatedContent = await page.locator('h1').isVisible({ timeout: 2000 });

    return hasLocations && hasCommodities && !hasLoginForm && hasAuthenticatedContent;
  } catch (error) {
    console.warn('Authentication check error:', error);
    return false;
  }
}

/**
 * Perform login with test credentials
 */
export async function login(page: Page): Promise<void> {
  console.log('🔐 Performing login with test credentials...');
  
  // Wait for login form to be visible
  await page.waitForSelector('input[type="email"]', { timeout: 10000 });
  
  // Fill in credentials
  await page.fill('input[type="email"]', TEST_CREDENTIALS.email);
  await page.fill('input[type="password"]', TEST_CREDENTIALS.password);
  
  // Submit the form
  await page.click('button[type="submit"]');
  
  // Wait for login to complete and redirect
  await page.waitForFunction(
    () => {
      // Check if we're no longer on login page (URL changed) or if we see authenticated content
      return window.location.pathname !== '/login' ||
             (document.querySelector('h1')?.textContent?.includes('Welcome to Inventario') === true);
    },
    { timeout: 10000 }
  );

  // If we're still on login page but see authenticated content, manually navigate to home
  const currentUrl = page.url();
  if (currentUrl.includes('/login') && await page.locator('h1:has-text("Welcome to Inventario")').isVisible()) {
    console.log('🔄 Login successful but still on login URL, navigating to home...');
    await page.goto('/');
  }
  
  console.log('✅ Login completed successfully');
}

/**
 * Ensure the user is authenticated, login if necessary
 */
export async function ensureAuthenticated(page: Page): Promise<void> {
  // First check if we're already authenticated
  if (await isAuthenticated(page)) {
    console.log('✅ Already authenticated');
    return;
  }
  
  // Check if we're on the login page
  if (await isLoginPage(page)) {
    console.log('🔐 On login page, performing login...');
    await login(page);
    return;
  }
  
  // If we're neither authenticated nor on login page, navigate to login
  console.log('🔄 Navigating to login page...');
  await page.goto('/login');
  await login(page);
}

/**
 * Login if needed before accessing a protected page
 * This function handles the common case where navigating to a protected page
 * redirects to login, and we need to authenticate first
 */
export async function loginIfNeeded(page: Page, targetUrl?: string): Promise<void> {
  // If we have a target URL, try to navigate there first
  if (targetUrl) {
    await page.goto(targetUrl);
    
    // Wait a moment for any redirects to complete
    await page.waitForTimeout(1000);
  }
  
  // Check if we ended up on the login page (due to redirect)
  if (await isLoginPage(page)) {
    console.log('🔄 Redirected to login, authenticating...');
    await login(page);
    
    // After login, navigate to target URL if specified
    if (targetUrl && targetUrl !== '/') {
      console.log(`🔄 Navigating to target URL: ${targetUrl}`);
      await page.goto(targetUrl);
    }
  }
  
  // Verify we're now authenticated with retries
  let authVerified = false;
  for (let i = 0; i < 3; i++) {
    await page.waitForTimeout(1000); // Wait a moment for page to settle
    authVerified = await isAuthenticated(page);
    if (authVerified) break;
    console.log(`⏳ Authentication verification attempt ${i + 1}/3...`);
  }

  if (!authVerified) {
    console.error('❌ Authentication verification failed');
    console.error('Current URL:', page.url());
    console.error('Page title:', await page.title());
    throw new Error('Failed to authenticate - login did not complete successfully');
  }
}

/**
 * Logout the current user
 */
export async function logout(page: Page): Promise<void> {
  try {
    // Look for logout button or user menu
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
    
    console.log('✅ Logout completed successfully');
  } catch (error) {
    console.warn('⚠️ Logout failed or not needed:', error);
  }
}

/**
 * Navigate to a URL with authentication handling
 * This is a replacement for page.goto() that handles authentication
 */
export async function navigateWithAuth(page: Page, url: string): Promise<void> {
  console.log(`🔄 Navigating to ${url} with authentication handling...`);
  
  await loginIfNeeded(page, url);
  
  // Ensure we're on the correct page
  const currentUrl = page.url();
  if (!currentUrl.includes(url.replace('/', ''))) {
    await page.goto(url);
  }
  
  console.log(`✅ Successfully navigated to ${url}`);
}
