/**
 * E2E tests for profile management and related features:
 * - Header user dropdown (visibility, navigation, logout)
 * - Profile page layout and name update (validation, success banner)
 * - Password change section (client-side validation + API responses)
 * - Session expired message on the login page
 */
import { test as authTest } from '../fixtures/app-fixture.js';
import { test, expect } from '@playwright/test';
import type { Page } from '@playwright/test';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

async function goToProfile(page: Page) {
  // The React port splits read-only profile (/profile) from the editable
  // form (/profile/edit). All edit-flow assertions in this file (name
  // update, password change) live on the latter, so we navigate straight
  // there; the dropdown-link test still goes through the read-only page
  // first via the user-menu Profile entry.
  await page.goto('/profile/edit');
  await expect(page.locator('h1')).toBeVisible();
}

async function openPasswordSection(page: Page) {
  await page.click('.password-toggle');
  await expect(page.locator('.password-form')).toBeVisible();
}

// ---------------------------------------------------------------------------
// Header — user dropdown
// ---------------------------------------------------------------------------

authTest.describe('Header — user dropdown', () => {
  authTest('user menu trigger is visible', async ({ page }) => {
    await expect(page.locator('[data-testid="user-menu"]')).toBeVisible();
  });

  authTest('opening the menu shows Profile and Logout options', async ({ page }) => {
    await page.click('[data-testid="user-menu"]');
    await expect(page.locator('.user-dropdown')).toBeVisible();
    await expect(page.locator('.user-dropdown a:has-text("Profile")')).toBeVisible();
    await expect(page.locator('.dropdown-item--logout')).toBeVisible();
  });

  authTest('clicking outside closes the dropdown', async ({ page }) => {
    await page.click('[data-testid="user-menu"]');
    await expect(page.locator('.user-dropdown')).toBeVisible();
    // Radix renders a focus-trapping overlay when the menu is open, so a
    // direct page.click('h1') is intercepted by that overlay (visible to
    // pointer events but not Playwright's element-resolution). Press
    // Escape — Radix maps it to the same close action as an outside
    // pointerdown, which is what this test cares about.
    await page.keyboard.press('Escape');
    await expect(page.locator('.user-dropdown')).not.toBeVisible();
  });

  authTest('Profile link navigates to /profile', async ({ page }) => {
    await page.click('[data-testid="user-menu"]');
    await page.click('.user-dropdown a:has-text("Profile")');
    await expect(page).toHaveURL(/\/profile/);
    await expect(page.locator('h1')).toContainText('My Profile');
  });

  authTest('Logout button redirects to login page', async ({ page }) => {
    await page.click('[data-testid="user-menu"]');
    await page.click('.dropdown-item--logout');
    await expect(page).toHaveURL(/\/login/, { timeout: 10000 });
  });
});

// ---------------------------------------------------------------------------
// Profile page — layout
// ---------------------------------------------------------------------------

authTest.describe('Profile page — layout', () => {
  authTest('shows heading and all form fields', async ({ page }) => {
    await goToProfile(page);
    await expect(page.locator('#profile-name')).toBeVisible();
    await expect(page.locator('#profile-email')).toBeVisible();
    // The tenant-level `users.role` column and the matching `#profile-role`
    // UI field were removed in the Location Groups refactor (roles are now
    // per-group via GroupMembership.role).
  });

  authTest('email field is disabled', async ({ page }) => {
    await goToProfile(page);
    await expect(page.locator('#profile-email')).toBeDisabled();
  });

  authTest('name field is pre-populated', async ({ page }) => {
    await goToProfile(page);
    const nameValue = await page.inputValue('#profile-name');
    expect(nameValue.trim().length).toBeGreaterThan(0);
  });
});

// ---------------------------------------------------------------------------
// Profile page — name update
// ---------------------------------------------------------------------------

authTest.describe('Profile page — name update', () => {
  authTest('whitespace-only name is rejected with a field error', async ({ page }) => {
    await goToProfile(page);
    await page.fill('#profile-name', '   ');
    await page.click('[data-testid="profile-save"]');
    await expect(page.locator('.field-error')).toBeVisible();
    await expect(page.locator('.field-error')).toContainText('required');
  });

  authTest('valid name change shows success banner and persists', async ({ page }) => {
    await goToProfile(page);

    const originalName = await page.inputValue('#profile-name');
    const newName = `E2E Test ${Date.now()}`;

    await page.fill('#profile-name', newName);
    await page.click('[data-testid="profile-save"]');
    await expect(page.locator('.success-banner')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('.success-banner')).toContainText('updated');

    // Restore original name so other tests are not affected
    await page.fill('#profile-name', originalName || 'Admin');
    await page.click('[data-testid="profile-save"]');
    await expect(page.locator('.success-banner')).toBeVisible({ timeout: 10000 });
  });
});

