import { Page, expect } from '@playwright/test';
import { TestRecorder, log, warn, error } from '../../utils/test-recorder.js';
import { setCsrfToken } from './csrf.js';
import { ensureGroupSlug, gotoScoped } from './group-url.js';

/**
 * Recover from a transient /no-group landing for a user that actually
 * has a group. Webkit can lose the post-login race between
 * /api/v1/auth/me and /api/v1/groups (parallel React-Query fetches),
 * leaving RootRedirect with `user?.default_group_id` undefined and
 * navigating to /no-group on first paint. Reloading via
 * `page.goto('/')` can hit the same race a second time, so instead we
 * resolve the slug authoritatively from `/api/v1/groups` (which
 * `ensureGroupSlug` does inside the page context with the JWT
 * already in localStorage) and navigate to `/g/<slug>` directly,
 * bypassing RootRedirect altogether.
 *
 * Returns true when the recovery moved the page off /no-group; false
 * if the slug couldn't be resolved (genuinely groupless user).
 */
async function recoverFromNoGroupRace(page: Page, recorder?: TestRecorder): Promise<boolean> {
  if (!page.url().includes('/no-group')) return true;
  log(recorder, '🔄 Detected /no-group landing — bypassing RootRedirect via direct /g/<slug> navigation...');
  try {
    const slug = await ensureGroupSlug(page);
    await page.goto(`/g/${encodeURIComponent(slug)}`);
    return !page.url().includes('/no-group');
  } catch (err) {
    warn(recorder, '⚠️ Could not resolve a group slug — leaving the page on /no-group:', err);
    return false;
  }
}

/**
 * Test credentials for e2e tests
 * These match the test users created when seeding without parameters (e2e test environment)
 */
export const TEST_CREDENTIALS = {
  email: 'admin@test-org.com',
  password: 'TestPassword123'
};

/**
 * Zero-group test user credentials. Seeded by debug/seeddata as an active
 * user with no group memberships so e2e tests can authenticate against the
 * real `/api/v1/groups` empty-collection response without intercepting it.
 * See issue #1277 — admin can't be used because the last-admin invariant
 * blocks `POST /groups/{id}/leave`.
 */
export const ORPHAN_TEST_CREDENTIALS = {
  email: 'orphan@test-org.com',
  password: 'TestPassword123'
};

/**
 * Platform system-admin credentials. Seeded by debug/seeddata
 * (`sysadmin@test-org.com`, IsSystemAdmin = true) so the admin-section
 * e2e suite can reach `/api/v1/admin/*` and the `/admin/*` UI without a
 * separate `inventario admin grant-system-admin` CLI step. See #1758.
 */
export const SYSADMIN_TEST_CREDENTIALS = {
  email: 'sysadmin@test-org.com',
  password: 'TestPassword123'
};

/**
 * Disposable plain (non-admin) user the block/unblock spec deactivates
 * then reactivates. Seeded by debug/seeddata as `blocktarget@test-org.com`
 * and referenced by no other spec, so a parallel run never observes it
 * mid-block. See #1758.
 */
export const BLOCK_TARGET_TEST_CREDENTIALS = {
  email: 'blocktarget@test-org.com',
  password: 'TestPassword123'
};

/**
 * Check if the current page is the login page
 */
export async function isLoginPage(page: Page): Promise<boolean> {
  try {
    // Check URL first
    const currentUrl = page.url();
    if (currentUrl.includes('/login')) {
      return true;
    }

    // Check for login form elements
    const loginForm = page.locator('form').filter({ hasText: 'Login' });
    const emailField = page.locator('input[type="email"]');
    const passwordField = page.locator('input[type="password"]');

    const hasLoginForm = await loginForm.isVisible();
    const hasEmailField = await emailField.isVisible();
    const hasPasswordField = await passwordField.isVisible();

    return hasLoginForm && hasEmailField && hasPasswordField;
  } catch (error) {
    return false;
  }
}

/**
 * Check if the user is currently authenticated.
 * Uses [data-testid="user-menu"] visibility as the primary indicator. After
 * cutover #1423 the testid lives on the SidebarMenuButton inside the
 * authenticated React shell (`AppSidebar.tsx`). The shell only mounts under
 * guarded routes, so the testid reliably reflects auth state. Nav elements
 * (Locations, Commodities) are always rendered when the shell is up
 * regardless of route, so they must NOT be used as an auth indicator.
 */
export async function isAuthenticated(page: Page, recorder?: TestRecorder): Promise<boolean> {
  try {
    // Wait for the React router + TanStack Query initial fetch to settle so
    // the shell has had a chance to mount or redirect us back to /login.
    await page.waitForLoadState('networkidle', { timeout: 5000 });

    // The user-menu testid is only present inside the authenticated Shell
    // (AppSidebar's bottom-of-sidebar account button), making it a
    // race-condition-free auth indicator.
    const hasUserMenu = await page.locator('[data-testid="user-menu"]').isVisible({ timeout: 3000 });
    return hasUserMenu;
  } catch (err) {
    warn(recorder, 'Authentication check error:', err);
    return false;
  }
}

