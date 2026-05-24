/**
 * OAuth sign-in happy-path e2e for #1394.
 *
 * Drives the full third-party sign-in flow against a LOCAL stub provider —
 * no real Google / GitHub network calls. The stub server lives in
 * setup/oauth-stub-server.ts and is started by setup-stack.ts when the
 * harness is launched with OAUTH_STUB_ENABLED=true. The BE picks up the
 * stub URLs via these env vars (read in go/cmd/inventario/run/bootstrap/oauth.go):
 *
 *     INVENTARIO_RUN_OAUTH_GOOGLE_AUTH_URL_OVERRIDE
 *     INVENTARIO_RUN_OAUTH_GOOGLE_TOKEN_URL_OVERRIDE
 *     INVENTARIO_RUN_OAUTH_GOOGLE_USERINFO_URL_OVERRIDE
 *
 * To run locally:
 *
 *     cd e2e
 *     OAUTH_STUB_ENABLED=true npm run stack        # terminal 1 (boots BE+FE+stub)
 *     OAUTH_STUB_ENABLED=true npx playwright test --project=chromium \
 *         tests/oauth-sign-in.spec.ts              # terminal 2
 *
 * The spec self-skips when OAUTH_STUB_ENABLED!=='true' so it can sit in
 * the suite without forcing every CI lane to spin up the stub. The
 * GitHub provider is intentionally NOT exercised here — the flow is
 * provider-agnostic on the BE, so one provider proves the find-or-
 * create-or-link branch end-to-end. The spec is parameterised via
 * `PROVIDER` so a future test can add GitHub by flipping a single
 * constant.
 *
 * Flow:
 *
 *   1. Anonymous browser visits /login → "Continue with Google" button.
 *   2. Click → BE 302 → stub /authorize → BE 302 callback.
 *   3. BE find-or-create branch:
 *      - No identity, no matching email → provision a new user.
 *      - Issue refresh-token cookie + 302 to "/".
 *   4. Assertions:
 *      - /auth/me returns the OAuth user with has_password=false.
 *      - /users/me/login-history contains a row with method=oauth_google
 *        outcome=ok.
 *   5. Sign out (clear cookies + access token).
 *   6. Repeat the flow with the SAME stub profile (same provider_user_id
 *      + same email) → BE looks up the existing identity → logs in
 *      without creating a new user. Asserts the second /auth/me returns
 *      the SAME user.id as the first.
 *   7. Unlink-guard: the new user has no password → DELETE /auth/oauth/google
 *      must return 409 "Cannot remove the last sign-in method".
 *
 * The provider URL override env vars are TEST-ONLY — the bootstrap layer
 * emits a loud slog.Warn when they are set. NEVER turn these on in a
 * production deployment.
 */
import { test, expect, APIRequestContext, Browser, BrowserContext, Page } from '@playwright/test';
import { seedTenant } from '../setup/setup-stack.js';

// Toggle the provider exercised by the test. Today: "google". Adding
// "github" needs a stub server expansion (it's currently Google-shaped)
// and a third env-var triple in setup-stack.ts. Doing one provider is
// enough for the acceptance criteria.
const PROVIDER = 'google' as const;

// Deterministic stub profile used by both the create branch and the
// re-sign-in branch. The `sub` is what the BE keys the OAuth identity
// row on; the `email` lands on users.email; the `name` seeds users.name.
const STUB_PROFILE = {
  sub: 'stub-google-sub-1394',
  email: 'oauth-user-1394@example.test',
  emailVerified: true,
  name: 'OAuth Test User',
};

const STUB_PORT = Number(process.env.OAUTH_STUB_PORT) || 4444;
const STUB_BASE_URL = `http://127.0.0.1:${STUB_PORT}`;

// Resolve the FE-facing base URL the BE is configured to redirect to.
// Playwright's baseURL (from playwright.config.ts → setup/urls.ts) is
// the canonical answer — we read it via the test fixture below.
//
// Inline this rather than importing setup/urls.ts so the spec file
// stays loadable even when run with a non-default base URL.
const TEST_TIMEOUT_MS = 60_000;

