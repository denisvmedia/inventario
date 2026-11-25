import { expect, request as playwrightRequest, type APIRequestContext } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';
import { TEST_CREDENTIALS } from './includes/auth.js';
import { BASE_URL } from '../setup/urls.js';

// Personal API tokens (#2808): create one under Settings → Privacy &
// security, see it once, use it from a client that has no cookie and no
// CSRF token, and revoke it.
//
// The API calls go through a fresh request context so nothing from the
// browser session (refresh cookie, access token) can make a call succeed
// that the token alone would not.
test.describe('Personal API tokens (#2808)', () => {
  test('create, use from a script, and revoke', async ({ page }) => {
    const name = `e2e token ${Date.now()}`;

    await page.goto('/settings');
    await page.click('[data-testid="settings-nav-privacy"]');
    await expect(page.getByTestId('section-privacy')).toBeVisible();
    await page.getByTestId('privacy-row-apiTokens').click();
    await expect(page.getByTestId('api-tokens-page')).toBeVisible();

    await page.getByTestId('api-tokens-create-btn').click();
    const dialog = page.getByTestId('api-token-create-dialog');
    await expect(dialog).toBeVisible();
    await dialog.getByTestId('api-token-name').fill(name);
    await dialog.getByTestId('api-token-scope-read_write').click();
    await dialog.getByTestId('api-token-password').fill(TEST_CREDENTIALS.password);
    await dialog.getByTestId('api-token-create-submit').click();

    const secretInput = dialog.getByTestId('api-token-secret');
    await expect(secretInput).toBeVisible();
    const secret = await secretInput.inputValue();
    expect(secret).toMatch(/^inv_pat_[0-9A-Za-z]{43}$/);

    await dialog.getByTestId('api-token-reveal-done').click();
    await expect(dialog).toBeHidden();

    const card = page.locator('[data-testid="api-token-card"]', { hasText: name });
    await expect(card).toBeVisible();
    await expect(card.getByTestId('api-token-scope-badge')).toHaveText('Read and write');
    // The list never shows the token itself.
    await expect(page.getByText(secret)).toHaveCount(0);

    const script: APIRequestContext = await playwrightRequest.newContext({ baseURL: BASE_URL });
    try {
      const auth = { Authorization: `Bearer ${secret}` };

      const groups = await script.get('/api/v1/groups', { headers: auth });
      expect(groups.status()).toBe(200);
      const body = await groups.json();
      const slug: string | undefined = body.data?.[0]?.attributes?.slug;
      expect(slug).toBeTruthy();

      const locations = await script.get(`/api/v1/g/${slug}/locations`, { headers: auth });
      expect(locations.status()).toBe(200);

      // A write goes through without a CSRF token: no cookie is involved.
      const created = await script.post(`/api/v1/g/${slug}/locations`, {
        headers: { ...auth, 'Content-Type': 'application/json' },
        data: { data: { type: 'locations', attributes: { name: `${name} location`, address: 'API' } } },
      });
      expect(created.status()).toBe(201);
      const locationId: string = (await created.json()).data.id;

      // Account and token-management endpoints refuse the token.
      const me = await script.get('/api/v1/auth/me', { headers: auth });
      expect(me.status()).toBe(403);
      const mint = await script.post('/api/v1/users/me/api-tokens', {
        headers: { ...auth, 'Content-Type': 'application/json' },
        data: { name: 'minted by a token', scope: 'read_write' },
      });
      expect(mint.status()).toBe(403);

      // Clean up the location while the token still works.
      const del = await script.delete(`/api/v1/g/${slug}/locations/${locationId}`, { headers: auth });
      expect(del.status()).toBe(204);

      // The use shows up on the card once the list refreshes.
      await page.reload();
      await expect(card.getByTestId('api-token-usage')).not.toContainText('Never used');

      await card.getByTestId('api-token-revoke-btn').click();
      await page.getByTestId('api-tokens-confirm-revoke-btn').click();
      await expect(card).toHaveCount(0);

      const afterRevoke = await script.get('/api/v1/groups', { headers: auth });
      expect(afterRevoke.status()).toBe(401);
    } finally {
      await script.dispose();
    }
  });
});
