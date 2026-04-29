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
});
