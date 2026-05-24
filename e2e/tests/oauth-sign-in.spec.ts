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
import { test, expect } from '@playwright/test';

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
