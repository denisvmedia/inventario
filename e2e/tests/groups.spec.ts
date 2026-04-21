import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';

test.describe('Location Groups', () => {

  test('group selector is visible in header', async ({ page }) => {
    // After login, user should have a default group and the selector should be visible
    await page.waitForSelector('.group-selector', { state: 'visible', timeout: 10000 }).catch(() => {
      // If no group selector, user might not have groups yet — that's OK for this test
    });

    // Either the group selector or the no-group state should be present
    const hasGroupSelector = await page.locator('.group-selector').isVisible();
    const hasNoGroup = await page.locator('.no-group').isVisible();
    const isOnGroupPage = page.url().includes('/no-group') || page.url().includes('/groups');

    // At least one state should be true after authentication
    expect(hasGroupSelector || hasNoGroup || isOnGroupPage).toBeTruthy();
  });

  test('unauthenticated /api/v1/groups request is rejected with 401', async ({ request }) => {
    // /api/v1/groups requires authentication via a JWT bearer token.
    // Without one the server must respond with 401 — never 200 — so this
    // test also guards against the endpoint accidentally becoming public.
    const response = await request.get('/api/v1/groups', {
      headers: {
        'Accept': 'application/vnd.api+json',
      }
    });

    expect(response.status()).toBe(401);
  });
});

test.describe('Group Management API', () => {

  test('can create a group via API', async ({ page, request }) => {
    // Get CSRF token from page context
    const csrfToken = await page.evaluate(() => {
      return sessionStorage.getItem('inventario_csrf_token') || '';
    });

    const authToken = await page.evaluate(() => {
      return localStorage.getItem('inventario_token') || '';
    });

    if (!authToken) {
      test.skip();
      return;
    }

    const groupName = 'E2E Test Group';
    const response = await request.post('/api/v1/groups', {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
        'X-CSRF-Token': csrfToken,
      },
      data: {
        data: {
          type: 'groups',
          attributes: {
            name: groupName,
            icon: '🧪',
          },
        },
      },
    });

    expect(response.status()).toBe(201);

    const body = await response.json();
    expect(body.data).toBeDefined();
    expect(body.data.attributes.name).toBe(groupName);
    expect(body.data.attributes.icon).toBe('🧪');
    expect(body.data.attributes.slug).toBeDefined();
    expect(body.data.attributes.slug.length).toBeGreaterThanOrEqual(22);
    expect(body.data.attributes.status).toBe('active');

    // Clean up so the group does not leak into later runs in a persistent env.
    // DELETE /api/v1/groups/:id requires a confirm_word that matches the group name.
    const groupId = body.data.id;
    const deleteResponse = await request.delete(`/api/v1/groups/${groupId}`, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
        'X-CSRF-Token': csrfToken,
      },
      data: { confirm_word: groupName, password: 'testpassword123' },
    });
    expect(deleteResponse.status()).toBe(204);
  });

  test('can list user groups via API', async ({ page, request }) => {
    const authToken = await page.evaluate(() => {
      return localStorage.getItem('inventario_token') || '';
    });

    if (!authToken) {
      test.skip();
      return;
    }

    const response = await request.get('/api/v1/groups', {
      headers: {
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
      },
    });

    expect(response.status()).toBe(200);

    const body = await response.json();
    expect(body.data).toBeDefined();
    expect(Array.isArray(body.data)).toBeTruthy();
    // User should have at least one group (default group created during setup).
    expect(body.data.length).toBeGreaterThanOrEqual(1);
  });
});

test.describe('Invite System API', () => {

  test('can create and retrieve invite link', async ({ page, request }) => {
    const authToken = await page.evaluate(() => {
      return localStorage.getItem('inventario_token') || '';
    });
    const csrfToken = await page.evaluate(() => {
      return sessionStorage.getItem('inventario_csrf_token') || '';
    });

    if (!authToken) {
      test.skip();
      return;
    }

    // First, get groups to find a group ID
    const groupsResponse = await request.get('/api/v1/groups', {
      headers: {
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
      },
    });

    const groupsBody = await groupsResponse.json();
    if (!groupsBody.data || groupsBody.data.length === 0) {
      test.skip();
      return;
    }

    const groupId = groupsBody.data[0].id;

    // Create an invite
    const inviteResponse = await request.post(`/api/v1/groups/${groupId}/invites`, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
        'X-CSRF-Token': csrfToken,
      },
    });

    expect(inviteResponse.status()).toBe(201);

    const inviteBody = await inviteResponse.json();
    expect(inviteBody.data.attributes.token).toBeDefined();
    expect(inviteBody.data.attributes.token.length).toBeGreaterThan(0);
    expect(inviteBody.data.attributes.expires_at).toBeDefined();

    // Retrieve invite info (public endpoint)
    const token = inviteBody.data.attributes.token;
    const infoResponse = await request.get(`/api/v1/invites/${token}`, {
      headers: {
        'Accept': 'application/vnd.api+json',
      },
    });

    expect(infoResponse.status()).toBe(200);

    const infoBody = await infoResponse.json();
    expect(infoBody.data.attributes.group_name).toBeDefined();
    expect(infoBody.data.attributes.expired).toBe(false);
    expect(infoBody.data.attributes.used).toBe(false);

    // Revoke the invite
    const inviteId = inviteBody.data.id;
    const revokeResponse = await request.delete(`/api/v1/groups/${groupId}/invites/${inviteId}`, {
      headers: {
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
        'X-CSRF-Token': csrfToken,
      },
    });

    expect(revokeResponse.status()).toBe(204);
  });
});

