/**
 * E2E coverage for the system-wide admin section (umbrella #1744,
 * QA gate #1758).
 *
 * The system admin is bootstrapped by the test harness: debug/seeddata
 * provisions `sysadmin@test-org.com` with `is_system_admin = true`
 * (mirroring the production `inventario admin grant-system-admin` CLI
 * step) so every harness lane gets the fixture without a bespoke CLI
 * call — see SYSADMIN_TEST_CREDENTIALS in includes/auth.ts.
 *
 * Spec layout follows the issue #1758 E2E checklist:
 *   1. browse tenants → tenant detail → users + groups tabs
 *   2. non-admins are denied the admin surface (UI 403 + API 403)
 *   3. block invalidates the target's live access token; unblock restores it
 *   4. admin group membership add / role-change / remove + soft-delete
 *   5. impersonation: start → banner → navigate → end → admin restored
 *   6. impersonation safety: no nested impersonation, no token refresh
 *
 * Cross-tenant rejection (`admin.member.tenant_mismatch`) is not
 * reachable here — the e2e database holds a single tenant — and is
 * covered by the backend integration tests for #1749.
 *
 * Note: Playwright's testDir is `./tests`, so this spec lives at
 * `tests/admin/` rather than the `e2e/specs/admin/` path named in the
 * issue, which would not be discovered by the runner.
 */
import { test, expect, type APIRequestContext, type Page } from '@playwright/test';
import waitOn from 'wait-on';
import {
  login,
  SYSADMIN_TEST_CREDENTIALS,
  BLOCK_TARGET_TEST_CREDENTIALS,
  ORPHAN_TEST_CREDENTIALS,
  TEST_CREDENTIALS,
} from '../includes/auth.js';
import { BASE_URL } from '../../setup/urls.js';

const TEAMMATE_CREDENTIALS = {
  email: 'teammate@test-org.com',
  password: 'TestPassword123',
};

const JSON_API = 'application/vnd.api+json';

type ApiSession = { token: string; csrf: string; userId: string };

/**
 * Log in directly against the JSON API (no browser). Returns the
 * access token, CSRF token and user id from the LoginResponse body.
 */
async function apiLogin(
  request: APIRequestContext,
  credentials: { email: string; password: string },
): Promise<ApiSession> {
  const resp = await request.post('/api/v1/auth/login', {
    headers: { 'Content-Type': 'application/json' },
    data: credentials,
  });
  expect(resp.status(), `login ${credentials.email}`).toBe(200);
  const body = await resp.json();
  expect(body.access_token, `access_token for ${credentials.email}`).toBeTruthy();
  expect(body.user?.id, `user id for ${credentials.email}`).toBeTruthy();
  // CSRF is required for every mutating admin endpoint this spec hits.
  // Fail fast here with a clear message rather than letting a missing
  // token surface as a confusing 403 far downstream.
  expect(body.csrf_token, `csrf_token for ${credentials.email}`).toBeTruthy();
  return { token: body.access_token, csrf: body.csrf_token, userId: body.user.id };
}

/** Log in as the seeded system admin through the real login form. */
async function loginAsSysadmin(page: Page): Promise<void> {
  await page.goto('/login');
  await login(page, undefined, SYSADMIN_TEST_CREDENTIALS);
}

/** Read the access + CSRF tokens the frontend stashed after login. */
async function pageTokens(page: Page): Promise<{ token: string; csrf: string }> {
  return page.evaluate(() => ({
    token: localStorage.getItem('inventario_token') || '',
    csrf: sessionStorage.getItem('inventario_csrf_token') || '',
  }));
}

function authHeaders(token: string, csrf?: string): Record<string, string> {
  const h: Record<string, string> = { Authorization: `Bearer ${token}` };
  if (csrf) h['X-CSRF-Token'] = csrf;
  return h;
}