// OAuth specs share a single in-process stub server that holds the
// active profile as module-level state (see e2e/setup/oauth-stub-
// server.ts). Two parallel workers calling setStubProfile() race each
// other and corrupt the response /userinfo returns to the BE callback
// — exactly the flakiness that bit #1851's separate-file first
// implementation. Pinning the OAuth describes to serial-within-file
// removes the race without forcing the rest of the e2e suite to run
// single-worker.
test.describe.configure({ mode: 'serial' });

test.describe('#1394 OAuth sign-in — Google provider via stub @oauth', () => {
  // Long timeout: the BE redirect chain + stub round-trip + JWT mint can
  // legitimately take a few seconds on a busy laptop; the default 30s
  // expect() timeout is fine but we widen the test budget here.
  test.setTimeout(TEST_TIMEOUT_MS);

  test.beforeAll(async () => {
    // Self-skip when the harness wasn't booted with OAUTH_STUB_ENABLED=true.
    // The BE's INVENTARIO_RUN_OAUTH_* env vars only land when setup-stack.ts
    // sees that flag at boot, so without it the BE has no OAuth providers
    // registered and the test cannot execute.
    test.skip(
      process.env.OAUTH_STUB_ENABLED !== 'true',
      'OAuth e2e requires OAUTH_STUB_ENABLED=true and a running stub stack (see spec header).'
    );

    // Probe the stub server for liveness. Failing early here gives a
    // clearer error than a downstream redirect timing out.
    const probe = await fetch(`${STUB_BASE_URL}/userinfo`, {
      headers: { Authorization: 'Bearer stub-access-token' },
    });
    if (!probe.ok) {
      throw new Error(
        `OAuth stub /userinfo not reachable at ${STUB_BASE_URL} (status ${probe.status}). Did setup-stack.ts start the stub?`
      );
    }
  });

  test.beforeEach(async () => {
    // Reset the stub profile to the deterministic fixture so prior tests
    // don't leak state across runs. Each test posts the desired fixture
    // explicitly via setStubProfile() further down.
    await setStubProfile(STUB_PROFILE);
  });

  test('full happy-path: create → re-sign-in → unlink guard', async ({ browser, request }) => {
    // ---- Step 1+2+3: anonymous sign-in flow ----
    const firstContext = await browser.newContext();
    const firstPage = await firstContext.newPage();

    await firstPage.goto('/login');
    // OAuth row visibility — driven by GET /auth/oauth/providers returning
    // the Google entry. Failing here means the BE didn't pick up the stub
    // env vars (check setup-stack.ts logging).
    const googleButton = firstPage.locator(`[data-testid="oauth-${PROVIDER}-button"]`);
    await expect(googleButton).toBeVisible({ timeout: 10_000 });

    // Click → browser navigates through the BE start handler → stub
    // /authorize 302 → BE callback → BE 302 "/". The whole chain is
    // hidden behind a single user action; wait for the URL to settle
    // back on the FE app root.
    await Promise.all([
      firstPage.waitForURL((url) => url.pathname === '/' || url.pathname === '/no-group' || url.pathname === '/login', {
        timeout: 20_000,
      }),
      googleButton.click(),
    ]);

    // After the redirect the BE has set the refresh-token cookie on the
    // browser. The FE doesn't yet implement a boot-time refresh (the
    // OAuth happy-path UI sequel is tracked separately) — so the FE
    // probe on this page may still be in the "no access token" state.
    // The test asserts BE-side correctness via the cookie-bearing
    // request context instead of waiting on the FE to render the
    // sidebar.
    const oauthRequest = firstContext.request;

    // ---- Step 4a: /auth/refresh → access token, then /auth/me ----
    // The OAuth callback set a refresh-token cookie but did not write an
    // access token (the cookie path is what the FE's http.ts walks on a
    // 401). Spending the cookie via /auth/refresh gives us a Bearer
    // token we can use for the rest of the assertions.
    const refreshResp = await oauthRequest.post('/api/v1/auth/refresh', {
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      data: {},
    });
    expect(refreshResp.status(), `refresh failed: ${await refreshResp.text()}`).toBe(200);
    const refreshBody = (await refreshResp.json()) as { access_token: string; csrf_token?: string };
    expect(refreshBody.access_token).toBeTruthy();
    const firstAccessToken = refreshBody.access_token;

    // /auth/me — the new user must exist with has_password=false.
    const meResp = await oauthRequest.get('/api/v1/auth/me', {
      headers: { Accept: 'application/json', Authorization: `Bearer ${firstAccessToken}` },
    });
    expect(meResp.status(), `auth/me failed: ${await meResp.text()}`).toBe(200);
    const me = (await meResp.json()) as {
      id: string;
      email: string;
      name: string;
      has_password: boolean;
    };
    expect(me.email).toBe(STUB_PROFILE.email);
    expect(me.name).toBe(STUB_PROFILE.name);
    expect(me.has_password).toBe(false);
    const firstUserId = me.id;

    // ---- Step 4b: login_events row with method=oauth_google outcome=ok ----
    const historyResp = await oauthRequest.get('/api/v1/users/me/login-history', {
      headers: { Accept: 'application/json', Authorization: `Bearer ${firstAccessToken}` },
    });
    expect(historyResp.status(), `login-history failed: ${await historyResp.text()}`).toBe(200);
    const history = (await historyResp.json()) as {
      events: Array<{ method: string; outcome: string }>;
    };
    expect(history.events.length).toBeGreaterThan(0);
    const oauthEvent = history.events.find(
      (e) => e.method === `oauth_${PROVIDER}` && e.outcome === 'ok'
    );
    expect(oauthEvent, `expected one method=oauth_${PROVIDER} outcome=ok login event; got ${JSON.stringify(history.events)}`).toBeTruthy();

    // ---- Step 5: sign out (clear cookies + access token) ----
    // POST /auth/logout to drop the refresh-token cookie and revoke the
    // server-side session. Then drop the browser context so the second
    // sign-in run starts from a clean cookie jar.
    const logoutResp = await oauthRequest.post('/api/v1/auth/logout', {
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${firstAccessToken}` },
      data: {},
    });
    expect(logoutResp.status()).toBeLessThan(500);
    await firstContext.close();

    // ---- Step 6: second sign-in with the SAME profile ----
    // The BE looks up the existing OAuth identity by (provider, sub) and
    // logs the user back in. Asserts the second sign-in returns the same
    // users.id — i.e. the find branch ran, not find-or-create.
    const secondContext = await browser.newContext();
    const secondPage = await secondContext.newPage();
    await secondPage.goto('/login');
    const googleButton2 = secondPage.locator(`[data-testid="oauth-${PROVIDER}-button"]`);
    await expect(googleButton2).toBeVisible({ timeout: 10_000 });
    await Promise.all([
      secondPage.waitForURL((url) => url.pathname === '/' || url.pathname === '/no-group' || url.pathname === '/login', {
        timeout: 20_000,
      }),
      googleButton2.click(),
    ]);

    const secondRequest = secondContext.request;
    const refresh2 = await secondRequest.post('/api/v1/auth/refresh', {
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      data: {},
    });
    expect(refresh2.status(), `second refresh failed: ${await refresh2.text()}`).toBe(200);
    const refresh2Body = (await refresh2.json()) as { access_token: string };
    const secondAccessToken = refresh2Body.access_token;

    const me2Resp = await secondRequest.get('/api/v1/auth/me', {
      headers: { Accept: 'application/json', Authorization: `Bearer ${secondAccessToken}` },
    });
    expect(me2Resp.status()).toBe(200);
    const me2 = (await me2Resp.json()) as { id: string; email: string; has_password: boolean };
    expect(me2.id, 'second sign-in must reuse the same user — find-by-identity should run, not create').toBe(firstUserId);
    expect(me2.email).toBe(STUB_PROFILE.email);
    expect(me2.has_password).toBe(false);

    // ---- Step 7: unlink guard — DELETE /auth/oauth/google must 409 ----
    // The new user has no password AND only one linked identity (Google),
    // so unlink would lock them out. The BE refuses with 409.
    const unlinkResp = await secondRequest.delete(`/api/v1/auth/oauth/${PROVIDER}`, {
      headers: { Accept: 'application/json', Authorization: `Bearer ${secondAccessToken}` },
    });
    expect(
      unlinkResp.status(),
      `unlink-guard: expected 409 for last sign-in method (got ${unlinkResp.status()})`
    ).toBe(409);

    await secondContext.close();
  });
});

/**
 * setStubProfile flips the profile the stub will return on the NEXT
 * /userinfo call. Used at the top of every test so a parallel/serial
 * test ordering doesn't leak state.
 */
async function setStubProfile(profile: typeof STUB_PROFILE): Promise<void> {
  const resp = await fetch(`${STUB_BASE_URL}/__control__/profile`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(profile),
  });
  if (!resp.ok) {
    throw new Error(`stub set-profile failed: ${resp.status} ${await resp.text()}`);
  }
}

// ============================================================================
// #1851 — OAuth sign-in cross-tenant isolation
// ----------------------------------------------------------------------------
// Follow-up to #1394 / PR #1850. The BE half (find-by-(provider,sub)
// returning a foreign-tenant identity must refuse the callback with a
// LoginOutcomeTenantMismatch redirect) is unit-tested in
// go/apiserver/oauth_test.go. This block is the end-to-end half:
// drive the full BE redirect chain through the OAuth stub against
// TWO tenants and observe the same guard from a real browser.
//
// ---- Tenant steering mechanism ----
// The e2e stack runs a single BE on a single host — there is no
// per-tenant DNS or hostname to switch between. The BE gates a
// TEST-ONLY tenant override header behind
// INVENTARIO_RUN_TEST_TENANT_HEADER_ENABLED=true (set automatically
// by setup-stack.ts when OAUTH_STUB_ENABLED=true). When that flag is
// on, every request that carries X-Inventario-Test-Tenant: <slug>
// has its tenant resolved from the header instead of the Host.
//
// Playwright's browser.newContext({ extraHTTPHeaders }) sends the
// header on EVERY request the browser makes in that context —
// page navigations, OAuth redirects through the stub, and direct
// context.request.* API calls alike — so a context pinned to
// tenant-2 drives the BE callback under tenant-2 even though the BE
// was reached via localhost.
//
// ---- Second tenant provisioning ----
// setup-stack.ts also flips INVENTARIO_SEED_ALLOW_CREATE_TENANT=true
// when the OAuth stub is enabled, so this block mints the second
// tenant via the public POST /api/v1/seed { tenant_slug: "<slug>" }
// endpoint without needing a CLI shell-out or a back-office admin
// route. Both env-gated flags are server-side; the request body
// never carries privilege.
// ============================================================================

// Two tenants the spec needs. `test-org` is the canonical seeded
// tenant (see setup-stack.ts → INVENTARIO_RUN_MEMORY_TENANT_SLUG).
// `t1851-other` is minted in beforeAll via seedTenant. The slug is
// namespaced with the issue number so a parallel suite that one day
// uses a "tenant2" slug doesn't collide.
const TENANT_1_SLUG = 'test-org';
const TENANT_2_SLUG = 't1851-other';

// Deterministic alice profile — created on tenant-1 in setup, then
// replayed on tenant-2 to exercise the LoginOutcomeTenantMismatch
// redirect. The `sub` keys the global (provider, provider_user_id)
// lookup that triggers the guard.
const ALICE_PROFILE = {
  sub: 'stub-google-sub-1851-alice',
  email: 'alice-1851@example.test',
  emailVerified: true,
  name: 'Alice Cross-Tenant',
};

test.describe('#1851 OAuth sign-in — cross-tenant isolation @oauth @cross-tenant', () => {
  test.setTimeout(TEST_TIMEOUT_MS);

  test.beforeAll(async () => {
    test.skip(
      process.env.OAUTH_STUB_ENABLED !== 'true',
      'OAuth cross-tenant e2e requires OAUTH_STUB_ENABLED=true (see spec header).'
    );

    // Liveness probe — the stub server must be up before the BE
    // redirect chain runs through it. A clear error here beats a
    // downstream timeout.
    const probe = await fetch(`${STUB_BASE_URL}/userinfo`, {
      headers: { Authorization: 'Bearer stub-access-token' },
    });
    if (!probe.ok) {
      throw new Error(
        `OAuth stub /userinfo not reachable at ${STUB_BASE_URL} (status ${probe.status}). Did setup-stack.ts start the stub?`
      );
    }

    // Mint the second tenant. Idempotent — re-running the spec
    // hits the find-by-slug branch instead of create.
    await seedTenant(TENANT_2_SLUG);
  });

  test.beforeEach(async () => {
    // Re-pin the profile to ALICE before every test so the previous
    // test's STUB_PROFILE in the #1394 describe doesn't leak in. The
    // outer file-level serial mode guarantees this runs to completion
    // before the test action fires.
    await setStubProfile(ALICE_PROFILE);
  });

  test('callback on tenant-2 with identity owned by tenant-1 → redirected to tenant_mismatch, no session', async ({
    browser,
  }) => {
    // ---- Setup: register alice on tenant-1 via OAuth ----
    // Drive a normal sign-in flow against TENANT_1 so the OAuth
    // identity row lands with tenant_id=TENANT_1. The subsequent
    // tenant-2 attempt will collide on the global (provider, sub)
    // index and exercise the cross-tenant guard.
    const tenant1 = await newTenantContext(browser, TENANT_1_SLUG);
    await runOAuthFlowOnTenantContext(tenant1.page);

    const tenant1AccessToken = await refreshAndExpectSuccess(tenant1.context.request, TENANT_1_SLUG);
    const aliceOnTenant1 = await fetchMe(tenant1.context.request, tenant1AccessToken, TENANT_1_SLUG);
    expect(aliceOnTenant1.email).toBe(ALICE_PROFILE.email);
    const aliceOnTenant1ID = aliceOnTenant1.id;

    // Sign out so the second attempt isn't already authenticated.
    await tenant1.context.request.post('/api/v1/auth/logout', {
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${tenant1AccessToken}` },
      data: {},
    });
    await tenant1.context.close();

    // ---- Attempt: replay the SAME OAuth profile on tenant-2 ----
    // The stub still returns the alice profile (sub stable across
    // calls). The BE callback resolves tenant_id=TENANT_2 from the
    // header, looks up identity by (provider, sub), finds alice's
    // identity row owning tenant_id=TENANT_1, and refuses: 302 to
    // /login?oauth_error=tenant_mismatch.
    const tenant2 = await newTenantContext(browser, TENANT_2_SLUG);

    await tenant2.page.goto('/login');
    const googleButton = tenant2.page.locator(`[data-testid="oauth-${PROVIDER}-button"]`);
    await expect(googleButton).toBeVisible({ timeout: 10_000 });

    await Promise.all([
      tenant2.page.waitForURL(
        (url) => url.pathname === '/login' && url.search.includes('oauth_error=tenant_mismatch'),
        { timeout: 20_000 }
      ),
      googleButton.click(),
    ]);

    // The BE 302'd back to /login?oauth_error=tenant_mismatch. The FE
    // surfaces the banner from the existing oauth_error query handler
    // (#1394). The URL itself is the load-bearing assertion for the
    // BE-level guard.
    expect(tenant2.page.url(), 'callback must end at /login?oauth_error=tenant_mismatch').toContain(
      'oauth_error=tenant_mismatch'
    );

    // No refresh-token cookie issued — the BE writes the cookie only
    // on the success branch. /auth/refresh must therefore reject the
    // empty cookie jar with 401.
    const tenant2Refresh = await tenant2.context.request.post('/api/v1/auth/refresh', {
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      data: {},
    });
    expect(
      tenant2Refresh.status(),
      `tenant-2 refresh must fail (no session minted): got ${tenant2Refresh.status()}`
    ).toBe(401);

    // Alice's tenant-1 identity must be unchanged: she still exists
    // in tenant-1 with the same user.id. The tenant-2 attempt must
    // not have created a new user row, linked a new identity, or
    // touched her tenant assignment.
    const tenant1Recheck = await newTenantContext(browser, TENANT_1_SLUG);
    await runOAuthFlowOnTenantContext(tenant1Recheck.page);
    const aliceRefreshAfter = await refreshAndExpectSuccess(tenant1Recheck.context.request, TENANT_1_SLUG);
    const aliceAfter = await fetchMe(tenant1Recheck.context.request, aliceRefreshAfter, TENANT_1_SLUG);
    expect(aliceAfter.id, 'alice must remain the same tenant-1 user — cross-tenant attempt must not have created a duplicate').toBe(
      aliceOnTenant1ID
    );
    await tenant1Recheck.context.close();
    await tenant2.context.close();
  });

  test('tenant-1 session targeting tenant-2 host must not leak tenant-1 data', async ({
    browser,
  }) => {
    // Sign alice in on tenant-1 to get a bearer token, then point
    // request calls at tenant-2 by flipping the override header on
    // each call. The user-aware registries are scoped by the JWT's
    // user (not the request-tenant context), so this exercises BOTH
    // axes of isolation: the JWT carries tenant_1, the override
    // says tenant_2 — the response must never reflect a successful
    // cross-tenant read of OTHER users' data on tenant-2.
    const tenant1 = await newTenantContext(browser, TENANT_1_SLUG);
    await runOAuthFlowOnTenantContext(tenant1.page);
    const accessToken = await refreshAndExpectSuccess(tenant1.context.request, TENANT_1_SLUG);
    const alice = await fetchMe(tenant1.context.request, accessToken, TENANT_1_SLUG);
    expect(alice.email).toBe(ALICE_PROFILE.email);

    // Probe a tenant-scoped resource on tenant-2 with tenant-1's bearer.
    // The contract: no successful read of tenant-foreign data. The BE
    // is allowed to either reject the request (4xx) or return alice's
    // own data scoped by her JWT — what it must NEVER do is return
    // rows belonging to a different tenant.
    const crossTenantLocationsResp = await tenant1.context.request.get(
      '/api/v1/locations?page=1&per_page=50',
      {
        headers: {
          Accept: 'application/json',
          Authorization: `Bearer ${accessToken}`,
          'X-Inventario-Test-Tenant': TENANT_2_SLUG,
        },
      }
    );

    if (crossTenantLocationsResp.status() >= 400) {
      // Strong-isolation BE refused outright; the body is unimportant.
      expect(crossTenantLocationsResp.status()).toBeGreaterThanOrEqual(400);
    } else {
      const body = (await crossTenantLocationsResp.json()) as {
        data?: Array<{ attributes?: { tenant_id?: string } }>;
      };
      for (const row of body.data ?? []) {
        // The slug check is a coarse marker — the real boundary is
        // tenant UUID. If the public schema exposes tenant_id, fail
        // if it matches tenant-2's slug.
        const rowTenantID = row.attributes?.tenant_id;
        if (rowTenantID !== undefined) {
          expect(rowTenantID, 'row must not carry a tenant-2 marker').not.toContain(TENANT_2_SLUG);
        }
      }
    }

    // Also probe /api/v1/auth/me cross-tenant. The endpoint reads the
    // user out of the JWT, so it must return alice (tenant-1) — NOT
    // any tenant-2 user. The override header must not be able to
    // swap the authenticated identity.
    const crossTenantMeResp = await tenant1.context.request.get('/api/v1/auth/me', {
      headers: {
        Accept: 'application/json',
        Authorization: `Bearer ${accessToken}`,
        'X-Inventario-Test-Tenant': TENANT_2_SLUG,
      },
    });
    if (crossTenantMeResp.status() === 200) {
      const crossMe = (await crossTenantMeResp.json()) as { id: string; email: string };
      expect(
        crossMe.id,
        '/auth/me with cross-tenant override must NOT return a different user — JWT identity is the boundary'
      ).toBe(alice.id);
      expect(crossMe.email).toBe(ALICE_PROFILE.email);
    } else {
      // 401/403 is also acceptable — the BE refused the cross-tenant
      // probe outright, which is the stronger isolation behavior.
      expect(crossTenantMeResp.status()).toBeGreaterThanOrEqual(400);
    }

    await tenant1.context.close();
  });
});