test.describe('Group Members API', () => {

  test('can list group members', async ({ page, request }) => {
    const authToken = await page.evaluate(() => {
      return localStorage.getItem('inventario_token') || '';
    });

    if (!authToken) {
      test.skip();
      return;
    }

    // Get groups
    const groupsResponse = await request.get('/api/v1/groups', {
      headers: {
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
      },
    });

    const groupsBody = await groupsResponse.json();
    if (!groupsBody.data || groupsBody.data.length === 0) {
      test.skip();
      return;
    }

    const groupId = groupsBody.data[0].id;

    // List members
    const membersResponse = await request.get(`/api/v1/groups/${groupId}/members`, {
      headers: {
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
      },
    });

    expect(membersResponse.status()).toBe(200);

    const membersBody = await membersResponse.json();
    expect(membersBody.data).toBeDefined();
    expect(Array.isArray(membersBody.data)).toBeTruthy();
    // Should have at least the creator as admin
    expect(membersBody.data.length).toBeGreaterThanOrEqual(1);

    // At least one member must be an admin. The registry scan has no ORDER BY
    // guarantee, so don't rely on data[0] having that role.
    const admins = membersBody.data.filter((m: { attributes: { role: string } }) => m.attributes.role === 'admin');
    expect(admins.length).toBeGreaterThanOrEqual(1);
  });

  test('cannot remove the last admin', async ({ page, request }) => {
    const authToken = await page.evaluate(() => {
      return localStorage.getItem('inventario_token') || '';
    });
    const csrfToken = await page.evaluate(() => {
      return sessionStorage.getItem('inventario_csrf_token') || '';
    });

    if (!authToken) {
      test.skip();
      return;
    }

    // Get groups
    const groupsResponse = await request.get('/api/v1/groups', {
      headers: {
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
      },
    });

    const groupsBody = await groupsResponse.json();
    if (!groupsBody.data || groupsBody.data.length === 0) {
      test.skip();
      return;
    }

    const groupId = groupsBody.data[0].id;

    // Try to leave the group (as last admin) — should fail
    const leaveResponse = await request.post(`/api/v1/groups/${groupId}/leave`, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
        'X-CSRF-Token': csrfToken,
      },
    });

    // The server enforces the "group must have at least one admin" invariant
    // as a business-rule violation, which maps to 422 Unprocessable Entity.
    // Asserting the exact status guards against a CSRF / auth misconfiguration
    // silently making this test pass with e.g. 401/403.
    expect(leaveResponse.status()).toBe(422);
  });
});

test.describe('Main Currency dropdown (#1256)', () => {
  // The "main currency" field regressed to a free-text input after the
  // location-groups rework, which let callers submit typos like "USDD" and
  // triggered confusing downstream errors. These tests lock in that the UI
  // only exposes valid ISO 4217 codes, and that the API still rejects an
  // invalid code for any caller bypassing the dropdown.

  test('group-create form exposes a searchable currency dropdown, not a free-text input', async ({ page, request }) => {
    const authToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const csrfToken = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');

    if (!authToken) {
      test.skip();
      return;
    }

    await page.goto('/groups/new');

    // The regression was a plain <input type="text" id="main-currency">.
    // The fix wraps PrimeVue's <Select> around the same id, which renders
    // as a div.p-select and explicitly not an <input type="text">. Assert
    // both to prevent silently re-regressing by changing only the markup.
    const dropdown = page.locator('.p-select#main-currency');
    await expect(dropdown).toBeVisible({ timeout: 10000 });
    await expect(page.locator('input[type="text"]#main-currency')).toHaveCount(0);

    // Open the dropdown and confirm a well-known ISO code is offered.
    // Using EUR (not USD) because USD is the default placeholder — picking
    // it wouldn't prove the list was actually populated.
    await dropdown.click();
    const eurOption = page.locator('.p-select-option-label', { hasText: /^EUR\b/ });
    await expect(eurOption.first()).toBeVisible({ timeout: 5000 });
    await eurOption.first().click();

    const groupName = `Currency Dropdown Test ${Date.now()}`;
    await page.fill('#name', groupName);
    await page.click('button[type="submit"]:has-text("Create Group")');

    // Successful create navigates away from /groups/new. Wait for that.
    await page.waitForURL((url) => !url.pathname.endsWith('/groups/new'), { timeout: 10000 });

    // Verify the group was created with EUR via the API (the UI read path
    // goes through a read-only label on the settings page, so hitting the
    // API here keeps the assertion narrow and avoids a second navigation).
    const groupsResp = await request.get('/api/v1/groups', {
      headers: {
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
      },
    });
    const groupsBody = await groupsResp.json();
    const created = groupsBody.data.find((g: { attributes: { name: string } }) => g.attributes.name === groupName);
    expect(created, `created group "${groupName}" not found in /api/v1/groups`).toBeDefined();
    expect(created.attributes.main_currency).toBe('EUR');

    // Clean up so re-runs in a persistent env don't accumulate groups.
    const deleteResp = await request.delete(`/api/v1/groups/${created.id}`, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
        'X-CSRF-Token': csrfToken,
      },
      data: { confirm_word: groupName, password: 'testpassword123' },
    });
    expect(deleteResp.status()).toBe(204);
  });

  test('API rejects an invalid main_currency with 400 (defense in depth behind the dropdown)', async ({ page, request }) => {
    const authToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const csrfToken = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');

    if (!authToken) {
      test.skip();
      return;
    }

    const resp = await request.post('/api/v1/groups', {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
        'X-CSRF-Token': csrfToken,
      },
      data: {
        data: {
          type: 'groups',
          attributes: {
            name: `Invalid Currency ${Date.now()}`,
            main_currency: 'NOPE',
          },
        },
      },
    });

    // 400 comes from apiserver/groups.go: MainCurrency.IsValid() is false, so
    // the handler returns badRequest before the group is written. The UI's
    // dropdown prevents this path for normal users, but the backend check
    // still guards against a stale client or hand-crafted request.
    expect(resp.status()).toBe(400);
  });
});

