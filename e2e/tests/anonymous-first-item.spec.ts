/**
 * Anonymous "add your first item before login" journey (#1988).
 *
 * Exercises the no-data-loss guarantee end-to-end: a logged-out visitor
 * fills the create form on the public landing page, is sent to REGISTER (the
 * anonymous fill is framed as new-user onboarding), and after auth the
 * drafted item is replayed into their group with nothing lost. A real new
 * user would register + verify their email before signing in; this spec
 * reuses an already-seeded account to exercise the replay mechanism itself,
 * relying on the pending-first-item marker surviving the navigation to /login.
 *
 * The landing "Add New Item" card is gated on the `public_scan` feature flag
 * (the public AI-scan endpoint is mounted only when the operator opts in).
 * Rather than coordinate that backend env across the whole e2e job matrix —
 * the same call ai-scan.spec.ts made for the mock provider — we stub
 * GET /api/v1/feature-flags to report `public_scan: true`. The AI scan itself
 * is skipped via "Fill manually", so the flow has NO dependency on the scan
 * endpoint; the post-login replay POSTs to the REAL backend, so the item is
 * genuinely created and verified.
 *
 * Plain @playwright/test (no app-fixture) so the page starts logged OUT —
 * RootGate only renders the landing surface for an anonymous visitor.
 */
import { expect, test } from '@playwright/test';

import {
  fillCommodityWizardAndSubmit,
  verifyCommodityDetails,
  type TestCommodity,
} from './includes/commodities.js';
import { deleteCommodityViaAPI, extractApiAuth } from './includes/commodities-api.js';
import { TEST_CREDENTIALS } from './includes/auth.js';
import { SEEDED_TEST_USERS } from './includes/user-isolation-auth.js';

const MARKER_KEY = 'inventario_pending_first_item';
const DRAFT_KEY = 'commodity-draft:anon:create';
const DETAIL_URL = /\/g\/[^/]+\/commodities\/[0-9a-fA-F-]{36}/;

// Stub /feature-flags so the landing Add card renders without touching the
// backend's PUBLIC_AI_VISION_SCAN_ENABLED config. We pass the REAL upstream
// response through (status + headers via `response`) and only flip
// public_scan in the body — so any other (or future) flags keep their real
// values, AND a broken /feature-flags still surfaces: a non-200 upstream
// leaves the flag false, the Add card stays hidden, and the test fails,
// rather than being masked by a synthetic 200.
async function enablePublicScanFlag(page: import('@playwright/test').Page) {
  await page.route('**/api/v1/feature-flags', async (route) => {
    const response = await route.fetch();
    const body = (await response.json()) as Record<string, unknown>;
    body.public_scan = true;
    await route.fulfill({ response, json: body });
  });
}

