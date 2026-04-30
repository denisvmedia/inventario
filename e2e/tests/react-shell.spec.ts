import { expect, test } from '@playwright/test';

import { axeAudit } from '../utils/axe.js';

/**
 * @react-only smoke for the React frontend (#1397 / #1419).
 *
 * The legacy bundle mounts on `<div id="app">` and the React bundle mounts
 * on `<div id="root">` — the simplest contract test for "we're hitting the
 * right stack" is which root element the served HTML carries. We also
 * check the document title matches the brand string i18n's
 * `common:documentTitle` resolves to ("<title> · Inventario").
 *
 * Once #1407 (Auth pages) lands, this file is the natural home for the
 * deferred #1404 smoke — login → land on dashboard for default group →
 * switch group via URL → distinct data. Until then we cover what the
 * React bundle CAN render today: the catch-all 404 (a real, fully styled
 * page from #1404), the document title pattern, and an axe pass on each.
 */

test.describe('@react-only React frontend shell', () => {
  test('serves the React mount point at "/"', async ({ page }) => {
    await page.goto('/');
    // The Go binary picks the bundle at startup based on
    // INVENTARIO_FRONTEND. If this assert fails, the wrong stack is up
    // (most likely INVENTARIO_FRONTEND=legacy was set when the user
    // intended to run the new project).
    await expect(page.locator('#root')).toBeAttached();
    await expect(page.locator('#app')).toHaveCount(0);
  });

  test('document title interpolates the brand', async ({ page }) => {
    // Visit a route that resolves to a real page (not a placeholder) so
    // RouteTitle renders a non-trivial title. Catch-all 404 is reachable
    // without auth and uses the `errors:notFound.documentTitle` key.
    await page.goto('/some-nonexistent-route');
    // Title pattern: "<page> · Inventario" — see common:documentTitle.
    await expect(page).toHaveTitle(/Inventario$/);
  });

  test('catch-all 404 renders the styled NotFound page', async ({ page }) => {
    await page.goto('/some-nonexistent-route');
    await expect(page.getByTestId('page-not-found')).toBeVisible();
    await expect(page.getByRole('heading', { name: /page not found/i, level: 1 })).toBeVisible();
    await expect(page.getByRole('link', { name: /go home/i })).toBeVisible();
  });

  test('NotFound page is axe-clean', async ({ page }) => {
    await page.goto('/some-nonexistent-route');
    await expect(page.getByTestId('page-not-found')).toBeVisible();
    await axeAudit(page);
  });

  // Smoke: confirm the new commodity-page routes from #1410 are wired
  // and gate-protected. Without a login fixture for @react-only specs
  // we can't drive a full CRUD — that's a follow-up — but we can assert
  // the routing skeleton is correct and unauth → login bounce works.
  test('commodities list redirects unauthenticated users to /login', async ({ page }) => {
    await page.goto('/g/household/commodities');
    await expect(page).toHaveURL(/\/login/);
    await expect(page.locator('#root')).toBeAttached();
  });

  test('commodities new + detail + print routes redirect unauth users', async ({ page }) => {
    for (const path of [
      '/g/household/commodities/new',
      '/g/household/commodities/some-id',
      '/g/household/commodities/some-id/print',
    ]) {
      await page.goto(path);
      await expect(page, `${path} should bounce to /login`).toHaveURL(/\/login/);
    }
  });

  // Authenticated CRUD smoke. Uses the React login form directly
  // (data-testid="email"/"password"/"login-button" are stable from
  // #1407) — the legacy app-fixture's `user-menu` selector doesn't
  // exist in the React shell yet, so we drive the form ourselves.
  // Test data is timestamped to avoid collisions when the spec runs
  // alongside legacy specs sharing the same DB.
  test('logged-in user can add and delete a commodity', async ({ page }) => {
    test.setTimeout(60_000);
    await page.goto('/login');
    await page.getByTestId('email').fill('admin@test-org.com');
    await page.getByTestId('password').fill('testpassword123');
    await page.getByTestId('login-button').click();
    // RootRedirect bounces to /g/<default-slug>; wait for a real
    // group-scoped URL before navigating onward.
    await expect(page).toHaveURL(/\/g\/[a-zA-Z0-9_-]+/, { timeout: 15_000 });

    // Land on /commodities for the active group. The toolbar's "Add
    // item" button is the entry to the multi-step dialog.
    const groupUrl = new URL(page.url());
    const segments = groupUrl.pathname.split('/');
    const slug = segments[2];
    await page.goto(`/g/${slug}/commodities`);
    await expect(page.getByTestId('page-commodities')).toBeVisible();

    // Click "Add item" → dialog opens. Skip filling everything; the
    // schema lets a draft ride with just name/short_name/type/area.
    await page.getByTestId('commodities-add-button').click();
    await expect(page.getByLabel(/Form steps/i)).toBeVisible();
    const stamp = Date.now();
    const itemName = `e2e-react-${stamp}`;
    await page.getByLabel(/^Name$/i).fill(itemName);
    await page.getByLabel(/^Short name$/i).fill('e2e');
    await page.getByLabel(/^Type$/i).selectOption('other');
    // First option after the placeholder is the seeded area.
    const areaSelect = page.getByLabel(/^Area$/i);
    const areaOptions = await areaSelect.locator('option').all();
    if (areaOptions.length > 1) {
      await areaSelect.selectOption({ index: 1 });
    }
    // Tick draft so we can skip the price triad.
    await page.getByLabel(/Save as draft/i).check();
    // Walk to the final step.
    for (let i = 0; i < 4; i++) {
      await page.getByTestId('commodity-form-next').click();
    }
    await page.getByTestId('commodity-form-submit').click();
    // The new row eventually appears in the list (or the toast lands).
    await expect(page.getByText(itemName)).toBeVisible({ timeout: 10_000 });

    // Delete it. Click the row title → Sheet preview → Open full →
    // Delete. The bare-click guard from #1410 catches the click and
    // opens the Sheet preview.
    await page.getByText(itemName).click();
    await expect(page.getByTestId('commodity-preview-sheet')).toBeVisible();
    await page.getByTestId('commodity-preview-open').click();
    await expect(page.getByTestId('page-commodity-detail')).toBeVisible();
    await page.getByTestId('commodity-detail-delete').click();
    // ConfirmProvider's dialog body-scroll-lock signals the modal is
    // up. Click Delete inside it.
    await expect(page.locator('body[data-scroll-locked]')).toBeVisible();
    await page
      .getByRole('button', { name: /^Delete$/, exact: true })
      .last()
      .click();
    // Detail page navigates back to the list; the row is gone.
    await expect(page).toHaveURL(/\/commodities$/);
    await expect(page.getByText(itemName)).not.toBeVisible();
  });
});
