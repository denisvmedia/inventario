/**
 * E2E coverage for the self-service account-deletion Danger Zone dialog
 * (#2147). The backend contract — password re-auth, sole-owner-of-shared
 * rejection, full-purge on the happy path — is covered at the service level
 * in go/services/account_deletion_test.go and at the handler level in
 * go/apiserver/auth.go. These tests drive the dialog end-to-end to guard the
 * UX pieces:
 *
 *   1) Happy path — a DISPOSABLE user who is the sole member of their own
 *      (private) group can erase their account: DELETE /auth/me → 204 →
 *      redirect to /login, and a subsequent re-login fails (the row is gone).
 *   2) Wrong password — surfaces the inline password error; the dialog stays
 *      open and the user remains signed in (nothing is mutated).
 *   3) Last-owner block — a user who is the sole owner of a SHARED group
 *      (other members exist) is blocked with the lastOwner banner; the dialog
 *      stays open and the user remains signed in.
 *
 * Each scenario logs in as a DEDICATED disposable seed user (see
 * debug/seeddata/seeddata_delete_targets.go). The shared admin account
 * (TEST_CREDENTIALS) is NEVER used as a deletion target — erasing it would
 * break every other spec.
 */
import { Page, expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';
import {
  login,
  DELETE_SOLO_TARGET_TEST_CREDENTIALS,
  DELETE_WRONG_PASSWORD_TEST_CREDENTIALS,
  DELETE_LAST_OWNER_TEST_CREDENTIALS,
} from './includes/auth.js';

// loginAs switches the session to a specific disposable seed user. It mirrors
// the proven user-switch pattern used by no-group-redirect.spec.ts and
// settings-default-group.spec.ts: navigate to /login and log in as the target,
// letting that fresh login replace whatever session the app-fixture planted.
//
// IMPORTANT: do NOT clear localStorage/CSRF before navigating. The access token
// lives in localStorage but the refresh token is an httpOnly cookie — clearing
// only the token leaves the cookie valid, so the unauthenticated boot silently
// refreshes the session and RootGate redirects away from /login before the form
// renders, hanging waitForSelector('input[type="email"]'). login() itself waits
// for the email field, so no explicit pre-wait is needed.
async function loginAs(page: Page, credentials: { email: string; password: string }): Promise<void> {
  await page.goto('/login');
  await login(page, undefined, credentials);
}

// openDeleteAccountDialog navigates to /settings (Account is the default
// landing tab — #1888) and opens the Danger Zone delete dialog.
async function openDeleteAccountDialog(page: Page): Promise<void> {
  await page.goto('/settings');
  await page.waitForSelector('[data-testid="delete-account-button"]', { timeout: 10000 });
  await page.click('[data-testid="delete-account-button"]');
  await page.waitForSelector('[data-testid="delete-confirm-email"]', { state: 'visible', timeout: 5000 });
}

test.describe('Self-service account deletion (#2147)', () => {
  test('happy path: sole-member user erases their account and cannot log back in', async ({ page }) => {
    const target = DELETE_SOLO_TARGET_TEST_CREDENTIALS;
    await loginAs(page, target);
    await openDeleteAccountDialog(page);

    // Typing the WRONG email keeps the destructive submit blocked — the
    // client-side accidental-click guard sets a field error and short-circuits
    // before any network call, so the account survives a fat-fingered confirm.
    await page.fill('[data-testid="delete-confirm-email"]', 'not-my-email@test-org.com');
    await page.fill('[data-testid="delete-password"]', target.password);
    await page.click('[data-testid="delete-account-submit"]');
    await expect(page.locator('[data-testid="delete-confirm-email-error"]')).toBeVisible();
    // Still on /settings, dialog still open — no DELETE was issued.
    await expect(page.locator('[data-testid="delete-account-dialog"]')).toBeVisible();
    await expect(page).toHaveURL(/\/settings$/);

    // Correct email + correct password → the real delete fires.
    await page.fill('[data-testid="delete-confirm-email"]', target.email);
    await page.fill('[data-testid="delete-password"]', target.password);

    const deleteResp = page.waitForResponse(
      (r) => r.url().includes('/api/v1/auth/me') && r.request().method() === 'DELETE',
    );
    await page.click('[data-testid="delete-account-submit"]');
    const resp = await deleteResp;
    expect(resp.status()).toBe(204);

    // The FE logs out + redirects to /login after the 204.
    await expect(page).toHaveURL(/\/login(\?.*)?$/, { timeout: 15000 });

    // The account is gone — re-login must now fail. The seeded login helper
    // throws on a non-200, so a successful login here would mean the account
    // wasn't actually erased.
    await page.waitForSelector('input[type="email"]', { timeout: 30000 });
    await page.fill('input[type="email"]', target.email);
    await page.fill('input[type="password"]', target.password);
    const reLoginResp = page.waitForResponse(
      (r) => r.url().includes('/api/v1/auth/login'),
      { timeout: 20000 },
    );
    await page.click('button[type="submit"]');
    expect((await reLoginResp).status()).toBe(401);
    // No token was minted and we stay on /login.
    await expect(page).toHaveURL(/\/login(\?.*)?$/);
    expect(await page.evaluate(() => localStorage.getItem('inventario_token'))).toBeNull();
  });

  test('wrong password surfaces the inline password error and keeps the user signed in', async ({ page }) => {
    const target = DELETE_WRONG_PASSWORD_TEST_CREDENTIALS;
    await loginAs(page, target);
    await openDeleteAccountDialog(page);

    await page.fill('[data-testid="delete-confirm-email"]', target.email);
    await page.fill('[data-testid="delete-password"]', 'definitely-not-the-real-password');

    const deleteResp = page.waitForResponse(
      (r) => r.url().includes('/api/v1/auth/me') && r.request().method() === 'DELETE',
    );
    await page.click('[data-testid="delete-account-submit"]');
    expect((await deleteResp).status()).toBe(422);

    // Inline password error visible; dialog stays open; still on /settings.
    await expect(page.locator('[data-testid="delete-password-error"]')).toBeVisible();
    await expect(page.locator('[data-testid="delete-account-dialog"]')).toBeVisible();
    await expect(page).toHaveURL(/\/settings$/);
    // Token survived — the user is still authenticated.
    expect(await page.evaluate(() => localStorage.getItem('inventario_token'))).not.toBeNull();
  });

  test('last owner of a shared group is blocked with the lastOwner banner', async ({ page }) => {
    const target = DELETE_LAST_OWNER_TEST_CREDENTIALS;
    await loginAs(page, target);
    await openDeleteAccountDialog(page);

    await page.fill('[data-testid="delete-confirm-email"]', target.email);
    await page.fill('[data-testid="delete-password"]', target.password);

    const deleteResp = page.waitForResponse(
      (r) => r.url().includes('/api/v1/auth/me') && r.request().method() === 'DELETE',
    );
    await page.click('[data-testid="delete-account-submit"]');
    expect((await deleteResp).status()).toBe(422);

    // The server-error banner shows the lastOwner copy ("transfer ownership"),
    // the dialog stays open, and the user remains signed in.
    const banner = page.locator('[data-testid="delete-server-error"]');
    await expect(banner).toBeVisible();
    await expect(banner).toContainText(/transfer ownership/i);
    await expect(page.locator('[data-testid="delete-account-dialog"]')).toBeVisible();
    await expect(page).toHaveURL(/\/settings$/);
    expect(await page.evaluate(() => localStorage.getItem('inventario_token'))).not.toBeNull();
  });
});