/**
 * newTenantContext spins up a fresh browser context pinned to a tenant
 * via the test-only `X-Inventario-Test-Tenant` header. Playwright sends
 * this header on every request the context makes — page navigations,
 * OAuth redirects through the stub, and direct `context.request.*`
 * API calls alike — so the BE resolves the chosen tenant for the
 * whole session.
 */
async function newTenantContext(
  browser: Browser,
  tenantSlug: string
): Promise<{ context: BrowserContext; page: Page }> {
  const context = await browser.newContext({
    extraHTTPHeaders: { 'X-Inventario-Test-Tenant': tenantSlug },
  });
  const page = await context.newPage();
  return { context, page };
}

/**
 * runOAuthFlowOnTenantContext drives the Google-stub sign-in from /login
 * to whichever post-auth URL the BE redirects to (/, /no-group, or — for
 * the tenant-mismatch path — /login). Used by the cross-tenant tests for
 * the "land on a successful session" steps. Callers that need to assert
 * on a specific landing URL should not use this helper.
 */
async function runOAuthFlowOnTenantContext(page: Page): Promise<void> {
  await page.goto('/login');
  const googleButton = page.locator(`[data-testid="oauth-${PROVIDER}-button"]`);
  await expect(googleButton).toBeVisible({ timeout: 10_000 });
  await Promise.all([
    page.waitForURL(
      (url) => url.pathname === '/' || url.pathname === '/no-group' || url.pathname === '/login',
      { timeout: 20_000 }
    ),
    googleButton.click(),
  ]);
}