/**
 * Perform login with test credentials and extract CSRF token.
 * Defaults to the seeded admin credentials; pass `credentials` to log in as
 * a different seeded user (e.g. ORPHAN_TEST_CREDENTIALS for #1277 tests).
 */
export async function login(
  page: Page,
  recorder?: TestRecorder,
  credentials: { email: string; password: string } = TEST_CREDENTIALS,
): Promise<string | null> {
  log(recorder, `🔐 Performing login as ${credentials.email}...`);

  // Wait for login form to be visible. Generous timeout because the
  // login route lazy-loads under heavy parallel load — when many
  // workers hit the same vite dev server simultaneously the bundle
  // fetch + initial render can take 15-20s on a busy laptop.
  await page.waitForSelector('input[type="email"]', { timeout: 30000 });

  // Fill in credentials
  await page.fill('input[type="email"]', credentials.email);
  await page.fill('input[type="password"]', credentials.password);
  // Wait for login API response and fail fast on non-200 statuses.
  const loginResponsePromise = page.waitForResponse(
    (response) => response.url().includes('/api/v1/auth/login'),
    { timeout: 20000 }
  );

  // Submit the form
  await page.click('button[type="submit"]');
  const loginResponse = await loginResponsePromise;
  if (loginResponse.status() !== 200) {
    let responseText = '';
    try {
      responseText = await loginResponse.text();
    } catch {
      // no-op
    }
    error(
      recorder,
      `❌ Login failed: status=${loginResponse.status()} body=${responseText.slice(0, 500)}`
    );
    throw new Error(`Login failed with status ${loginResponse.status()}`);
  }

  // Extract CSRF token directly from successful login response.
  let csrfToken: string | null = null;
  try {
    const data = await loginResponse.json();
    if (data?.csrf_token) {
      csrfToken = data.csrf_token;
      setCsrfToken(csrfToken);
      log(recorder, '🔑 CSRF token extracted from login response');
    }
  } catch {
    warn(recorder, '⚠️ Login succeeded but CSRF token parsing failed');
  }

  // Wait for the post-login navigation to leave /login. The previous
  // version OR'd this with `h1 contains "Welcome to Inventario"`, but
  // that string is the /no-group page heading (auth.json:noGroup.title)
  // — not a login signal — so the wait would short-circuit the moment
  // a webkit race bounced us through /no-group, hiding the bug from the
  // helper while the test itself fails on the next h1 assertion.
  await page.waitForFunction(
    () => !window.location.pathname.startsWith('/login'),
    { timeout: 30000 }
  );

  // Give UI a brief moment to settle after redirect/auth state propagation.
  await page.waitForTimeout(500);

  // Webkit auth-state race recovery — see recoverFromNoGroupRace for
  // background. Gated by credentials so the orphan fixture
  // (`ORPHAN_TEST_CREDENTIALS`), which legitimately lands on /no-group,
  // skips the API probe entirely.
  if (credentials.email !== ORPHAN_TEST_CREDENTIALS.email) {
    await recoverFromNoGroupRace(page, recorder);
  }

  // If we're still on login page but the page has rendered, manually
  // navigate to home (defensive — should not happen given the
  // waitForFunction above, but keeps parity with prior behaviour).
  if (page.url().includes('/login')) {
    log(recorder, '🔄 Still on /login after auth completed, navigating home...');
    await page.goto('/');
  }

  log(recorder, '✅ Login completed successfully');

  return csrfToken;
}

/**
 * Ensure the user is authenticated, login if necessary
 * Returns the CSRF token if login was performed
 */
export async function ensureAuthenticated(page: Page, recorder?: TestRecorder): Promise<string | null> {
  // Wait for any ongoing authentication initialization
  await page.waitForTimeout(500);

  // First check if we're already authenticated
  if (await isAuthenticated(page, recorder)) {
    log(recorder, '✅ Already authenticated');
    return null;
  }

  // Check if we're on the login page
  if (await isLoginPage(page)) {
    log(recorder, '🔐 On login page, performing login...');
    return await login(page, recorder);
  }

  // If we're neither authenticated nor on login page, navigate to login
  log(recorder, '🔄 Navigating to login page...');
  await page.goto('/login');
  return await login(page, recorder);
}

/**
 * Login if needed before accessing a protected page
 * This function handles the common case where navigating to a protected page
 * redirects to login, and we need to authenticate first
 * Returns the CSRF token if login was performed
 */
