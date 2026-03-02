import { Page, expect } from '@playwright/test';
import { TestRecorder, log, warn, error } from '../../utils/test-recorder.js';
import { setCsrfToken } from './csrf.js';

/**
 * Test credentials for e2e tests
 * These match the test users created when seeding without parameters (e2e test environment)
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
 * Uses [data-testid="user-menu"] visibility as the primary indicator because
 * that element is only rendered via v-if="authStore.isAuthenticated" in App.vue.
 * Nav elements (Locations, Commodities) are always visible regardless of auth
 * state and must NOT be used for this check.
 */
export async function isAuthenticated(page: Page, recorder?: TestRecorder): Promise<boolean> {
  try {
    // Wait for the Vue router to finish any pending navigation / auth
    // initialization before we inspect the DOM.
    await page.waitForLoadState('networkidle', { timeout: 5000 });

    // The user-menu button is rendered only when authStore.isAuthenticated is
    // true (v-if in App.vue), making it a race-condition-free auth indicator.
    const hasUserMenu = await page.locator('[data-testid="user-menu"]').isVisible({ timeout: 3000 });
    return hasUserMenu;
  } catch (err) {
    warn(recorder, 'Authentication check error:', err);
    return false;
  }
}

/**
 * Perform login with test credentials and extract CSRF token
 */
export async function login(page: Page, recorder?: TestRecorder): Promise<string | null> {
  log(recorder, '🔐 Performing login with test credentials...');

  // Wait for login form to be visible
  await page.waitForSelector('input[type="email"]', { timeout: 10000 });

  // Fill in credentials
  await page.fill('input[type="email"]', TEST_CREDENTIALS.email);
  await page.fill('input[type="password"]', TEST_CREDENTIALS.password);
  // Wait for login API response and fail fast on non-200 statuses.
  const loginResponsePromise = page.waitForResponse(
    (response) => response.url().includes('/api/v1/auth/login'),
    { timeout: 20000 }
  );

  // Submit the form
  await page.click('button[type="submit"]');
  const loginResponse = await loginResponsePromise;
  if (loginResponse.status() !== 200) {
    let responseText = '';
    try {
      responseText = await loginResponse.text();
    } catch {
      // no-op
    }
    error(
      recorder,
      `❌ Login failed: status=${loginResponse.status()} body=${responseText.slice(0, 500)}`
    );
    throw new Error(`Login failed with status ${loginResponse.status()}`);
  }

  // Extract CSRF token directly from successful login response.
  let csrfToken: string | null = null;
  try {
    const data = await loginResponse.json();
    if (data?.csrf_token) {
      csrfToken = data.csrf_token;
      setCsrfToken(csrfToken);
      log(recorder, '🔑 CSRF token extracted from login response');
    }
  } catch {
    warn(recorder, '⚠️ Login succeeded but CSRF token parsing failed');
  }

  // Wait for login to complete and redirect
  await page.waitForFunction(
    () => {
      // Check if we're no longer on login page (URL changed) or if we see authenticated content.
      return !window.location.pathname.startsWith('/login') ||
             (document.querySelector('h1')?.textContent?.includes('Welcome to Inventario') === true);
    },
    { timeout: 30000 }
  );

  // Give UI a brief moment to settle after redirect/auth state propagation.
  await page.waitForTimeout(500);

  // If we're still on login page but see authenticated content, manually navigate to home
  const currentUrl = page.url();
  if (currentUrl.includes('/login') && await page.locator('h1:has-text("Welcome to Inventario")').isVisible()) {
    log(recorder, '🔄 Login successful but still on login URL, navigating to home...');
    await page.goto('/');
  }

  log(recorder, '✅ Login completed successfully');

  return csrfToken;
}

/**
 * Ensure the user is authenticated, login if necessary
 * Returns the CSRF token if login was performed
 */
export async function ensureAuthenticated(page: Page, recorder?: TestRecorder): Promise<string | null> {
  // Wait for any ongoing authentication initialization
  await page.waitForTimeout(500);

  // First check if we're already authenticated
  if (await isAuthenticated(page, recorder)) {
    log(recorder, '✅ Already authenticated');
    return null;
  }

  // Check if we're on the login page
  if (await isLoginPage(page)) {
    log(recorder, '🔐 On login page, performing login...');
    return await login(page, recorder);
  }

  // If we're neither authenticated nor on login page, navigate to login
  log(recorder, '🔄 Navigating to login page...');
  await page.goto('/login');
  return await login(page, recorder);
}

/**
 * Login if needed before accessing a protected page
 * This function handles the common case where navigating to a protected page
 * redirects to login, and we need to authenticate first
 * Returns the CSRF token if login was performed
 */
export async function loginIfNeeded(page: Page, targetUrl?: string, recorder?: TestRecorder): Promise<string | null> {
  let csrfToken: string | null = null;

  // If we have a target URL, try to navigate there first
  if (targetUrl) {
    await page.goto(targetUrl);

    // Wait a moment for any redirects to complete
    await page.waitForTimeout(1000);
  }

  // Check if we ended up on the login page (due to redirect)
  if (await isLoginPage(page)) {
    log(recorder, '🔄 Redirected to login, authenticating...');
    csrfToken = await login(page, recorder);

    // After login, navigate to target URL if specified
    if (targetUrl && targetUrl !== '/') {
      log(recorder, `🔄 Navigating to target URL: ${targetUrl}`);
      await page.goto(targetUrl);
    }
  }

  // Verify we're now authenticated with retries
  let authVerified = false;
  for (let i = 0; i < 3; i++) {
    await page.waitForTimeout(1000); // Wait a moment for page to settle
    authVerified = await isAuthenticated(page, recorder);
    if (authVerified) break;
    log(recorder, `⏳ Authentication verification attempt ${i + 1}/3...`);
  }

  if (!authVerified) {
    error(recorder, '❌ Authentication verification failed');
    error(recorder, `Current URL: ${page.url()}`);
    error(recorder, `Page title: ${await page.title()}`);
    throw new Error('Failed to authenticate - login did not complete successfully');
  }

  return csrfToken;
}

/**
 * Logout the current user
 */
export async function logout(page: Page, recorder?: TestRecorder): Promise<void> {
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

    log(recorder, '✅ Logout completed successfully');
  } catch (err) {
    warn(recorder, '⚠️ Logout failed or not needed:', err);
  }
}

/**
 * Navigate to a URL with authentication handling
 * This is a replacement for page.goto() that handles authentication
 * Returns the CSRF token if login was performed
 */
export async function navigateWithAuth(page: Page, url: string, recorder?: TestRecorder): Promise<string | null> {
  log(recorder, `🔄 Navigating to ${url} with authentication handling...`);

  const csrfToken = await loginIfNeeded(page, url, recorder);

  // Ensure we're on the correct page
  const currentUrl = page.url();
  if (!currentUrl.includes(url.replace('/', ''))) {
    await page.goto(url);
  }

  log(recorder, `✅ Successfully navigated to ${url}`);

  return csrfToken;
}

/**
 * Get the CSRF token from the page context
 * This retrieves the token stored in the frontend's memory
 */
export async function getCsrfToken(page: Page): Promise<string | null> {
  try {
    const token = await page.evaluate(() => {
      // Access the CSRF token from the frontend's api.ts module
      // This assumes the token is stored in a global or accessible location
      return (window as any).__csrfToken || null;
    });
    return token;
  } catch (err) {
    return null;
  }
}