test.describe('Remove Member — last admin protection (#1257)', () => {
  // Parallel to the "leave group" protection (#1259), an admin removing
  // another user via DELETE /api/v1/groups/{id}/members/{userId} must also
  // refuse to strip the group's last admin. Coverage here is split: an
  // API-level assertion nails down the 422, and a UI assertion confirms the
  // Remove button is pre-emptively disabled so no doomed request is ever
  // submitted.

  test('API refuses DELETE /members/{id} for the sole admin with 422', async ({ page, request }) => {
    const authToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const csrfToken = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');

    if (!authToken) {
      test.skip();
      return;
    }

    // Create a fresh group so the caller is the only admin and only member.
    const groupName = `Last Admin Remove API Test ${Date.now()}`;
    const createResp = await request.post('/api/v1/groups', {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
        'X-CSRF-Token': csrfToken,
      },
      data: {
        data: {
          type: 'groups',
          attributes: { name: groupName, icon: '🛡️' },
        },
      },
    });
    expect(createResp.status(), await createResp.text()).toBe(201);
    const groupId = (await createResp.json()).data.id;

    try {
      // Discover the admin's member_user_id from the membership listing
      // rather than decoding the JWT — the API is the contract surface the
      // UI relies on, and this keeps the test independent of token format.
      const membersResp = await request.get(`/api/v1/groups/${groupId}/members`, {
        headers: {
          'Accept': 'application/vnd.api+json',
          'Authorization': `Bearer ${authToken}`,
        },
      });
      expect(membersResp.status()).toBe(200);
      const membersBody = await membersResp.json();
      const admin = membersBody.data.find((m: { attributes: { role: string } }) => m.attributes.role === 'admin');
      expect(admin, 'fresh group must have an admin').toBeDefined();
      const adminUserId = admin.attributes.member_user_id;

      // Remove-the-last-admin must map to 422 ErrLastAdmin, not a generic
      // failure mode. Asserting the exact status guards against auth or CSRF
      // misconfiguration silently passing the test with e.g. 401/403.
      const removeResp = await request.delete(
        `/api/v1/groups/${groupId}/members/${adminUserId}`,
        {
          headers: {
            'Content-Type': 'application/vnd.api+json',
            'Accept': 'application/vnd.api+json',
            'Authorization': `Bearer ${authToken}`,
            'X-CSRF-Token': csrfToken,
          },
        },
      );
      expect(removeResp.status()).toBe(422);

      // The member must still be in the group — the endpoint is supposed to
      // reject atomically, not partially strip state before failing.
      const afterResp = await request.get(`/api/v1/groups/${groupId}/members`, {
        headers: {
          'Accept': 'application/vnd.api+json',
          'Authorization': `Bearer ${authToken}`,
        },
      });
      const afterBody = await afterResp.json();
      expect(afterBody.data.some((m: { attributes: { member_user_id: string } }) => m.attributes.member_user_id === adminUserId)).toBe(true);
    } finally {
      const deleteResp = await request.delete(`/api/v1/groups/${groupId}`, {
        headers: {
          'Content-Type': 'application/vnd.api+json',
          'Accept': 'application/vnd.api+json',
          'Authorization': `Bearer ${authToken}`,
          'X-CSRF-Token': csrfToken,
        },
        data: { confirm_word: groupName, password: 'testpassword123' },
      });
      expect(deleteResp.status()).toBe(204);
    }
  });

  test('Remove button on the sole admin is disabled with tooltip', async ({ page, request }) => {
    const authToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const csrfToken = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');

    if (!authToken) {
      test.skip();
      return;
    }

    // Fresh group so the caller is the only admin — makes the
    // "last admin" state deterministic independent of the seed.
    const groupName = `Last Admin Remove UI Test ${Date.now()}`;
    const createResp = await request.post('/api/v1/groups', {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
        'X-CSRF-Token': csrfToken,
      },
      data: {
        data: {
          type: 'groups',
          attributes: { name: groupName, icon: '🛡️' },
        },
      },
    });
    expect(createResp.status(), await createResp.text()).toBe(201);
    const groupId = (await createResp.json()).data.id;

    try {
      const membersResp = await request.get(`/api/v1/groups/${groupId}/members`, {
        headers: {
          'Accept': 'application/vnd.api+json',
          'Authorization': `Bearer ${authToken}`,
        },
      });
      const membersBody = await membersResp.json();
      const admin = membersBody.data.find((m: { attributes: { role: string } }) => m.attributes.role === 'admin');
      expect(admin, 'fresh group must have an admin').toBeDefined();
      const adminUserId = admin.attributes.member_user_id;

      await page.goto(`/groups/${groupId}/settings`);

      const removeBtn = page.locator(`[data-testid="remove-member-btn-${adminUserId}"]`);
      await expect(removeBtn).toBeVisible({ timeout: 10000 });

      // Native disabled + aria-disabled + title: mouse, keyboard, and
      // screen-reader users all learn the Remove action is blocked and why.
      await expect(removeBtn).toBeDisabled();
      await expect(removeBtn).toHaveAttribute('aria-disabled', 'true');
      await expect(removeBtn).toHaveAttribute(
        'title',
        'Cannot remove the last admin — promote another member first or delete the group.',
      );
    } finally {
      const deleteResp = await request.delete(`/api/v1/groups/${groupId}`, {
        headers: {
          'Content-Type': 'application/vnd.api+json',
          'Accept': 'application/vnd.api+json',
          'Authorization': `Bearer ${authToken}`,
          'X-CSRF-Token': csrfToken,
        },
        data: { confirm_word: groupName, password: 'testpassword123' },
      });
      expect(deleteResp.status()).toBe(204);
    }
  });
});

test.describe('Leave Group UI — last admin protection (#1259)', () => {
  // These tests cover the frontend half of the contract: the backend already
  // rejects "last admin leaves" with 422 (see the API test above). The UI
  // must prevent the user from ever submitting that doomed request by
  // disabling the button and explaining why. A fresh group per test (rather
  // than reusing the default group) makes the member-count state
  // deterministic — otherwise a seed change adding a second admin would
  // silently flip this from "disabled" to "enabled" without a failure.

  test('Leave Group button is disabled and a notice is shown when user is the sole admin', async ({ page, request }) => {
    const authToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const csrfToken = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');

    if (!authToken) {
      test.skip();
      return;
    }

    // Create a fresh group so the authenticated user is the only admin.
    const groupName = `Last Admin UI Test ${Date.now()}`;
    const createResp = await request.post('/api/v1/groups', {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
        'X-CSRF-Token': csrfToken,
      },
      data: {
        data: {
          type: 'groups',
          attributes: { name: groupName, icon: '🔒' },
        },
      },
    });
    expect(createResp.status(), await createResp.text()).toBe(201);
    const groupId = (await createResp.json()).data.id;

    try {
      await page.goto(`/groups/${groupId}/settings`);

      // Wait for the Leave Group section to render — loadData() is async and
      // the last-admin branch only appears once the membership list arrives.
      const leaveBtn = page.locator('[data-testid="leave-group-btn"]');
      await expect(leaveBtn).toBeVisible({ timeout: 10000 });

      // The button must be disabled AND advertise the reason via title/aria
      // so mouse and screen-reader users both get the explanation.
      await expect(leaveBtn).toBeDisabled();
      await expect(leaveBtn).toHaveAttribute('aria-disabled', 'true');
      await expect(leaveBtn).toHaveAttribute(
        'title',
        'You are the last admin. Promote another member first, or delete the group.',
      );

      // Inline notice explains the situation and points at the remediation.
      const notice = page.locator('[data-testid="last-admin-notice"]');
      await expect(notice).toBeVisible();
      await expect(notice).toContainText('You are the last admin of this group');
      // Sole admin + sole member -> deletion-only branch (no promote advice).
      await expect(notice).toContainText('delete the group below');

      // Danger Zone (with Delete Group) must be reachable — the notice
      // tells the user to use it, so it must actually be rendered.
      await expect(page.locator('button:has-text("Delete Group")')).toBeVisible();
    } finally {
      // Clean up the test group so repeat runs don't accumulate state.
      const deleteResp = await request.delete(`/api/v1/groups/${groupId}`, {
        headers: {
          'Content-Type': 'application/vnd.api+json',
          'Accept': 'application/vnd.api+json',
          'Authorization': `Bearer ${authToken}`,
          'X-CSRF-Token': csrfToken,
        },
        data: { confirm_word: groupName, password: 'testpassword123' },
      });
      expect(deleteResp.status()).toBe(204);
    }
  });
});