export async function loginIfNeeded(page: Page, targetUrl?: string, recorder?: TestRecorder): Promise<string | null> {
  let csrfToken: string | null = null;

  // Probe the app via "/" so the router decides whether we land on
  // /login (unauthed) or on /g/<slug>/... (authed). We can't hit a flat
  // data path like /locations directly anymore — after #1321 the legacy
  // stubs are gone, so a raw goto("/locations") would just land on the
  // 404 route. Once the probe settles we either log in first, or jump
  // straight to the scoped target via gotoScoped.
  if (targetUrl) {
    await page.goto('/');
    await page.waitForTimeout(1000);
  }

  // Check if we ended up on the login page (due to redirect)
  if (await isLoginPage(page)) {
    log(recorder, '🔄 Redirected to login, authenticating...');
    csrfToken = await login(page, recorder);
  }

  // Webkit auth-state race recovery (see recoverFromNoGroupRace). The
  // probe `page.goto('/')` above can hit the race on its own — even
  // when login already happened in app-fixture.ts and the JWT is
  // already in localStorage — because RootRedirect re-renders cold on
  // every full reload. The orphan-user specs (no-group-redirect.spec.ts,
  // settings-default-group.spec.ts) never reach this code path — they
  // `page.goto` directly without going through navigateWithAuth — so
  // triggering recovery on /no-group here is safe.
  if (targetUrl === '/') {
    await recoverFromNoGroupRace(page, recorder);
  }

  // Navigate to the real target now that auth is settled. gotoScoped
  // rewrites flat data paths (/locations, /files, …) into their
  // /g/<slug>/... scoped equivalents so data routes resolve after #1321.
  if (targetUrl && targetUrl !== '/') {
    log(recorder, `🔄 Navigating to target URL: ${targetUrl}`);
    await gotoScoped(page, targetUrl);
  }

  // Verify we're now authenticated with retries
  let authVerified = false;
  for (let i = 0; i < 3; i++) {
    await page.waitForTimeout(1000); // Wait a moment for page to settle
    authVerified = await isAuthenticated(page, recorder);
    if (authVerified) break;
    log(recorder, `⏳ Authentication verification attempt ${i + 1}/3...`);
  }

  if (!authVerified) {
    error(recorder, '❌ Authentication verification failed');
    error(recorder, `Current URL: ${page.url()}`);
    error(recorder, `Page title: ${await page.title()}`);
    throw new Error('Failed to authenticate - login did not complete successfully');
  }

  return csrfToken;
}

/**
 * Logout the current user
 */
export async function logout(page: Page, recorder?: TestRecorder): Promise<void> {
  try {
    // Click the sidebar account button to open the dropdown menu, then the
    // Sign out item. After cutover #1423 both surfaces carry data-testid
    // handles (`user-menu`, `sign-out`); the legacy text-based fallbacks
    // remain in case a non-canonical surface (e.g. Settings page) renders
    // a "Logout" button later.
    const userMenu = page.locator('[data-testid="user-menu"]').or(page.locator('button:has-text("Logout")'));

    if (await userMenu.isVisible()) {
      await userMenu.click();

      const logoutButton = page
        .locator('[data-testid="sign-out"]')
        .or(page.locator('button:has-text("Sign out")'))
        .or(page.locator('button:has-text("Logout")'))
        .or(page.locator('a:has-text("Logout")'));
      if (await logoutButton.isVisible()) {
        await logoutButton.click();
      }
    }

    // Wait for logout to complete
    await page.waitForFunction(
      () => window.location.pathname === '/login',
      { timeout: 5000 }
    );

    log(recorder, '✅ Logout completed successfully');
  } catch (err) {
    warn(recorder, '⚠️ Logout failed or not needed:', err);
  }
}

/**
 * Navigate to a URL with authentication handling
 * This is a replacement for page.goto() that handles authentication
 * Returns the CSRF token if login was performed
 */
export async function navigateWithAuth(page: Page, url: string, recorder?: TestRecorder): Promise<string | null> {
  log(recorder, `🔄 Navigating to ${url} with authentication handling...`);

  const csrfToken = await loginIfNeeded(page, url, recorder);

  // Ensure we're on the correct page. Use gotoScoped so a flat data path
  // like "/locations" gets rewritten to "/g/<slug>/locations" — after
  // #1321 the legacy flat stubs are gone and a raw page.goto("/locations")
  // would land on the 404 route.
  const currentUrl = page.url();
  if (!currentUrl.includes(url.replace('/', ''))) {
    await gotoScoped(page, url);
  }

  log(recorder, `✅ Successfully navigated to ${url}`);

  return csrfToken;
}

/**
 * Get the CSRF token from the page context
 * This retrieves the token stored in the frontend's memory
 */
export async function getCsrfToken(page: Page): Promise<string | null> {
  try {
    const token = await page.evaluate(() => {
      // Access the CSRF token from the frontend's api.ts module
      // This assumes the token is stored in a global or accessible location
      return (window as any).__csrfToken || null;
    });
    return token;
  } catch (err) {
    return null;
  }
}
