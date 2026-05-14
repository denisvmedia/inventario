import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';
import { TEST_CREDENTIALS, login } from './includes/auth.js';

// Issue #1644 — PR-B
// "two browsers, see two sessions, revoke one → other unaffected;
//  trigger 2 wrong-password attempts → both show in history with
//  bad_password outcome".
//
// The app-fixture already logs in once. We approximate the "two
// browsers" requirement by opening a second incognito context that
// performs its own login — this issues a second refresh token row
// for the same user, exactly what we want to see in /profile/sessions.
test.describe('Sessions & Login history (#1644)', () => {
  test('two sessions visible; revoking one keeps the current session alive', async ({ page, browser }) => {
    // First, ensure there are no pre-existing stale sessions from
    // an earlier test that ran in the same shared db. Sign out
    // all-other-sessions if the button shows.
    await page.goto('/profile/sessions');
    await expect(page.getByTestId('sessions-page')).toBeVisible();

    const initialCards = await page.getByTestId('session-card').count();
    if (initialCards > 1) {
      // Use the "Sign out all other sessions" CTA on the page so we
      // start the test from a clean (one-session) state.
      await page.getByTestId('sessions-revoke-all-btn').click();
      await page.getByTestId('sessions-confirm-revoke-all-btn').click();
      // Wait for the second card to disappear.
      await expect(page.getByTestId('session-card')).toHaveCount(1);
    }

    // Open a second browser context and authenticate — this creates a
    // brand-new refresh token for the same user.
    const secondCtx = await browser.newContext();
    try {
      const secondPage = await secondCtx.newPage();
      await secondPage.goto('/login');
      await login(secondPage, undefined, TEST_CREDENTIALS);

      // Back on the original page, reload the sessions list and
      // assert we now see two session cards.
      await page.goto('/profile/sessions');
      await expect(page.getByTestId('session-card')).toHaveCount(2);

      // The "current" pill must live on the original context's card.
      // Find the non-current card and revoke it.
      const nonCurrent = page.getByTestId('session-card').filter({
        has: page.locator('[data-session-current="false"]'),
      });
      await expect(nonCurrent.first()).toBeVisible();
      await nonCurrent.first().getByTestId('session-revoke-btn').click();
      await page.getByTestId('sessions-confirm-revoke-btn').click();
      await expect(page.getByTestId('session-card')).toHaveCount(1);

      // The original session — the one we are using on the test page —
      // must still be alive: the page still loads, the user-menu testid
      // is still visible.
      await expect(page.locator('[data-testid="user-menu"]')).toBeVisible();
    } finally {
      await secondCtx.close();
    }
  });

  test('failed sign-in attempts surface in login history with bad_password outcome', async ({ page, request }) => {
    // Two POSTs to /api/v1/auth/login with the right email but a
    // wrong password — the BE records a bad_password login_event for
    // each. We bypass the FE login form here because the form
    // helpfully clears the password on failure; raw API calls are
    // cleaner and exercise the exact code path the issue's
    // acceptance criteria points at.
    for (let i = 0; i < 2; i++) {
      const response = await request.post('/api/v1/auth/login', {
        data: {
          email: TEST_CREDENTIALS.email,
          password: 'definitely-wrong-' + i,
        },
        headers: { 'Content-Type': 'application/json' },
      });
      expect(response.status()).toBe(401);
    }

    // Visit the login-history page (using the same logged-in
    // session) and assert there are at least 2 bad_password rows.
    await page.goto('/profile/login-history');
    await expect(page.getByTestId('login-history-page')).toBeVisible();
    await expect(page.getByTestId('login-history-row').filter({
      has: page.locator('[data-outcome="bad_password"]'),
    })).toHaveCount(2, { timeout: 10000 });
  });
});