test.describe('System admin section (#1744 / #1758)', () => {
  test.beforeAll(async () => {
    // global-setup waits for the stack, but sibling specs may have
    // bounced services in between — re-probe so the first navigation
    // doesn't race a still-warming server.
    await waitOn({
      resources: [BASE_URL],
      timeout: 15000,
      interval: 250,
      window: 1000,
      tcpTimeout: 1000,
    });
  });

  test('browses tenants, tenant detail, users and groups tabs', async ({ page }) => {
    await loginAsSysadmin(page);

    // The admin sidebar entry is only rendered for system admins.
    await expect(page.getByTestId('sidebar-admin-group')).toBeVisible();

    await page.goto('/admin/tenants');
    await expect(page.getByTestId('admin-tenants-page')).toBeVisible();

    const tenantRows = page.getByTestId('admin-tenant-row');
    await expect(tenantRows.first()).toBeVisible();

    // Into the tenant detail page.
    await tenantRows.first().click();
    await expect(page.getByTestId('admin-tenant-detail-page')).toBeVisible();
    await expect(page).toHaveURL(/\/admin\/tenants\/[^/]+$/);

    // Users tab — the seeded tenant has the well-known fixture users.
    await page.getByTestId('admin-tenant-tab-users').click();
    await expect(page.getByTestId('admin-tenant-users-table')).toBeVisible();
    await expect(page.getByTestId('admin-tenant-user-row').first()).toBeVisible();

    // Groups tab.
    await page.getByTestId('admin-tenant-tab-groups').click();
    await expect(page.getByTestId('admin-tenant-groups-table')).toBeVisible();
    await expect(page.getByTestId('admin-tenant-group-row').first()).toBeVisible();
  });

  test('denies the admin surface to non-admin users (UI + API 403)', async ({ page, request }) => {
    // A regular tenant user — not a system admin.
    await page.goto('/login');
    await login(page, undefined, TEST_CREDENTIALS);

    // No admin sidebar entry.
    await expect(page.getByTestId('sidebar-admin-group')).toHaveCount(0);

    // Deep-linking into /admin renders the in-place 403, not the data.
    await page.goto('/admin/tenants');
    await expect(page.getByTestId('admin-forbidden')).toBeVisible();
    await expect(page.getByTestId('admin-tenants-page')).toHaveCount(0);

    // The API rejects the non-admin token outright.
    const { token } = await pageTokens(page);
    expect(token).toBeTruthy();
    const resp = await request.get('/api/v1/admin/tenants', {
      headers: authHeaders(token),
    });
    expect(resp.status()).toBe(403);
  });

  test('blocking a user invalidates their live token; unblock restores access', async ({
    page,
    request,
  }) => {
    // Capture a token for the block target BEFORE it is blocked.
    const target = await apiLogin(request, BLOCK_TARGET_TEST_CREDENTIALS);

    // Sanity: the captured token is live right now.
    const before = await request.get('/api/v1/auth/me', {
      headers: authHeaders(target.token),
    });
    expect(before.status()).toBe(200);

    await loginAsSysadmin(page);
    await page.goto(`/admin/users/${target.userId}`);
    await expect(page.getByTestId('admin-user-detail-page')).toBeVisible();

    try {
      // Block via the user-detail UI.
      await page.getByTestId('admin-user-block').click();
      await expect(page.getByTestId('admin-user-action-dialog')).toBeVisible();
      await page.getByTestId('admin-user-action-reason').fill('e2e block/unblock coverage (#1758)');
      const blockResp = page.waitForResponse(
        (r) => r.url().includes(`/users/${target.userId}/block`) && r.request().method() === 'POST',
      );
      await page.getByTestId('admin-user-action-confirm').click();
      expect((await blockResp).status()).toBe(200);

      // The UI now offers "unblock", confirming the state flipped.
      await expect(page.getByTestId('admin-user-unblock')).toBeVisible();

      // The token issued before the block is now rejected — block bumps
      // the JWT-blacklist iat-staleness threshold for the user.
      const after = await request.get('/api/v1/auth/me', {
        headers: authHeaders(target.token),
      });
      expect(after.status()).toBe(401);
    } finally {
      // Unblock restores the account — always runs even if assertions fail.
      const { token, csrf } = await pageTokens(page);
      const unblockResp = await request.post(`/api/v1/admin/users/${target.userId}/unblock`, {
        headers: { 'Content-Type': 'application/json', ...authHeaders(token, csrf) },
        data: { reason: 'e2e cleanup (#1758)' },
      });
      expect(unblockResp.status()).toBe(200);
    }

    // A fresh login succeeds again now that the account is active.
    const relogin = await request.post('/api/v1/auth/login', {
      headers: { 'Content-Type': 'application/json' },
      data: BLOCK_TARGET_TEST_CREDENTIALS,
    });
    expect(relogin.status()).toBe(200);
  });

  test('admin edits group membership and soft-deletes the group', async ({ page, request }) => {
    await loginAsSysadmin(page);
    const { token, csrf } = await pageTokens(page);
    expect(token).toBeTruthy();

    // A throwaway group owned by the system admin. Soft-deleting it at
    // the end of the test is its own cleanup — the purge worker
    // finishes the job — so it never leaks into later runs.
    const groupName = `Admin QA Group ${Date.now()}`;
    const createResp = await request.post('/api/v1/groups', {
      headers: { 'Content-Type': JSON_API, Accept: JSON_API, ...authHeaders(token, csrf) },
      data: { data: { type: 'groups', attributes: { name: groupName, icon: '🧪' } } },
    });
    expect(createResp.status()).toBe(201);
    const groupId = (await createResp.json()).data.id as string;

    // A second user to manage as a member.
    const member = await apiLogin(request, TEAMMATE_CREDENTIALS);

    // Add the member (viewer) via the admin membership endpoint.
    const addResp = await request.post(`/api/v1/admin/groups/${groupId}/members`, {
      headers: { 'Content-Type': 'application/json', ...authHeaders(token, csrf) },
      data: { userID: member.userId, role: 'viewer' },
    });
    expect(addResp.status()).toBe(201);

    // The membership editor renders the new member.
    await page.goto(`/admin/groups/${groupId}`);
    await expect(page.getByTestId('admin-group-detail-page')).toBeVisible();
    await expect(page.getByTestId('admin-group-member-row')).toHaveCount(2);

    // Role change: viewer → user.
    const roleResp = await request.patch(
      `/api/v1/admin/groups/${groupId}/members/${member.userId}`,
      {
        headers: { 'Content-Type': 'application/json', ...authHeaders(token, csrf) },
        data: { role: 'user' },
      },
    );
    expect(roleResp.status()).toBe(200);

    // Remove the member.
    const removeResp = await request.delete(
      `/api/v1/admin/groups/${groupId}/members/${member.userId}`,
      { headers: authHeaders(token, csrf) },
    );
    expect(removeResp.status()).toBe(204);

    await page.reload();
    await expect(page.getByTestId('admin-group-member-row')).toHaveCount(1);

    // Soft-delete the group: status → pending_deletion.
    const deleteResp = await request.delete(`/api/v1/admin/groups/${groupId}`, {
      headers: authHeaders(token, csrf),
    });
    expect([200, 202, 204]).toContain(deleteResp.status());

    // The detail page shows the pending-deletion banner.
    await page.goto(`/admin/groups/${groupId}`);
    await expect(page.getByTestId('admin-group-pending-banner')).toBeVisible();
  });

  test('impersonation: start, banner, navigate, end, admin restored', async ({ page, request }) => {
    // The orphan fixture is a safe impersonation target — it is a
    // non-admin, active user and impersonating it does not mutate any
    // state a sibling spec depends on.
    const orphan = await apiLogin(request, ORPHAN_TEST_CREDENTIALS);

    await loginAsSysadmin(page);
    await page.goto(`/admin/users/${orphan.userId}`);
    await expect(page.getByTestId('admin-user-detail-page')).toBeVisible();

    // Start impersonation. The frontend hard-reloads the app on
    // success, so anchor on the POST response before the reload.
    const startResp = page.waitForResponse(
      (r) => r.url().includes(`/users/${orphan.userId}/impersonate`) && r.request().method() === 'POST',
    );
    await page.getByTestId('admin-user-impersonate').click();
    await expect(page.getByTestId('admin-user-action-dialog')).toBeVisible();
    await page.getByTestId('admin-user-action-confirm').click();
    expect((await startResp).ok()).toBeTruthy();

    // The persistent impersonation banner appears.
    await expect(page.getByTestId('impersonation-banner')).toBeVisible({ timeout: 20000 });

    // The banner survives an in-app navigation.
    await page.goto('/profile');
    await expect(page.getByTestId('impersonation-banner')).toBeVisible();

    // End impersonation → the admin session is restored.
    const endResp = page.waitForResponse(
      (r) => r.url().includes('/impersonation/end') && r.request().method() === 'POST',
    );
    await page.getByTestId('impersonation-end').click();
    expect((await endResp).ok()).toBeTruthy();

    await expect(page.getByTestId('impersonation-banner')).toBeHidden({ timeout: 20000 });
    // Admin privileges are back: the admin sidebar entry is visible again.
    await expect(page.getByTestId('sidebar-admin-group')).toBeVisible({ timeout: 20000 });
  });

  test('impersonation safety: no nested impersonation, no token refresh', async ({ request }) => {
    const orphan = await apiLogin(request, ORPHAN_TEST_CREDENTIALS);
    const sysadmin = await apiLogin(request, SYSADMIN_TEST_CREDENTIALS);

    // Start an impersonation session for the orphan user.
    const startResp = await request.post(`/api/v1/admin/users/${orphan.userId}/impersonate`, {
      headers: { 'Content-Type': 'application/json', ...authHeaders(sysadmin.token, sysadmin.csrf) },
      data: { reason: 'e2e impersonation safety check (#1758)' },
    });
    expect(startResp.ok()).toBeTruthy();
    const startBody = await startResp.json();
    const impToken = startBody.access_token as string;
    const impCsrf = (startBody.csrf_token as string) ?? '';
    expect(impToken).toBeTruthy();

    // No chain: an impersonation session cannot start a nested one.
    // Rejected either by RequireSystemAdmin (403 — the impersonation
    // token carries is_system_admin=false) or the no-chain handler
    // guard (422 admin.impersonate.nested).
    const nested = await request.post(`/api/v1/admin/users/${orphan.userId}/impersonate`, {
      headers: { 'Content-Type': 'application/json', ...authHeaders(impToken, impCsrf) },
      data: { reason: 'nested attempt' },
    });
    expect(nested.ok(), 'nested impersonation must be rejected').toBeFalsy();
    expect([403, 422]).toContain(nested.status());

    // No refresh: the impersonation token cannot mint a fresh access
    // token via the refresh endpoint.
    const refreshed = await request.post('/api/v1/auth/refresh', {
      headers: authHeaders(impToken, impCsrf),
    });
    expect(refreshed.ok(), 'impersonation token must not refresh').toBeFalsy();
    expect([401, 403]).toContain(refreshed.status());

    // Clean up: end the impersonation session.
    const end = await request.post('/api/v1/admin/impersonation/end', {
      headers: authHeaders(impToken, impCsrf),
    });
    expect(end.ok()).toBeTruthy();
  });
});