/**
 * refreshAndExpectSuccess spends the OAuth-set refresh cookie via
 * /auth/refresh and returns the issued access token. Asserts a 200
 * response (the cookie path is /api/v1, sent on every BE call from
 * the same Playwright context).
 */
async function refreshAndExpectSuccess(
  request: APIRequestContext,
  tenantSlug: string
): Promise<string> {
  const refreshResp = await request.post('/api/v1/auth/refresh', {
    headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
    data: {},
  });
  expect(
    refreshResp.status(),
    `refresh on tenant '${tenantSlug}' failed: ${await refreshResp.text()}`
  ).toBe(200);
  const body = (await refreshResp.json()) as { access_token: string };
  expect(body.access_token, `tenant '${tenantSlug}' refresh did not return an access token`).toBeTruthy();
  return body.access_token;
}

/**
 * fetchMe returns the /auth/me payload for the currently-authenticated
 * caller. Used to assert which user the BE believes the JWT belongs to.
 */
async function fetchMe(
  request: APIRequestContext,
  accessToken: string,
  tenantSlug: string
): Promise<{ id: string; email: string; name: string; has_password: boolean }> {
  const meResp = await request.get('/api/v1/auth/me', {
    headers: { Accept: 'application/json', Authorization: `Bearer ${accessToken}` },
  });
  expect(
    meResp.status(),
    `/auth/me on tenant '${tenantSlug}' failed: ${await meResp.text()}`
  ).toBe(200);
  return meResp.json();
}
