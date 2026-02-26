/**
 * E2E tests for user registration and email verification (Issue #839).
 *
 * These tests exercise public routes (/register, /verify-email) that do not
 * require an authenticated session. They are intentionally lightweight:
 * full end-to-end email delivery is not tested here because the stub email
 * service only logs to stdout. The tests focus on UI behaviour, form
 * validation, and API integration from the browser's perspective.
 */
import { test, expect, Page } from '@playwright/test';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Navigate to the register page and wait for the form to be ready. */
async function goToRegister(page: Page) {
  await page.goto('/register');
  await page.waitForSelector('input[data-testid="name"]', { timeout: 10000 });
}

/** Fill and submit the registration form. */
async function fillAndSubmitRegister(
  page: Page,
  name: string,
  email: string,
  password: string,
) {
  await page.fill('input[data-testid="name"]', name);
  await page.fill('input[data-testid="email"]', email);
  await page.fill('input[data-testid="password"]', password);
  await page.click('button[data-testid="register-button"]');
}

// ---------------------------------------------------------------------------
// Register page — UI
// ---------------------------------------------------------------------------

test.describe('Register page — UI', () => {
  test('shows the registration form', async ({ page }) => {
    await goToRegister(page);

    await expect(page.locator('h1')).toContainText('Inventario');
    await expect(page.locator('input[data-testid="name"]')).toBeVisible();
    await expect(page.locator('input[data-testid="email"]')).toBeVisible();
    await expect(page.locator('input[data-testid="password"]')).toBeVisible();
    await expect(page.locator('button[data-testid="register-button"]')).toBeDisabled();
  });

  test('enables submit button only when all fields are filled', async ({ page }) => {
    await goToRegister(page);

    const btn = page.locator('button[data-testid="register-button"]');
    await expect(btn).toBeDisabled();

    await page.fill('input[data-testid="name"]', 'Test User');
    await expect(btn).toBeDisabled();

    await page.fill('input[data-testid="email"]', 'test@example.com');
    await expect(btn).toBeDisabled();

    await page.fill('input[data-testid="password"]', 'password123');
    await expect(btn).toBeEnabled();
  });

  test('contains a link back to the login page', async ({ page }) => {
    await goToRegister(page);
    const link = page.locator('a[href="/login"]');
    await expect(link).toBeVisible();
  });
});

// ---------------------------------------------------------------------------
// Register page — happy path
// ---------------------------------------------------------------------------

test.describe('Register page — happy path', () => {
  test('shows success message after registration', async ({ page }) => {
    await goToRegister(page);

    // Use a unique email so retries don't collide with other test runs.
    const email = `e2e-reg-${Date.now()}@example.com`;
    await fillAndSubmitRegister(page, 'E2E User', email, 'Password123!');

    // Server always returns HTTP 200 with a success message (anti-enumeration).
    await expect(page.locator('.success-message')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('.success-message')).toContainText('check your email');
  });
});

// ---------------------------------------------------------------------------
// Register page — validation
// ---------------------------------------------------------------------------

test.describe('Register page — validation errors', () => {
  test('shows error when password is too short / weak', async ({ page }) => {
    await goToRegister(page);
    await fillAndSubmitRegister(page, 'Test User', `weak-${Date.now()}@example.com`, '123');

    await expect(page.locator('.error-message')).toBeVisible({ timeout: 10000 });
  });
});

// ---------------------------------------------------------------------------
// Verify-email page
// ---------------------------------------------------------------------------

test.describe('Verify email page', () => {
  test('shows missing-token state when no token is provided', async ({ page }) => {
    await page.goto('/verify-email');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('.status-message.missing')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('.status-message.missing')).toContainText('No verification token');
  });

  test('shows error for an invalid token', async ({ page }) => {
    await page.goto('/verify-email?token=totally-invalid-token');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('.status-message.error')).toBeVisible({ timeout: 10000 });
  });

  test('contains a link to the login page', async ({ page }) => {
    await page.goto('/verify-email');
    await page.waitForLoadState('networkidle');

    await expect(page.locator('a[href="/login"]')).toBeVisible({ timeout: 10000 });
  });
});

// ---------------------------------------------------------------------------
// Registration modes
//
// The server mode is a CLI flag so we cannot change it at runtime. Instead,
// we mock the POST /api/v1/register response with page.route() to exercise
// the frontend's handling of each mode's API response.
// ---------------------------------------------------------------------------

test.describe('Registration mode — open', () => {
  test('shows "check your email" success message', async ({ page }) => {
    await page.route('**/api/v1/register', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'Registration successful. Please check your email to verify your account.' }),
      }),
    );

    await goToRegister(page);
    await fillAndSubmitRegister(page, 'Open User', `open-${Date.now()}@example.com`, 'Password123!');

    await expect(page.locator('.success-message')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('.success-message')).toContainText('check your email');
  });
});

test.describe('Registration mode — approval', () => {
  test('shows "pending administrator approval" success message', async ({ page }) => {
    await page.route('**/api/v1/register', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'Registration successful. Your account is pending administrator approval.' }),
      }),
    );

    await goToRegister(page);
    await fillAndSubmitRegister(page, 'Approval User', `approval-${Date.now()}@example.com`, 'Password123!');

    await expect(page.locator('.success-message')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('.success-message')).toContainText('pending administrator approval');
  });
});

test.describe('Registration mode — closed', () => {
  test('shows an error message when registration is disabled', async ({ page }) => {
    await page.route('**/api/v1/register', (route) =>
      route.fulfill({
        status: 403,
        contentType: 'text/plain',
        body: 'Registrations are currently closed',
      }),
    );

    await goToRegister(page);
    await fillAndSubmitRegister(page, 'Closed User', `closed-${Date.now()}@example.com`, 'Password123!');

    await expect(page.locator('.error-message')).toBeVisible({ timeout: 10000 });
    // The form should remain visible (not switch to the success state).
    await expect(page.locator('form.register-form-content')).toBeVisible();
  });
});

// ---------------------------------------------------------------------------
// Login page — register link
// ---------------------------------------------------------------------------

test.describe('Login page — register link', () => {
  test('login form has a link to the register page', async ({ page }) => {
    await page.goto('/login');
    await page.waitForSelector('input[type="email"]', { timeout: 10000 });

    const link = page.locator('a[href="/register"]');
    await expect(link).toBeVisible();
  });

  test('clicking the register link navigates to /register', async ({ page }) => {
    await page.goto('/login');
    await page.waitForSelector('input[type="email"]', { timeout: 10000 });

    await page.locator('a[href="/register"]').click();
    await expect(page).toHaveURL(/\/register/, { timeout: 10000 });
  });
});