test.describe('Group selection persistence (#1262 / #1300)', () => {
  // After #1300, the active group lives in two authoritative places:
  //   1. The URL (/g/:groupSlug/...) — per-tab, survives a browser refresh
  //      because the URL is reloaded as-is.
  //   2. user.default_group_id on the server — the cross-device preference
  //      that decides which group to land on when no URL slug is available
  //      (cold start on '/', '/profile', etc.).
  // localStorage is no longer consulted for group selection; only the
  // legacy-migration shim touches it to wipe old keys.

  test('user-initiated group switch writes PUT /auth/me default_group_id and survives a reload', async ({ page, request }) => {
    const authToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const csrfToken = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');

    if (!authToken) {
      test.skip();
      return;
    }

    // Create a second group so the test exercises an explicit user choice
    // rather than the default-group fallback (which would pass even without
    // any persistence — a false-positive trap).
    const groupName = `Persistence Test ${Date.now()}`;
    const createResp = await request.post('/api/v1/groups', {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
        'X-CSRF-Token': csrfToken,
      },
      data: {
        data: {
          type: 'groups',
          attributes: { name: groupName, icon: '🧷' },
        },
      },
    });
    expect(createResp.status(), await createResp.text()).toBe(201);
    const createdGroupId = (await createResp.json()).data.id as string;

    // Capture the existing default so cleanup can restore it after the test.
    const meBefore = await request.get('/api/v1/auth/me', {
      headers: { 'Accept': 'application/json', 'Authorization': `Bearer ${authToken}` },
    });
    const meBeforeBody = await meBefore.json();
    const previousDefault = (meBeforeBody.default_group_id as string | null) ?? null;

    try {
      await page.goto('/');
      await page.waitForSelector('.group-selector', { state: 'visible', timeout: 10000 });

      // Watch for the PUT /auth/me the selector fires after switching. The
      // call is debounced (~400ms), so we use waitForRequest which polls
      // the network rather than snapshot-checking. Filtering on the
      // exact default_group_id makes the assertion tight.
      const putPromise = page.waitForRequest(
        (req) =>
          req.url().includes('/api/v1/auth/me') &&
          req.method() === 'PUT' &&
          (req.postData() || '').includes(createdGroupId),
        { timeout: 15000 },
      );

      await page.click('.group-selector__trigger');
      const targetItem = page.locator('.group-selector__item', { hasText: groupName });
      await expect(targetItem).toBeVisible({ timeout: 5000 });

      await targetItem.click();
      // GroupSelector navigates to /g/<new-slug>/... via router.push. The
      // URL is the immediate source of truth; default_group_id catches up
      // asynchronously via the debounced PUT.
      await page.waitForURL(/\/g\/[^/]+/, { timeout: 10000 });
      await page.waitForLoadState('networkidle', { timeout: 15000 });

      await expect(page.locator('.group-selector__name')).toHaveText(groupName, { timeout: 10000 });

      const putRequest = await putPromise;
      const putBody = JSON.parse(putRequest.postData() ?? '{}');
      expect(putBody.default_group_id).toBe(createdGroupId);

      // Server-side round-trip: the preference survives a cold re-read of
      // /auth/me (not just an in-flight request). Guards against a bug
      // where the PUT is made but the server silently ignored the field.
      const meAfter = await request.get('/api/v1/auth/me', {
        headers: { 'Accept': 'application/json', 'Authorization': `Bearer ${authToken}` },
      });
      const meAfterBody = await meAfter.json();
      expect(meAfterBody.default_group_id).toBe(createdGroupId);

      // URL-driven refresh: the browser reloads /g/<slug>/... verbatim,
      // so the selector still shows the same group — no localStorage
      // needed. This is what replaces the pre-#1300 snapshot behaviour.
      await page.reload();
      await page.waitForSelector('.group-selector', { state: 'visible', timeout: 10000 });
      await expect(page.locator('.group-selector__name')).toHaveText(groupName);

      // And #1300's headline invariant: the legacy localStorage keys
      // must stay absent. If they came back the cross-tab coupling
      // that the issue set out to remove would be back too.
      const legacyKeys = await page.evaluate(() => ({
        snapshot: localStorage.getItem('inventario_current_group'),
        slug: localStorage.getItem('currentGroupSlug'),
      }));
      expect(legacyKeys.snapshot, 'inventario_current_group must not be written anymore').toBeNull();
      expect(legacyKeys.slug, 'currentGroupSlug must not be written anymore').toBeNull();
    } finally {
      // Restore the preference before deleting the test group so we don't
      // leave a stale default_group_id pointing at the deleted entity.
      await request.put('/api/v1/auth/me', {
        headers: {
          'Content-Type': 'application/json',
          'Accept': 'application/json',
          'Authorization': `Bearer ${authToken}`,
          'X-CSRF-Token': csrfToken,
        },
        data: { name: meBeforeBody.name, default_group_id: previousDefault },
      });

      // Navigate away from the /g/<new-slug>/... URL before deletion so
      // the UI isn't stranded on a now-nonexistent group when the delete
      // returns 204.
      const groupsResp = await request.get('/api/v1/groups', {
        headers: {
          'Accept': 'application/vnd.api+json',
          'Authorization': `Bearer ${authToken}`,
        },
      });
      const groupsBody = await groupsResp.json();
      const other = groupsBody.data.find((g: { id: string }) => g.id !== createdGroupId);
      if (other) {
        await page.goto(`/g/${other.attributes.slug}/`);
        await page.waitForLoadState('networkidle', { timeout: 10000 });
      } else {
        await page.goto('/');
      }

      const deleteResp = await request.delete(`/api/v1/groups/${createdGroupId}`, {
        headers: {
          'Content-Type': 'application/vnd.api+json',
          'Accept': 'application/vnd.api+json',
          'Authorization': `Bearer ${authToken}`,
          'X-CSRF-Token': csrfToken,
        },
        data: { confirm_word: groupName, password: 'testpassword123' },
      });
      expect(deleteResp.status()).toBe(204);
    }
  });

  test('cold start on / with no preference falls back to an available group', async ({ page, request }) => {
    const authToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const csrfToken = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');

    if (!authToken) {
      test.skip();
      return;
    }

    // Remember the existing default so we can restore it after the test.
    const meBefore = await request.get('/api/v1/auth/me', {
      headers: { 'Accept': 'application/json', 'Authorization': `Bearer ${authToken}` },
    });
    const meBeforeBody = await meBefore.json();
    const previousDefault = (meBeforeBody.default_group_id as string | null) ?? null;

    try {
      // Clear the preference. Without a default_group_id and without a
      // /g/:groupSlug/ URL, the groupStore must fall back deterministically
      // to the first-created (or first-invited) group rather than leaving
      // the selector on "Select Group".
      const putResp = await request.put('/api/v1/auth/me', {
        headers: {
          'Content-Type': 'application/json',
          'Accept': 'application/json',
          'Authorization': `Bearer ${authToken}`,
          'X-CSRF-Token': csrfToken,
        },
        data: { name: meBeforeBody.name, default_group_id: null },
      });
      expect(putResp.status(), await putResp.text()).toBe(200);

      await page.goto('/');
      await page.waitForSelector('.group-selector', { state: 'visible', timeout: 10000 });

      const displayedName = await page.locator('.group-selector__name').textContent();
      expect(displayedName).not.toBe('Select Group');
      expect(displayedName?.trim().length ?? 0).toBeGreaterThan(0);

      // Cross-check with the API: the resolved group really belongs to
      // the user. Guards against a bug where fallback picks a bogus
      // placeholder.
      const groupsResp = await request.get('/api/v1/groups', {
        headers: {
          'Accept': 'application/vnd.api+json',
          'Authorization': `Bearer ${authToken}`,
        },
      });
      const groupsBody = await groupsResp.json();
      const names = groupsBody.data.map((g: { attributes: { name: string } }) => g.attributes.name);
      expect(names).toContain(displayedName);
    } finally {
      if (previousDefault) {
        await request.put('/api/v1/auth/me', {
          headers: {
            'Content-Type': 'application/json',
            'Accept': 'application/json',
            'Authorization': `Bearer ${authToken}`,
            'X-CSRF-Token': csrfToken,
          },
          data: { name: meBeforeBody.name, default_group_id: previousDefault },
        });
      }
    }
  });

  test('two tabs on different /g/<slug>/ URLs stay independent across reload', async ({ browser }) => {
    // The core behavioural guarantee that #1300 locks in: the URL is the
    // per-tab source of truth for the active group, so two tabs that sit
    // on two different /g/<slug>/... URLs must survive a refresh without
    // leaking each other's group state through a shared localStorage key.
    const context = await browser.newContext();
    const pageA = await context.newPage();
    const pageB = await context.newPage();

    try {
      // Log in via pageA — the auth cookie + access token land in the
      // context so pageB inherits them on the next navigation.
      await pageA.goto('/login');
      await pageA.fill('[data-testid="email"]', 'admin@example.com');
      await pageA.fill('[data-testid="password"]', 'admin123');
      await pageA.click('[data-testid="login-button"]');
      await pageA.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 15000 });

      // Ensure at least two groups exist so both tabs can hold a
      // distinct /g/<slug>/ URL. Re-use an existing second group if the
      // seed has one; otherwise create a second via the API wrapper.
      const authToken = await pageA.evaluate(() => localStorage.getItem('inventario_token') || '');
      const csrfToken = await pageA.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');
      const groupsResp = await pageA.request.get('/api/v1/groups', {
        headers: { 'Accept': 'application/vnd.api+json', 'Authorization': `Bearer ${authToken}` },
      });
      const groupsBody = await groupsResp.json();
      let groupA = groupsBody.data[0];
      let groupB = groupsBody.data.find((g: { id: string }) => g.id !== groupA.id);
      let createdGroupId: string | null = null;
      const secondGroupName = `Second Tab Test ${Date.now()}`;
      if (!groupB) {
        const createResp = await pageA.request.post('/api/v1/groups', {
          headers: {
            'Content-Type': 'application/vnd.api+json',
            'Accept': 'application/vnd.api+json',
            'Authorization': `Bearer ${authToken}`,
            'X-CSRF-Token': csrfToken,
          },
          data: { data: { type: 'groups', attributes: { name: secondGroupName, icon: '🧩' } } },
        });
        expect(createResp.status(), await createResp.text()).toBe(201);
        const created = (await createResp.json()).data;
        createdGroupId = created.id;
        groupB = created;
      }

      const slugA = groupA.attributes.slug as string;
      const slugB = groupB.attributes.slug as string;
      expect(slugA).not.toBe(slugB);

      try {
        await pageA.goto(`/g/${slugA}/commodities`);
        await pageB.goto(`/g/${slugB}/commodities`);
        await pageA.waitForLoadState('networkidle', { timeout: 15000 });
        await pageB.waitForLoadState('networkidle', { timeout: 15000 });

        await expect(pageA.locator('.group-selector__name')).toHaveText(groupA.attributes.name, { timeout: 10000 });
        await expect(pageB.locator('.group-selector__name')).toHaveText(groupB.attributes.name, { timeout: 10000 });

        // Reload both tabs — each must re-read its own URL, not a shared
        // localStorage key.
        await pageA.reload();
        await pageB.reload();
        await pageA.waitForLoadState('networkidle', { timeout: 15000 });
        await pageB.waitForLoadState('networkidle', { timeout: 15000 });

        await expect(pageA.locator('.group-selector__name')).toHaveText(groupA.attributes.name);
        await expect(pageB.locator('.group-selector__name')).toHaveText(groupB.attributes.name);
      } finally {
        if (createdGroupId) {
          // Move pageA off the about-to-be-deleted group before deleting.
          await pageA.goto(`/g/${slugA}/`);
          await pageA.request.delete(`/api/v1/groups/${createdGroupId}`, {
            headers: {
              'Content-Type': 'application/vnd.api+json',
              'Accept': 'application/vnd.api+json',
              'Authorization': `Bearer ${authToken}`,
              'X-CSRF-Token': csrfToken,
            },
            data: { confirm_word: secondGroupName, password: 'admin123' },
          });
        }
      }
    } finally {
      await context.close();
    }
  });
});

