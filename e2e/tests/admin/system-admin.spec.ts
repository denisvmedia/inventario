/**
 * E2E coverage for the back-office (platform-operator) surface
 * (umbrella #1744, QA gate #1758, rewritten for the back-office plane
 * in #2100).
 *
 * The operators are bootstrapped by the test harness: debug/seeddata
 * provisions `operator@backoffice.test` (platform_admin) and
 * `support@backoffice.test` (support_agent) with MFA disabled, mirroring
 * the production `inventario backoffice bootstrap` CLI step. See
 * SEED fixtures in includes/backoffice.ts.
 *
 * Spec layout:
 *   1. browse tenants → tenant detail → users + groups tabs
 *   2. a tenant session is denied the admin surface (UI bounce + API 401)
 *   3. block invalidates the target's live access token; unblock restores it
 *   4. admin group membership add / role-change / remove + soft-delete
 *   5. impersonation: start → banner → navigate → end → operator restored
 *   6. impersonation safety: no nested impersonation, no token refresh
 *   7. impersonation safety: a system admin cannot be impersonated
 *   8. impersonation safety: support_agent may read but may not impersonate
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
import {
  BACKOFFICE_SUPPORT_CREDENTIALS,
  bearer,
  loginAsOperator,
  operatorApiLogin,
  operatorPageToken,
} from '../includes/backoffice.js';
import { BASE_URL } from '../../setup/urls.js';

const TEAMMATE_CREDENTIALS = {
  email: 'teammate@test-org.com',
  password: 'TestPassword123',
};

const JSON_API = 'application/vnd.api+json';

type ApiSession = { token: string; csrf: string; userId: string };

/**
 * Log in directly against the TENANT JSON API (no browser). Returns the
 * access token, CSRF token and user id from the LoginResponse body.
 * Impersonation targets and the group fixture live on the tenant plane;
 * the operator side goes through operatorApiLogin instead.
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
  // CSRF is required for every mutating TENANT endpoint this spec hits
  // (the back-office plane is bearer-only). Fail fast here with a clear
  // message rather than letting a missing token surface as a confusing
  // 403 far downstream.
  expect(body.csrf_token, `csrf_token for ${credentials.email}`).toBeTruthy();
  return { token: body.access_token, csrf: body.csrf_token, userId: body.user.id };
}

function tenantHeaders(session: ApiSession): Record<string, string> {
  return { ...bearer(session.token), 'X-CSRF-Token': session.csrf };
}

test.describe('Back-office admin section (#1744 / #1758 / #2100)', () => {
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
    await loginAsOperator(page);

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

  test('denies the admin surface to a tenant session (UI bounce + API 401)', async ({
    page,
    request,
  }) => {
    // A regular tenant user. Since #1785 the admin surface is a separate
    // auth plane, so even a tenant system admin would be refused here —
    // holding a tenant token is the disqualifier, not the role.
    await page.goto('/login');
    await login(page, undefined, TEST_CREDENTIALS);

    // Deep-linking into /admin bounces to the back-office login, not to
    // the admin data.
    await page.goto('/admin/tenants');
    await expect(page.getByTestId('backoffice-login-page')).toBeVisible();
    await expect(page).toHaveURL(/\/backoffice\/login\?.*reason=auth_required/);
    await expect(page.getByTestId('admin-tenants-page')).toHaveCount(0);

    // The API rejects the tenant token outright: RequireBackofficeAuth
    // refuses anything whose `aud` is not `backoffice`.
    const token = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    expect(token).toBeTruthy();
    const resp = await request.get('/api/v1/admin/tenants', { headers: bearer(token) });
    expect(resp.status()).toBe(401);
  });

  test('blocking a user invalidates their live token; unblock restores access', async ({
    page,
    request,
  }) => {
    // Capture a token for the block target BEFORE it is blocked.
    const target = await apiLogin(request, BLOCK_TARGET_TEST_CREDENTIALS);

    // Sanity: the captured token is live right now.
    const before = await request.get('/api/v1/auth/me', { headers: bearer(target.token) });
    expect(before.status()).toBe(200);

    await loginAsOperator(page);
    await page.goto(`/admin/users/${target.userId}`);
    await expect(page.getByTestId('admin-user-detail-page')).toBeVisible();

    try {
      // Block via the user-detail UI.
      await page.getByTestId('admin-user-block').click();
      await expect(page.getByTestId('admin-user-action-dialog')).toBeVisible();
      await page.getByTestId('admin-user-action-reason').fill('e2e block/unblock coverage (#2100)');
      const blockResp = page.waitForResponse(
        (r) => r.url().includes(`/users/${target.userId}/block`) && r.request().method() === 'POST',
      );
      await page.getByTestId('admin-user-action-confirm').click();
      expect((await blockResp).status()).toBe(200);

      // The UI now offers "unblock", confirming the state flipped.
      await expect(page.getByTestId('admin-user-unblock')).toBeVisible();

      // The token issued before the block is now rejected — block bumps
      // the JWT-blacklist iat-staleness threshold for the user.
      const after = await request.get('/api/v1/auth/me', { headers: bearer(target.token) });
      expect(after.status()).toBe(401);
    } finally {
      // Unblock restores the account — always runs even if assertions fail.
      const operatorToken = await operatorPageToken(page);
      const unblockResp = await request.post(`/api/v1/admin/users/${target.userId}/unblock`, {
        headers: { 'Content-Type': 'application/json', ...bearer(operatorToken) },
        data: { reason: 'e2e cleanup (#2100)' },
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
    const operator = await operatorApiLogin(request);

    // A throwaway group. Groups belong to the tenant plane, so the
    // operator (who has no tenant identity at all since #1785) cannot
    // create one — the seeded tenant admin does. Soft-deleting it at the
    // end of the test is its own cleanup: the purge worker finishes the
    // job, so it never leaks into later runs.
    const owner = await apiLogin(request, TEST_CREDENTIALS);
    const groupName = `Admin QA Group ${Date.now()}`;
    const createResp = await request.post('/api/v1/groups', {
      headers: { 'Content-Type': JSON_API, Accept: JSON_API, ...tenantHeaders(owner) },
      data: { data: { type: 'groups', attributes: { name: groupName, icon: '🧪' } } },
    });
    expect(createResp.status()).toBe(201);
    const groupId = (await createResp.json()).data.id as string;

    // A second user to manage as a member.
    const member = await apiLogin(request, TEAMMATE_CREDENTIALS);

    // Add the member (viewer) via the admin membership endpoint.
    const addResp = await request.post(`/api/v1/admin/groups/${groupId}/members`, {
      headers: { 'Content-Type': 'application/json', ...bearer(operator.token) },
      data: { userID: member.userId, role: 'viewer' },
    });
    expect(addResp.status()).toBe(201);

    // The membership editor renders the new member.
    await loginAsOperator(page);
    await page.goto(`/admin/groups/${groupId}`);
    await expect(page.getByTestId('admin-group-detail-page')).toBeVisible();
    await expect(page.getByTestId('admin-group-member-row')).toHaveCount(2);

    // Role change: viewer → user.
    const roleResp = await request.patch(
      `/api/v1/admin/groups/${groupId}/members/${member.userId}`,
      {
        headers: { 'Content-Type': 'application/json', ...bearer(operator.token) },
        data: { role: 'user' },
      },
    );
    expect(roleResp.status()).toBe(200);

    // Remove the member.
    const removeResp = await request.delete(
      `/api/v1/admin/groups/${groupId}/members/${member.userId}`,
      { headers: bearer(operator.token) },
    );
    expect(removeResp.status()).toBe(204);

    await page.reload();
    await expect(page.getByTestId('admin-group-member-row')).toHaveCount(1);

    // Soft-delete the group: status → pending_deletion.
    const deleteResp = await request.delete(`/api/v1/admin/groups/${groupId}`, {
      headers: bearer(operator.token),
    });
    expect([200, 202, 204]).toContain(deleteResp.status());

    // The detail page shows the pending-deletion banner.
    await page.goto(`/admin/groups/${groupId}`);
    await expect(page.getByTestId('admin-group-pending-banner')).toBeVisible();
  });

  test('impersonation: start, banner, navigate, end, operator restored', async ({
    page,
    request,
  }) => {
    // The orphan fixture is a safe impersonation target — it is a
    // non-admin, active user and impersonating it does not mutate any
    // state a sibling spec depends on.
    const orphan = await apiLogin(request, ORPHAN_TEST_CREDENTIALS);

    await loginAsOperator(page);
    await page.goto(`/admin/users/${orphan.userId}`);
    await expect(page.getByTestId('admin-user-detail-page')).toBeVisible();

    // Start impersonation. The frontend hard-reloads the app on
    // success, so anchor on the POST response before the reload.
    const startResp = page.waitForResponse(
      (r) =>
        r.url().includes(`/users/${orphan.userId}/impersonate`) && r.request().method() === 'POST',
    );
    await page.getByTestId('admin-user-impersonate').click();
    await expect(page.getByTestId('admin-user-action-dialog')).toBeVisible();
    await page.getByTestId('admin-user-action-confirm').click();
    expect((await startResp).ok()).toBeTruthy();

    // The persistent impersonation banner appears. The browser is now on
    // the TENANT plane under the borrowed identity.
    await expect(page.getByTestId('impersonation-banner')).toBeVisible({ timeout: 20000 });

    // The banner survives an in-app navigation.
    await page.goto('/profile');
    await expect(page.getByTestId('impersonation-banner')).toBeVisible();

    // End impersonation → the operator's back-office session is restored
    // and the FE returns to the impersonated user's admin detail page.
    const endResp = page.waitForResponse(
      (r) => r.url().includes('/impersonation/end') && r.request().method() === 'POST',
    );
    await page.getByTestId('impersonation-end').click();
    expect((await endResp).ok()).toBeTruthy();

    await expect(page.getByTestId('impersonation-banner')).toBeHidden({ timeout: 20000 });
    // Back on the back-office plane: the operator chrome renders, which
    // only RequireBackofficeAuth-gated routes do.
    await expect(page.getByTestId('admin-shell-operator')).toBeVisible({ timeout: 20000 });
    await expect(page.getByTestId('admin-user-detail-page')).toBeVisible({ timeout: 20000 });
  });

  test('impersonation safety: no nested impersonation, no token refresh', async ({ request }) => {
    const orphan = await apiLogin(request, ORPHAN_TEST_CREDENTIALS);
    const operator = await operatorApiLogin(request);

    // Start an impersonation session for the orphan user.
    const startResp = await request.post(`/api/v1/admin/users/${orphan.userId}/impersonate`, {
      headers: { 'Content-Type': 'application/json', ...bearer(operator.token) },
      data: { reason: 'e2e impersonation safety check (#2100)' },
    });
    expect(startResp.ok()).toBeTruthy();
    const impToken = (await startResp.json()).access_token as string;
    expect(impToken).toBeTruthy();

    // No chain: an impersonation session cannot start a nested one. The
    // impersonation token is a TENANT JWT, so RequireBackofficeAuth
    // rejects it at the gate (401) well before the handler's own
    // defence-in-depth nested guard (422) can fire.
    const nested = await request.post(`/api/v1/admin/users/${orphan.userId}/impersonate`, {
      headers: { 'Content-Type': 'application/json', ...bearer(impToken) },
      data: { reason: 'nested attempt' },
    });
    expect(nested.ok(), 'nested impersonation must be rejected').toBeFalsy();
    expect([401, 403, 422]).toContain(nested.status());

    // No refresh: the impersonation token cannot mint a fresh access
    // token via the tenant refresh endpoint.
    const refreshed = await request.post('/api/v1/auth/refresh', {
      headers: bearer(impToken),
    });
    expect(refreshed.ok(), 'impersonation token must not refresh').toBeFalsy();
    expect([401, 403]).toContain(refreshed.status());

    // Clean up: end the impersonation session. `end` self-validates the
    // imp token off the Authorization header, so no cookie is needed.
    const end = await request.post('/api/v1/admin/impersonation/end', {
      headers: bearer(impToken),
    });
    expect(end.ok()).toBeTruthy();
  });

  test('impersonation safety: a system admin cannot be impersonated', async ({ request }) => {
    const operator = await operatorApiLogin(request);
    // The tenant-side system-admin grant (#1784) still guards the target
    // side: a user holding one may not be borrowed, whatever the
    // operator's back-office role.
    const sysadmin = await apiLogin(request, SYSADMIN_TEST_CREDENTIALS);

    const resp = await request.post(`/api/v1/admin/users/${sysadmin.userId}/impersonate`, {
      headers: { 'Content-Type': 'application/json', ...bearer(operator.token) },
      data: { reason: 'e2e admin-target rejection check (#2100)' },
    });
    expect(resp.status(), 'impersonating a system admin must be rejected').toBe(422);
    // The JSON:API error envelope pins the specific guard that fired.
    expect(await resp.text()).toContain('admin.impersonate.target_is_admin');
  });

  test('impersonation safety: a support agent may read but may not impersonate', async ({
    request,
  }) => {
    const support = await operatorApiLogin(request, BACKOFFICE_SUPPORT_CREDENTIALS);
    const orphan = await apiLogin(request, ORPHAN_TEST_CREDENTIALS);

    // support_agent keeps read access to the admin surface.
    const tenants = await request.get('/api/v1/admin/tenants', { headers: bearer(support.token) });
    expect(tenants.status()).toBe(200);

    // But RequirePlatformAdmin refuses it at impersonation start.
    const resp = await request.post(`/api/v1/admin/users/${orphan.userId}/impersonate`, {
      headers: { 'Content-Type': 'application/json', ...bearer(support.token) },
      data: { reason: 'e2e role-boundary check (#2100)' },
    });
    expect(resp.status(), 'support_agent must not start impersonation').toBe(403);
    expect(await resp.text()).toContain('admin.role_required');
  });
});
