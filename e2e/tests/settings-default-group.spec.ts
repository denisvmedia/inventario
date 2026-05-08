/**
 * E2E tests for the Settings page default-group surface (#1592).
 *
 * The deeper #1592 invariant — first membership becomes default, default
 * re-promotes when removed, etc. — is locked in by Go unit tests in
 * services/group_service_test.go. These specs guard the user-facing surface:
 *
 * 1. Authenticated user with memberships sees a working <select> in the
 *    Account section, defaulted to their saved default_group_id.
 * 2. Authenticated user with zero memberships sees the "Create your first
 *    group" call-to-action instead.
 *
 * The two cases use disjoint seeded users (admin / orphan) and only read
 * the Settings page — no membership mutations — so they're safe to run in
 * parallel with the rest of the suite.
 */
import { Page, expect, test } from '@playwright/test';
import waitOn from 'wait-on';
import { test as authTest } from '../fixtures/app-fixture.js';
import { login, ORPHAN_TEST_CREDENTIALS } from './includes/auth.js';
import { BASE_URL } from '../setup/urls.js';

async function loginAsOrphan(page: Page): Promise<void> {
  await page.goto('/login');
  await login(page, undefined, ORPHAN_TEST_CREDENTIALS);
}

test.describe('Settings — default group (#1592)', () => {
  test.beforeAll(async () => {
    await waitOn({
      resources: [BASE_URL],
      timeout: 15000,
      interval: 250,
      window: 1000,
      tcpTimeout: 1000,
    });
  });

  test('zero-group user sees the create-first-group call-to-action', async ({ page }) => {
    await loginAsOrphan(page);
    await page.goto('/settings');
    // The settings page renders the Appearance section by default; flip to
    // Account so the default-group surface is mounted.
    await page.locator('[data-testid="settings-nav-account"]').click();

    const cta = page.locator('[data-testid="settings-no-groups-cta"]');
    await expect(cta).toBeVisible({ timeout: 10000 });

    const ctaButton = page.locator('[data-testid="settings-no-groups-cta-button"]');
    await expect(ctaButton).toBeVisible();
    // The button should link the user to the onboarding entry point, where
    // the same NoGroupPage flow that handles first-login also handles "I
    // left my last group" repair.
    const href = await ctaButton.getAttribute('href');
    expect(href).toBe('/no-group');

    // The selector must NOT be rendered for a user with zero memberships —
    // there's nothing valid to pick, and any rendered <select> would be a
    // contradiction with the empty-state CTA above it.
    await expect(page.locator('[data-testid="settings-default-group-select"]')).toHaveCount(0);
  });
});

authTest.describe('Settings — default group, authenticated user (#1592)', () => {
  authTest('admin sees the default-group selector populated with their memberships', async ({ page }) => {
    await page.goto('/settings');
    await page.locator('[data-testid="settings-nav-account"]').click();

    const select = page.locator('[data-testid="settings-default-group-select"]');
    await expect(select).toBeVisible({ timeout: 10000 });

    // The selector must default to a real group (the user's preference under
    // the #1592 invariant; backfill ensured admin has one). An empty value
    // would mean the front-end is showing "no default" copy that no longer
    // exists.
    const value = await select.inputValue();
    expect(value).not.toBe('');

    // The empty-state CTA must NOT be rendered when the user has memberships.
    await expect(page.locator('[data-testid="settings-no-groups-cta"]')).toHaveCount(0);
  });
});