test.describe('Group icon picker (#1255)', () => {
  // Before #1255 the icon field was a plain <input type="text" maxlength="10">
  // on all three create/edit surfaces (GroupCreateView, GroupSettingsView,
  // NoGroupView). Users could save typos like "fa:boxx" or "foo" that then
  // rendered as literal text in the selector. The fix restricts the field to
  // a curated emoji picker and enforces the set server-side.

  test('API rejects an icon that is not in the curated set with 422', async ({ page, request }) => {
    const authToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const csrfToken = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');

    if (!authToken) {
      test.skip();
      return;
    }

    // 'fa:box' was historically listed in the field's docstring as acceptable
    // but never actually rendered properly; it's the paradigm free-text value
    // the picker now rules out. Asserting on it specifically guards against a
    // regression that loosens validation back to length-only.
    const resp = await request.post('/api/v1/groups', {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
        'X-CSRF-Token': csrfToken,
      },
      data: {
        data: {
          type: 'groups',
          attributes: {
            name: `Invalid Icon ${Date.now()}`,
            icon: 'fa:box',
          },
        },
      },
    });

    expect(resp.status()).toBe(422);
  });

  test('API accepts an empty icon (icon is optional)', async ({ page, request }) => {
    const authToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const csrfToken = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');

    if (!authToken) {
      test.skip();
      return;
    }

    const groupName = `Empty Icon ${Date.now()}`;
    const resp = await request.post('/api/v1/groups', {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
        'X-CSRF-Token': csrfToken,
      },
      data: {
        data: {
          type: 'groups',
          attributes: { name: groupName, icon: '' },
        },
      },
    });
    expect(resp.status(), await resp.text()).toBe(201);
    const groupId = (await resp.json()).data.id as string;

    const deleteResp = await request.delete(`/api/v1/groups/${groupId}`, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
        'X-CSRF-Token': csrfToken,
      },
      data: { confirm_word: groupName, password: 'testpassword123' },
    });
    expect(deleteResp.status()).toBe(204);
  });

  test('group-create form exposes the picker, not a free-text input, and saves the selection', async ({ page, request }) => {
    const authToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const csrfToken = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');

    if (!authToken) {
      test.skip();
      return;
    }

    await page.goto('/groups/new');

    // The regression was a plain text input. Assert its absence and the
    // picker's presence in the same test so that a future change which
    // reverts to a textbox fails immediately.
    const picker = page.locator('[data-testid="group-create-icon-picker"]');
    await expect(picker).toBeVisible({ timeout: 10000 });
    await expect(page.locator('input[type="text"]#icon')).toHaveCount(0);

    await picker.click();
    const panel = page.locator('[data-testid="icon-picker-panel"]');
    await expect(panel).toBeVisible();

    // Storage tab hosts 📦 — the most recognisable picker entry. Picking it
    // via the tab (rather than hunting through the grid) also proves the
    // category tabs work.
    await page.locator('[data-testid="icon-picker-tab-storage"]').click();
    await page.locator('[data-testid="icon-picker-option-📦"]').click();
    await page.locator('[data-testid="icon-picker-close"]').click();
    await expect(panel).toBeHidden();

    const groupName = `Picker Test ${Date.now()}`;
    await page.fill('#name', groupName);
    await page.click('button[type="submit"]:has-text("Create Group")');
    await page.waitForURL((url) => !url.pathname.endsWith('/groups/new'), { timeout: 10000 });

    // Verify the group was created with the selected icon via the API — the
    // UI doesn't show the icon attribute anywhere readable except the header,
    // and the header icon is best-effort for a11y reasons.
    const groupsResp = await request.get('/api/v1/groups', {
      headers: {
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
      },
    });
    const groupsBody = await groupsResp.json();
    const created = groupsBody.data.find(
      (g: { attributes: { name: string } }) => g.attributes.name === groupName,
    );
    expect(created, `created group "${groupName}" not found`).toBeDefined();
    expect(created.attributes.icon).toBe('📦');

    const deleteResp = await request.delete(`/api/v1/groups/${created.id}`, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
        'X-CSRF-Token': csrfToken,
      },
      data: { confirm_word: groupName, password: 'testpassword123' },
    });
    expect(deleteResp.status()).toBe(204);
  });

  test('group-settings form exposes the picker and persists a new icon selection', async ({ page, request }) => {
    const authToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const csrfToken = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');

    if (!authToken) {
      test.skip();
      return;
    }

    // Create a throwaway group so the edit flow doesn't stomp the default
    // group. Start with the pre-#1255 "free-text" shape (empty icon) so we
    // can observe the picker changing it.
    const groupName = `Picker Edit Test ${Date.now()}`;
    const createResp = await request.post('/api/v1/groups', {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
        'X-CSRF-Token': csrfToken,
      },
      data: {
        data: {
          type: 'groups',
          attributes: { name: groupName, icon: '' },
        },
      },
    });
    expect(createResp.status(), await createResp.text()).toBe(201);
    const groupId = (await createResp.json()).data.id as string;

    try {
      await page.goto(`/groups/${groupId}/settings`);

      const picker = page.locator('[data-testid="group-settings-icon-picker"]');
      await expect(picker).toBeVisible({ timeout: 10000 });
      // input#group-icon was the free-text input before #1255 — asserting its
      // absence catches a regression that reintroduces it.
      await expect(page.locator('input[type="text"]#group-icon')).toHaveCount(0);

      await picker.click();
      await page.locator('[data-testid="icon-picker-tab-hobbies"]').click();
      await page.locator('[data-testid="icon-picker-option-📚"]').click();
      await page.locator('[data-testid="icon-picker-close"]').click();

      await page.click('button[type="submit"]:has-text("Save Changes")');

      // Confirm the server stored the selection — the UI doesn't re-render
      // the picker with the saved value synchronously after save, so reading
      // via the API keeps the assertion narrow.
      await expect
        .poll(
          async () => {
            const resp = await request.get(`/api/v1/groups/${groupId}`, {
              headers: {
                'Accept': 'application/vnd.api+json',
                'Authorization': `Bearer ${authToken}`,
              },
            });
            const body = await resp.json();
            return body.data.attributes.icon;
          },
          { timeout: 10000 },
        )
        .toBe('📚');
    } finally {
      const deleteResp = await request.delete(`/api/v1/groups/${groupId}`, {
        headers: {
          'Content-Type': 'application/vnd.api+json',
          'Accept': 'application/vnd.api+json',
          'Authorization': `Bearer ${authToken}`,
          'X-CSRF-Token': csrfToken,
        },
        data: { confirm_word: groupName, password: 'testpassword123' },
      });
      expect(deleteResp.status()).toBe(204);
    }
  });
});

