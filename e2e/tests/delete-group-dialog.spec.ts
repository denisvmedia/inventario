/**
 * E2E coverage for the group-deletion Danger Zone dialog (issue #1289 Gap A,
 * spec #1219 §12). The backend contract — "typing the group name AND the
 * current password are both required, wrong password is distinguishable from
 * wrong confirm-word" — is covered at the handler level in
 * go/apiserver/groups_test.go. These tests drive the dialog end-to-end to
 * guard the UX pieces: password input is present, the inline error surfaces
 * specifically on the password field for wrong passwords, and the whole
 * flow actually transitions the group to pending_deletion on the happy path.
 */
import { expect, APIRequestContext, Page } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';

const ADMIN_PASSWORD = 'testpassword123';

async function createThrowawayGroup(
  request: APIRequestContext,
  adminAuth: { accessToken: string; csrfToken: string },
  label: string,
): Promise<{ id: string; name: string; slug: string }> {
  const name = `${label} ${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  const resp = await request.post('/api/v1/groups', {
    headers: {
      'Content-Type': 'application/vnd.api+json',
      'Accept': 'application/vnd.api+json',
      'Authorization': `Bearer ${adminAuth.accessToken}`,
      'X-CSRF-Token': adminAuth.csrfToken,
    },
    data: {
      data: {
        type: 'groups',
        attributes: { name, icon: '🧪' },
      },
    },
  });
  expect(resp.status(), await resp.text()).toBe(201);
  const body = await resp.json();
  return {
    id: body.data.id as string,
    name,
    slug: body.data.attributes.slug as string,
  };
}

async function hardDelete(
  request: APIRequestContext,
  adminAuth: { accessToken: string; csrfToken: string },
  group: { id: string; name: string },
) {
  // Teardown uses the same protocol under test, so it must supply both
  // fields — otherwise the group leaks when a test path short-circuits
  // before calling the UI.
  await request.delete(`/api/v1/groups/${group.id}`, {
    headers: {
      'Content-Type': 'application/vnd.api+json',
      'Accept': 'application/vnd.api+json',
      'Authorization': `Bearer ${adminAuth.accessToken}`,
      'X-CSRF-Token': adminAuth.csrfToken,
    },
    data: { confirm_word: group.name, password: ADMIN_PASSWORD },
  });
}

async function openDangerZone(page: Page, group: { id: string; slug: string }) {
  // The settings page sits under /groups/:groupId/settings — not the
  // group-scoped data routes, so no /g/<slug>/ prefix needed.
  await page.goto(`/groups/${group.id}/settings`);
  await page.waitForSelector('[data-testid="delete-group-open"]', { timeout: 10000 });
  await page.click('[data-testid="delete-group-open"]');
  await page.waitForSelector('[data-testid="delete-confirm-word"]', { state: 'visible', timeout: 5000 });
}

test.describe('Delete-group dialog (#1289 Gap A)', () => {
  test('wrong password surfaces inline error on the password field only', async ({ page, request }) => {
    const adminToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const adminCsrf = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');
    const adminAuth = { accessToken: adminToken, csrfToken: adminCsrf };

    const group = await createThrowawayGroup(request, adminAuth, 'Delete Dialog Wrong Password');

    try {
      await openDangerZone(page, group);

      await page.fill('[data-testid="delete-confirm-word"]', group.name);
      await page.fill('[data-testid="delete-password"]', 'definitely-not-the-real-password');

      const delResp = page.waitForResponse(
        (r) => r.url().includes(`/api/v1/groups/${group.id}`) && r.request().method() === 'DELETE',
      );
      await page.click('[data-testid="delete-group-submit"]');
      const resp = await delResp;
      expect(resp.status()).toBe(422);

      // Password error visible; confirm-word error not.
      await expect(page.locator('[data-testid="delete-password-error"]')).toBeVisible();
      await expect(page.locator('[data-testid="delete-confirm-error"]')).toHaveCount(0);

      // Group still active — nothing was mutated.
      const groupResp = await request.get(`/api/v1/groups/${group.id}`, {
        headers: {
          'Accept': 'application/vnd.api+json',
          'Authorization': `Bearer ${adminAuth.accessToken}`,
        },
      });
      expect(groupResp.status()).toBe(200);
      const body = await groupResp.json();
      expect(body.data.attributes.status).toBe('active');
    } finally {
      await hardDelete(request, adminAuth, group);
    }
  });

  test('correct confirm-word + password transitions the group to pending_deletion', async ({ page, request }) => {
    const adminToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const adminCsrf = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');
    const adminAuth = { accessToken: adminToken, csrfToken: adminCsrf };

    const group = await createThrowawayGroup(request, adminAuth, 'Delete Dialog Happy Path');

    try {
      await openDangerZone(page, group);

      await page.fill('[data-testid="delete-confirm-word"]', group.name);
      await page.fill('[data-testid="delete-password"]', ADMIN_PASSWORD);

      const delResp = page.waitForResponse(
        (r) => r.url().includes(`/api/v1/groups/${group.id}`) && r.request().method() === 'DELETE',
      );
      await page.click('[data-testid="delete-group-submit"]');
      const resp = await delResp;
      expect(resp.status()).toBe(204);

      // Group is now pending_deletion — the slug resolver returns 410 Gone
      // on its data routes, but the ID-based GET (not slug-resolved) still
      // returns the record so admins can see the deletion state. Use that.
      const groupResp = await request.get(`/api/v1/groups/${group.id}`, {
        headers: {
          'Accept': 'application/vnd.api+json',
          'Authorization': `Bearer ${adminAuth.accessToken}`,
        },
      });
      expect(groupResp.status()).toBe(200);
      const body = await groupResp.json();
      expect(body.data.attributes.status).toBe('pending_deletion');
    } finally {
      // Teardown no-op: the group is already in pending_deletion. Trying to
      // delete again returns 410 Gone. Swallow the error — the background
      // worker (once #1214 lands) will eventually purge it.
    }
  });
});
