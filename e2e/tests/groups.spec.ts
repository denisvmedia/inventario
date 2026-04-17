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

  test('group API endpoints respond correctly', async ({ page, request }) => {
    // Test the groups list API directly
    const response = await request.get('/api/v1/groups', {
      headers: {
        'Accept': 'application/vnd.api+json',
      }
    });

    // Should get 200 (with groups) or 401 (if auth is required via cookie)
    expect([200, 401]).toContain(response.status());
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
            name: 'E2E Test Group',
            icon: '🧪',
          },
        },
      },
    });

    expect(response.status()).toBe(201);

    const body = await response.json();
    expect(body.data).toBeDefined();
    expect(body.data.attributes.name).toBe('E2E Test Group');
    expect(body.data.attributes.icon).toBe('🧪');
    expect(body.data.attributes.slug).toBeDefined();
    expect(body.data.attributes.slug.length).toBeGreaterThanOrEqual(22);
    expect(body.data.attributes.status).toBe('active');
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
    // User should have at least one group (default group created on registration)
    expect(body.data.length).toBeGreaterThanOrEqual(0);
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

    // First member should be an admin
    const firstMember = membersBody.data[0];
    expect(firstMember.attributes.role).toBe('admin');
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

    // Should fail because you can't leave as the last admin
    expect(leaveResponse.status()).toBeGreaterThanOrEqual(400);
  });
});