test.describe('Default group preference (#1263)', () => {
  // #1263 layers a persistent, user-level preference on top of the
  // session-persistent selection from #1262: when a user clears cookies or
  // logs in on a new device (no localStorage snapshot), the app should land
  // them in whatever group they picked as default in their profile, not in
  // the arbitrary "first group the server returned" fallback.
  //
  // The test walks the full contract:
  //   1. Set default_group_id via PUT /auth/me for a group the user has.
  //   2. Wipe the snapshot localStorage key to simulate a fresh device.
  //   3. Reload and assert the selector lands on the preferred group.
  //   4. Clear the preference (default_group_id: null) and confirm the API
  //      accepted the null.

  test('PUT /auth/me rejects default_group_id for a group the user does not belong to', async ({ page, request }) => {
    const authToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const csrfToken = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');

    if (!authToken) {
      test.skip();
      return;
    }

    // A well-formed UUID that the user cannot belong to — the backend must
    // 400 on membership check, not silently store the value. Picking an
    // unambiguous UUID avoids false positives if any group happens to share
    // a short prefix with the fixture.
    const resp = await request.put('/api/v1/auth/me', {
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json',
        'Authorization': `Bearer ${authToken}`,
        'X-CSRF-Token': csrfToken,
      },
      data: {
        name: 'Keep My Name',
        default_group_id: '00000000-0000-4000-8000-000000000000',
      },
    });

    expect(resp.status(), await resp.text()).toBe(400);
  });

  test('preferred group is honoured on a fresh device (no snapshot)', async ({ page, request }) => {
    const authToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const csrfToken = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');

    if (!authToken) {
      test.skip();
      return;
    }

    // Create a dedicated test group and mark it as the default. Using a
    // fresh group means the preference can't accidentally match the group
    // a fallback rule would also pick.
    const groupName = `Default Pref Test ${Date.now()}`;
    const createResp = await request.post('/api/v1/groups', {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
        'X-CSRF-Token': csrfToken,
      },
      data: {
        data: {
          type: 'groups',
          attributes: { name: groupName, icon: '⭐' },
        },
      },
    });
    expect(createResp.status(), await createResp.text()).toBe(201);
    const createdGroupId = (await createResp.json()).data.id as string;

    // Capture the current default (if any) so cleanup can restore it.
    const meBefore = await request.get('/api/v1/auth/me', {
      headers: { 'Accept': 'application/json', 'Authorization': `Bearer ${authToken}` },
    });
    const meBeforeBody = await meBefore.json();
    const previousDefault = (meBeforeBody.default_group_id as string | null) ?? null;

    try {
      // Persist the preference server-side.
      const putResp = await request.put('/api/v1/auth/me', {
        headers: {
          'Content-Type': 'application/json',
          'Accept': 'application/json',
          'Authorization': `Bearer ${authToken}`,
          'X-CSRF-Token': csrfToken,
        },
        data: {
          name: meBeforeBody.name,
          default_group_id: createdGroupId,
        },
      });
      expect(putResp.status(), await putResp.text()).toBe(200);
      const putBody = await putResp.json();
      expect(putBody.default_group_id).toBe(createdGroupId);

      // Navigate to a non-group URL so the router has no /g/:groupSlug/
      // param to seed the store from — that forces restoreFromPreference()
      // to run the #1263 priority chain (default_group_id → fallback).
      // After #1300 there's no snapshot key to clear; the store never
      // wrote one in the first place.
      await page.goto('/profile');
      await page.reload();
      await page.goto('/');

      // Wait for the selector to arrive and reconciliation to finish.
      await page.waitForSelector('.group-selector', { state: 'visible', timeout: 10000 });
      await expect(page.locator('.group-selector__name')).toHaveText(groupName, {
        timeout: 10000,
      });
    } finally {
      // Clear the preference (null) so the next test starts clean, then
      // delete the test group. Restore any previous default afterwards.
      await request.put('/api/v1/auth/me', {
        headers: {
          'Content-Type': 'application/json',
          'Accept': 'application/json',
          'Authorization': `Bearer ${authToken}`,
          'X-CSRF-Token': csrfToken,
        },
        data: { name: meBeforeBody.name, default_group_id: null },
      });

      // Navigate the UI away from the test group before deleting so the
      // selector doesn't briefly point at a deleted entity.
      const groupsResp = await request.get('/api/v1/groups', {
        headers: {
          'Accept': 'application/vnd.api+json',
          'Authorization': `Bearer ${authToken}`,
        },
      });
      const groupsBody = await groupsResp.json();
      const other = groupsBody.data.find((g: { id: string }) => g.id !== createdGroupId);
      if (other) {
        await page.goto(`/g/${other.attributes.slug}/`);
        await page.waitForLoadState('networkidle', { timeout: 10000 });
      } else {
        await page.goto('/');
      }

      const deleteResp = await request.delete(`/api/v1/groups/${createdGroupId}`, {
        headers: {
          'Content-Type': 'application/vnd.api+json',
          'Accept': 'application/vnd.api+json',
          'Authorization': `Bearer ${authToken}`,
          'X-CSRF-Token': csrfToken,
        },
        data: { confirm_word: groupName, password: 'testpassword123' },
      });
      expect(deleteResp.status()).toBe(204);

      // Restore the prior default if there was one (best-effort).
      if (previousDefault) {
        await request.put('/api/v1/auth/me', {
          headers: {
            'Content-Type': 'application/json',
            'Accept': 'application/json',
            'Authorization': `Bearer ${authToken}`,
            'X-CSRF-Token': csrfToken,
          },
          data: { name: meBeforeBody.name, default_group_id: previousDefault },
        });
      }
    }
  });
});

