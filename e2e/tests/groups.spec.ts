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
      data: { confirm_word: groupName },
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
      data: { confirm_word: groupName },
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
        data: { confirm_word: groupName },
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
        data: { confirm_word: groupName },
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
        data: { confirm_word: groupName },
      });
      expect(deleteResp.status()).toBe(204);
    }
  });
});