test.describe('Anonymous first-item journey (#1988)', () => {
  test('logged-out → fill → login → item replayed into the group, nothing lost', async ({
    page,
    request,
  }) => {
    const suffix = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    // Currency is the seeded e2e group's currency (CZK) on purpose: the
    // anonymous dialog infers USD from the browser locale, so picking CZK
    // exercises the foreign-currency converted-price field pre-login, and
    // the post-login replay (toRequest re-run against the real CZK group)
    // collapses to same-currency — no cross-currency validation surprise.
    const commodity: TestCommodity = {
      name: `Anon First Item ${suffix}`,
      shortName: `Anon-${suffix.slice(-6)}`,
      type: 'electronics',
      count: 1,
      originalPrice: 42,
      originalPriceCurrency: 'CZK',
      convertedOriginalPrice: 42,
      currentPrice: 42,
      purchaseDate: '2026-01-15',
    };

    await enablePublicScanFlag(page);

    // 1. Logged out, "/" is the landing CTA (RootGate → LandingPage).
    await page.goto('/');
    await expect(page.getByTestId('landing-page')).toBeVisible();
    await expect(page.getByTestId('landing-add-item')).toBeVisible();

    // 2. Open the anonymous create dialog and skip the AI offer.
    await page.getByTestId('landing-add-item').click();
    await page.waitForSelector('[data-testid="commodity-form-dialog"]');
    await page.waitForSelector('[data-testid="commodity-form-ai-step"]', {
      state: 'visible',
      timeout: 10000,
    });
    await page.click('[data-testid="commodity-form-ai-fill-manually"]');

    // 3. Fill the wizard and submit. The anonymous submit is a pure hand-off:
    //    it stashes the draft + the pending-first-item marker and redirects to
    //    REGISTER (the fill is new-user onboarding) — it does NOT POST, so we
    //    land on /register, not a detail page.
    await fillCommodityWizardAndSubmit(page, commodity);
    await page.waitForURL(/\/register(\?.*)?$/, { timeout: 15000 });

    // The hand-off marker is set before the redirect.
    expect(await page.evaluate((k) => localStorage.getItem(k), MARKER_KEY)).not.toBeNull();

    // 4. A brand-new visitor would register + verify their email before
    //    signing in; this spec reuses a seeded account, so we head straight to
    //    /login. The pending-first-item marker lives in localStorage and
    //    survives the navigation. LoginPage sees it and routes to /welcome
    //    instead of the dashboard (peek, not consume — the resolver owns
    //    consumption). The first-item reassurance drawer (#1988) auto-opens
    //    over the form on arrival; dismiss it ("Got it") first — its modal
    //    overlay otherwise intercepts the form clicks. Wait for it to fully
    //    close so its exit animation can't race the form.
    await page.goto('/login');
    await page.getByTestId('pending-first-item-drawer-ok').click();
    await page.getByTestId('pending-first-item-drawer').waitFor({ state: 'hidden' });
    await page.getByTestId('email').fill(TEST_CREDENTIALS.email);
    await page.getByTestId('password').fill(TEST_CREDENTIALS.password);
    await page.getByTestId('login-button').click();

    // 5. FirstItemResolver replays the stash. The seeded admin has >1 group,
    //    so it shows the in-page picker; pick the first group. (Resilient to a
    //    single-group seed too, where it would silently skip straight to the
    //    detail page.)
    const picker = page.getByTestId('first-item-resolver-picker');
    await Promise.race([
      picker.waitFor({ state: 'visible', timeout: 30000 }),
      page.waitForURL(DETAIL_URL, { timeout: 30000 }),
    ]);
    if (await picker.isVisible().catch(() => false)) {
      await page.getByTestId('first-item-resolver-group').first().click();
    }

    // 6. Land on the new item's detail page with the entered values intact.
    await page.waitForURL(DETAIL_URL, { timeout: 30000 });
    const detail = new URL(page.url()).pathname.match(
      /\/g\/([^/]+)\/commodities\/([0-9a-fA-F-]{36})/,
    );
    // waitForURL(DETAIL_URL) just passed, so the match is guaranteed — assert
    // it (rather than `if (detail)`) so a future URL-shape change fails loudly
    // here instead of silently skipping the isolation probe + cleanup below.
    expect(detail, 'replay should land on /g/<slug>/commodities/<id>').not.toBeNull();
    const slug = decodeURIComponent(detail![1]);
    const id = detail![2];
    await verifyCommodityDetails(page, commodity);

    // 7. After a successful replay the marker + anonymous draft are cleared.
    expect(await page.evaluate((k) => localStorage.getItem(k), MARKER_KEY)).toBeNull();
    expect(await page.evaluate((k) => localStorage.getItem(k), DRAFT_KEY)).toBeNull();

    // 8. Tenant/group isolation (AGENTS.md "multi-tenant isolation testing"):
    //    a different seeded user — SEEDED_TEST_USERS[1] (user2), NOT a member
    //    of the owner's group — must not be able to read the replayed item.
    //    The generic case lives in group-data-isolation.spec.ts; this keeps
    //    the assertion attached to the #1988 replay path, which lands a real
    //    row inside the owner's RLS boundary.
    const otherUser = SEEDED_TEST_USERS[1];
    // Guard that the probe genuinely uses a different identity than the owner
    // (TEST_CREDENTIALS === SEEDED_TEST_USERS[0]) — a same-user probe would
    // return 200 and silently invalidate the isolation assertion.
    expect(otherUser.email).not.toBe(TEST_CREDENTIALS.email);
    const otherLogin = await request.post('/api/v1/auth/login', {
      data: { email: otherUser.email, password: otherUser.password },
    });
    expect(otherLogin.ok()).toBeTruthy();
    const otherToken = (await otherLogin.json()).access_token as string;
    // Fail fast if the login-response token contract ever changes.
    expect(otherToken).toBeTruthy();
    const probe = await request.get(
      `/api/v1/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`,
      {
        headers: {
          Accept: 'application/vnd.api+json',
          Authorization: `Bearer ${otherToken}`,
        },
      },
    );
    expect([403, 404]).toContain(probe.status());

    // Cleanup: the replay created a REAL commodity in the shared seeded
    // backend. Delete it (as the owner) so it can't perturb later specs
    // (counts / ordering) or make local re-runs noisier. Best-effort —
    // a failed delete must not fail an otherwise-green test.
    const auth = await extractApiAuth(page);
    await deleteCommodityViaAPI(request, auth, slug, id).catch(() => {});
  });
});