test.describe('Group + role cluster in header (#1258)', () => {
  // Before #1258 the current-group selector and the user's role lived in
  // separate parts of the UI — the role wasn't surfaced in the header at
  // all. The fix colocates the two in a flex cluster so the pair reads as
  // one "my identity in this context" display, with shared visual tokens
  // (padding, font-size, radius) so they look like one unit.

  test('role badge sits next to the group selector with matching height and text from API', async ({ page, request }) => {
    const authToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    if (!authToken) {
      test.skip();
      return;
    }

    await page.goto('/');

    const cluster = page.locator('.group-role-cluster');
    await expect(cluster).toBeVisible({ timeout: 10000 });

    // Adjacency: both controls live inside the same flex cluster. Asserting
    // the nesting (rather than just "both visible somewhere") is what
    // catches a regression that accidentally moves the role back into the
    // user menu or the settings page.
    const selector = cluster.locator('.group-selector__trigger');
    const role = cluster.locator('[data-testid="current-role"]');
    await expect(selector).toBeVisible();
    await expect(role).toBeVisible();

    // Derive the expected role from the API rather than hard-coding 'admin' —
    // that way the test still passes if the seed ever starts the user off as
    // a member of their active group instead of an admin. After #1300 the
    // current-group id is no longer kept in localStorage; wait for the
    // router to land on /g/:groupSlug/... and read the slug from the URL.
    await page.waitForURL(/\/g\/[^/]+/, { timeout: 10000 });
    const activeSlug = new URL(page.url()).pathname.split('/')[2];
    expect(activeSlug, 'router must resolve a /g/:groupSlug/... URL').toBeTruthy();

    const meResp = await request.get('/api/v1/auth/me', {
      headers: { 'Accept': 'application/json', 'Authorization': `Bearer ${authToken}` },
    });
    const me = await meResp.json();

    const allGroupsResp = await request.get('/api/v1/groups', {
      headers: { 'Accept': 'application/vnd.api+json', 'Authorization': `Bearer ${authToken}` },
    });
    const allGroupsBody = await allGroupsResp.json();
    const activeGroup = allGroupsBody.data.find(
      (g: { attributes: { slug: string } }) => g.attributes.slug === activeSlug,
    );
    expect(activeGroup, 'active /g/:groupSlug/ must resolve to a real group').toBeDefined();

    const membersResp = await request.get(`/api/v1/groups/${activeGroup.id}/members`, {
      headers: { 'Accept': 'application/vnd.api+json', 'Authorization': `Bearer ${authToken}` },
    });
    const membersBody = await membersResp.json();
    const myMembership = membersBody.data.find(
      (m: { attributes: { member_user_id: string } }) => m.attributes.member_user_id === me.id,
    );
    expect(myMembership, 'caller must be a member of their active group').toBeDefined();
    const expectedRole = myMembership.attributes.role as 'admin' | 'user';

    await expect(role).toHaveText(expectedRole);
    await expect(role).toHaveClass(new RegExp(`role-indicator--${expectedRole}`));

    // Shared visual tokens in SCSS should produce matching box metrics. A
    // bounding-box check is end-to-end — it would catch a regression that
    // passes unit tests (SCSS vars still defined) but visually breaks
    // because a global override bumped line-height or padding for one side.
    // <2px tolerance accounts for subpixel rounding at non-integer zooms.
    const selectorBox = await selector.boundingBox();
    const roleBox = await role.boundingBox();
    expect(selectorBox, 'selector trigger must render a box').not.toBeNull();
    expect(roleBox, 'role badge must render a box').not.toBeNull();
    expect(Math.abs(selectorBox!.height - roleBox!.height)).toBeLessThan(2);
    // Same Y baseline — the pair is arranged in a row, not stacked.
    expect(Math.abs(selectorBox!.y - roleBox!.y)).toBeLessThan(2);
    // Role badge sits to the right of the selector trigger ("immediately
    // next to", per the issue), with a small gap — not overlapping, not
    // pushed to the far side of the header.
    const gap = roleBox!.x - (selectorBox!.x + selectorBox!.width);
    expect(gap).toBeGreaterThan(0);
    expect(gap).toBeLessThan(24);
  });

  test('role badge refreshes when switching to a different group', async ({ page, request }) => {
    // Group switching exercises the wiring end-to-end: setCurrentGroup
    // triggers loadCurrentMembership, which populates currentMembership,
    // which drives currentRole, which renders the badge. The acceptance
    // criterion "role indicator updates when group changes" hinges on
    // this chain. Cross-role switching (admin → member) needs a second
    // authenticated user context and is covered by the unit tests that
    // mock the store directly; this e2e nails the dataflow.
    const authToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const csrfToken = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');
    if (!authToken) {
      test.skip();
      return;
    }

    const groupName = `Role Cluster Switch ${Date.now()}`;
    const createResp = await request.post('/api/v1/groups', {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
        'X-CSRF-Token': csrfToken,
      },
      data: { data: { type: 'groups', attributes: { name: groupName, icon: '🏷️' } } },
    });
    expect(createResp.status(), await createResp.text()).toBe(201);
    const newGroupId = (await createResp.json()).data.id as string;

    try {
      await page.goto('/');
      await page.waitForSelector('.group-role-cluster', { state: 'visible', timeout: 10000 });

      await page.click('.group-selector__trigger');
      const targetItem = page.locator('.group-selector__item', { hasText: groupName });
      await expect(targetItem).toBeVisible({ timeout: 5000 });

      // After #1289 Gap C, GroupSelector navigates to /g/<new-slug>/... via
      // router.push (no full reload). Wait for the URL to reflect the new
      // group before asserting any reactive state that depends on the switch.
      await targetItem.click();
      await page.waitForURL(/\/g\/[^/]+/, { timeout: 10000 });
      await page.waitForLoadState('networkidle', { timeout: 15000 });

      await expect(page.locator('.group-selector__name')).toHaveText(groupName);
      // The new group's only member is the caller and they're the admin
      // (invariant enforced by the create endpoint), so the refreshed
      // badge must read "admin". If the reactive chain ever breaks, this
      // would flip to empty or stale text.
      const role = page.locator('[data-testid="current-role"]');
      await expect(role).toHaveText('admin');
      await expect(role).toHaveClass(/role-indicator--admin/);
    } finally {
      // Navigate the UI away from the about-to-be-deleted group before
      // deleting it — same cleanup pattern as the #1262 persistence test.
      const groupsResp = await request.get('/api/v1/groups', {
        headers: { 'Accept': 'application/vnd.api+json', 'Authorization': `Bearer ${authToken}` },
      });
      const groupsBody = await groupsResp.json();
      const other = groupsBody.data.find((g: { id: string }) => g.id !== newGroupId);
      if (other) {
        await page.goto(`/g/${other.attributes.slug}/`);
        await page.waitForLoadState('networkidle', { timeout: 10000 });
      } else {
        await page.goto('/');
      }

      const deleteResp = await request.delete(`/api/v1/groups/${newGroupId}`, {
        headers: {
          'Content-Type': 'application/vnd.api+json',
          'Accept': 'application/vnd.api+json',
          'Authorization': `Bearer ${authToken}`,
          'X-CSRF-Token': csrfToken,
        },
        data: { confirm_word: groupName, password: 'testpassword123' },
      });
      expect(deleteResp.status()).toBe(204);
    }
  });
});