// ---------------------------------------------------------------------------
// Profile page — password change section (client-side validation)
// ---------------------------------------------------------------------------

authTest.describe('Profile page — password change section', () => {
  authTest('password form is hidden initially', async ({ page }) => {
    await goToProfile(page);
    await expect(page.locator('.password-form')).not.toBeVisible();
  });

  authTest('clicking the toggle shows and then hides the form', async ({ page }) => {
    await goToProfile(page);
    await page.click('.password-toggle');
    await expect(page.locator('.password-form')).toBeVisible();
    await page.click('.password-toggle');
    await expect(page.locator('.password-form')).not.toBeVisible();
  });

  authTest('same current and new password shows a validation error', async ({ page }) => {
    await goToProfile(page);
    await openPasswordSection(page);

    await page.fill('#current-password', 'mypassword');
    await page.fill('#new-password', 'mypassword');
    await page.fill('#confirm-password', 'mypassword');
    await page.click('[data-testid="change-password-submit"]');

    // The React form surfaces cross-field validation errors at the
    // offending field rather than as a top-of-form banner, so we drive
    // the field-scoped testid directly. (Server-error banner stays on
    // `.password-form .error-banner` for actual API failures.)
    const newErr = page.locator('[data-testid="new-password-error"]');
    await expect(newErr).toBeVisible();
    await expect(newErr).toContainText('differ');
  });

  authTest('mismatched confirmation shows a validation error', async ({ page }) => {
    await goToProfile(page);
    await openPasswordSection(page);

    await page.fill('#current-password', 'currentpass');
    await page.fill('#new-password', 'newpassword1');
    await page.fill('#confirm-password', 'newpassword2');
    await page.click('[data-testid="change-password-submit"]');

    const confirmErr = page.locator('[data-testid="confirm-password-error"]');
    await expect(confirmErr).toBeVisible();
    await expect(confirmErr).toContainText('match');
  });
});

// ---------------------------------------------------------------------------
// Profile page — password change API
// The success case is mocked to avoid permanently changing the test user's
// credentials. The wrong-password case uses the real server (no state change).
// ---------------------------------------------------------------------------

authTest.describe('Profile page — password change API', () => {
  authTest('wrong current password shows inline error without redirecting', async ({ page }) => {
    // No mock — real server returns 422 for a wrong current password.
    // The test user's actual password is never submitted, so no state changes.
    await goToProfile(page);
    await openPasswordSection(page);

    await page.fill('#current-password', 'DefinitelyWrong999');
    await page.fill('#new-password', 'NewPassword456');
    await page.fill('#confirm-password', 'NewPassword456');
    await page.click('[data-testid="change-password-submit"]');

    await expect(page.locator('.password-form .error-banner')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('.password-form .error-banner')).toContainText('incorrect');
    await expect(page).toHaveURL(/\/profile/);
  });

  authTest('success shows banner with logout notice and redirects to /login', async ({ page }) => {
    await page.route('**/api/v1/auth/change-password', route =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'Password changed successfully' }),
      })
    );

    await goToProfile(page);
    await openPasswordSection(page);

    await page.fill('#current-password', 'currentpassword');
    await page.fill('#new-password', 'newpassword123');
    await page.fill('#confirm-password', 'newpassword123');
    await page.click('[data-testid="change-password-submit"]');

    await expect(page.locator('.password-form .success-banner')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('.password-form .success-banner')).toContainText('signed out');
    // After the 2-second timeout in the component, the router pushes to /login
    await expect(page).toHaveURL(/\/login/, { timeout: 10000 });
  });
});

// ---------------------------------------------------------------------------
// Login page — session expired message
// (raw test — no auth fixture so the router does not redirect away from /login)
// ---------------------------------------------------------------------------

test.describe('Login page — session expired message', () => {
  test('shows the session message when reason=session_expired', async ({ page }) => {
    await page.goto('/login?reason=session_expired');
    await expect(page.locator('.session-message')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('.session-message')).toContainText('session');
  });

  test('does not show session message on a plain login page', async ({ page }) => {
    await page.goto('/login');
    await expect(page.locator('.session-message')).not.toBeVisible();
  });
});
