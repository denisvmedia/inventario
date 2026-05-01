import { expect, test } from '@playwright/test';

import { axeAudit } from '../utils/axe.js';

/**
 * Smoke for the React frontend (#1397 / #1419).
 *
 * The React bundle mounts on `<div id="root">`; we assert that as the
 * simplest contract test that the right HTML is being served. We also
 * check the document title matches the brand string i18n's
 * `common:documentTitle` resolves to ("<title> · Inventario").
 *
 * This file covers what the bundle can render without auth: the catch-all
 * 404 (a real, fully styled page from #1404), the document title pattern,
 * and an axe pass on each, plus the unauth → /login bounce for the
 * commodity routes from #1410.
 */

test.describe('Frontend shell', () => {
  test('serves the React mount point at "/"', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('#root')).toBeAttached();
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
  // and gate-protected. Without a shared login fixture we can't drive
  // a full CRUD — that's a follow-up (#1449) — but we can assert
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

  // Authenticated smoke: drive login through the UI, create the
  // commodity via the BE API (the multi-step Add Item dialog is
  // covered by 327 vitest cases — re-asserting it through Playwright
  // is brittle: option ordering depends on the seeded fixtures, and
  // the schema's whenNotDraft rules differ across browsers due to
  // form-event timing), then exercise the most-valuable UI flow:
  // Sheet preview → "View full details" → delete confirmation.
  // Full UI-driven CRUD is tracked in #1449 (shared login fixture).
  test('logged-in user can preview and delete a commodity', async ({ page }) => {
    test.setTimeout(60_000);
    await page.goto('/login');
    await page.getByTestId('email').fill('admin@test-org.com');
    await page.getByTestId('password').fill('testpassword123');
    await page.getByTestId('login-button').click();
    // RootRedirect bounces to /g/<default-slug>; wait for a real
    // group-scoped URL before navigating onward.
    await expect(page).toHaveURL(/\/g\/[a-zA-Z0-9_-]+/, { timeout: 15_000 });

    const groupUrl = new URL(page.url());
    const segments = groupUrl.pathname.split('/');
    const slug = segments[2];

    // The React frontend authenticates via Bearer token in localStorage
    // + X-CSRF-Token in sessionStorage (see frontend/src/lib/
    // auth-storage.ts). Cookies aren't used for the API, so neither
    // page.request nor the top-level `request` fixture would carry
    // creds — drive direct API calls through page.evaluate so they
    // run inside the SPA's origin and pick up storage automatically.
    const stamp = Date.now();
    const itemName = `e2e-react-${stamp}`;
    const createResult = await page.evaluate(
      async ({ slugArg, name }) => {
        const token = localStorage.getItem('inventario_token');
        const csrf = sessionStorage.getItem('inventario_csrf_token');
        const headers: Record<string, string> = {
          'Content-Type': 'application/json',
        };
        if (token) headers.Authorization = `Bearer ${token}`;
        if (csrf) headers['X-CSRF-Token'] = csrf;
        const areasR = await fetch(`/api/v1/g/${slugArg}/areas`, { headers });
        const areasBody = await areasR.json();
        const firstArea = areasBody?.data?.[0]?.id;
        if (!firstArea) {
          return { ok: false, status: 0, error: 'no seeded areas' };
        }
        const r = await fetch(`/api/v1/g/${slugArg}/commodities`, {
          method: 'POST',
          headers,
          body: JSON.stringify({
            data: {
              type: 'commodities',
              attributes: {
                name,
                short_name: 'e2e',
                type: 'other',
                area_id: firstArea,
                status: 'in_use',
                count: 1,
                // Currency needs to be well-formed even in draft mode.
                // The schema's whenNotDraft block skips the *required*
                // check, but the bojanz/currency type's own well-formed
                // validator runs unconditionally on the unmarshalled
                // value — empty string fails. Seeded test-org's main
                // currency is CZK; any ISO 4217 code works.
                original_price_currency: 'CZK',
                draft: true,
              },
            },
          }),
        });
        return { ok: r.ok, status: r.status, error: r.ok ? undefined : await r.text() };
      },
      { slugArg: slug, name: itemName },
    );
    expect(
      createResult.ok,
      `create commodity should succeed (got ${createResult.status}: ${createResult.error ?? ''})`,
    ).toBeTruthy();

    // Land on the list with `?inactive=1` so drafts are visible.
    // The default list filter sets `include_inactive=false` which hides
    // both drafts and non-`in_use` rows; without the toggle the freshly
    // created draft never shows up. The page reads the URL param as
    // `inactive=1` (boolean encoded as "1"/"0", not "true"/"false") —
    // see CommoditiesListPage.tsx line 114.
    await page.goto(`/g/${slug}/commodities?inactive=1`);
    await expect(page.getByTestId('page-commodities')).toBeVisible();
    await expect(page.getByText(itemName)).toBeVisible({ timeout: 10_000 });

    // Click the row title → Sheet preview opens. The bare-click
    // guard from #1410 intercepts the click and opens the overlay
    // instead of navigating; modifier-clicks fall through to the link.
    await page.getByText(itemName).click();
    await expect(page.getByTestId('commodity-preview-sheet')).toBeVisible();

    // "View full details" → canonical detail page.
    await page.getByTestId('commodity-preview-open').click();
    await expect(page.getByTestId('page-commodity-detail')).toBeVisible();

    // Delete via the detail page action. ConfirmProvider locks body
    // scroll while the modal is up — that's the signal it's open.
    await page.getByTestId('commodity-detail-delete').click();
    await expect(page.locator('body[data-scroll-locked]')).toBeVisible();
    await page
      .getByRole('button', { name: /^Delete$/, exact: true })
      .last()
      .click();

    // Detail page bounces back to the list; the row is gone.
    await expect(page).toHaveURL(/\/commodities$/);
    await expect(page.getByText(itemName)).not.toBeVisible();
  });
});
