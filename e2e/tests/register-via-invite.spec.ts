/**
 * E2E coverage for the register-via-invite flow (issue #1285, follow-up to
 * #1219 §7 and §11).
 *
 * These tests drive a completely fresh anonymous browser context through
 * /invite/<token> → Register → (auto-login + auto-accept) → group home.
 *
 * Contract under test:
 *   1. Clicking Register on /invite/<token> as an unauthenticated user
 *      persists the token to sessionStorage under `inventario_pending_invite`.
 *   2. RegisterView surfaces the invite banner with the group name so the
 *      invitee knows what they're joining.
 *   3. POST /api/v1/register receives the `invite_token` field in its body.
 *   4. The freshly created account is active (no verification email gating),
 *      is signed in automatically, accepts the invite, and the browser
 *      lands on '/' as a member of the group.
 *
 * The seeded test host runs RegistrationMode=open by default so this test
 * exercises the invite-token path purely for its "skip verification +
 * auto-accept" semantics; the closed-mode bypass is covered by backend
 * unit tests in go/apiserver/registration_invite_test.go.
 */
import { expect, APIRequestContext } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';

// The key the frontend uses for the sessionStorage handoff. Keep in sync
// with frontend/src/services/inviteHandoff.ts — if that changes, this test
// should fail loudly rather than silently skip the assertion.
const HANDOFF_KEY = 'inventario_pending_invite';

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
        attributes: { name, icon: '🎟️' },
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

async function createInvite(
  request: APIRequestContext,
  adminAuth: { accessToken: string; csrfToken: string },
  groupId: string,
): Promise<{ token: string }> {
  const resp = await request.post(`/api/v1/groups/${groupId}/invites`, {
    headers: {
      'Content-Type': 'application/vnd.api+json',
      'Accept': 'application/vnd.api+json',
      'Authorization': `Bearer ${adminAuth.accessToken}`,
      'X-CSRF-Token': adminAuth.csrfToken,
    },
  });
  expect(resp.status(), await resp.text()).toBe(201);
  const body = await resp.json();
  return { token: body.data.attributes.token as string };
}

async function deleteGroup(
  request: APIRequestContext,
  adminAuth: { accessToken: string; csrfToken: string },
  group: { id: string; name: string },
) {
  await request.delete(`/api/v1/groups/${group.id}`, {
    headers: {
      'Content-Type': 'application/vnd.api+json',
      'Accept': 'application/vnd.api+json',
      'Authorization': `Bearer ${adminAuth.accessToken}`,
      'X-CSRF-Token': adminAuth.csrfToken,
    },
    data: { confirm_word: group.name },
  });
}

test.describe('Register via invite (#1285)', () => {
  test('anonymous invitee registers + auto-joins the group', async ({ page, request, browser }) => {
    // Hand-rolled admin bootstrap: reuse the app-fixture's authenticated
    // session to mint the group + invite, then tear down at the end.
    const adminToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const adminCsrf = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');
    expect(adminToken).not.toBe('');
    const adminAuth = { accessToken: adminToken, csrfToken: adminCsrf };

    const group = await createThrowawayGroup(request, adminAuth, 'Register Via Invite');
    const inviteeContext = await browser.newContext();
    const inviteePage = await inviteeContext.newPage();

    try {
      const { token } = await createInvite(request, adminAuth, group.id);

      // Visit the invite landing page as an anonymous user. The page should
      // render the auth-prompt branch (not the authenticated Join branch).
      await inviteePage.goto(`/invite/${token}`);
      await inviteePage.waitForSelector('.invite-card', { state: 'visible', timeout: 10000 });
      await expect(inviteePage.locator('h2', { hasText: `Join ${group.name}` })).toBeVisible();

      const registerLink = inviteePage.locator('a.btn', { hasText: 'Register' });
      await expect(registerLink).toBeVisible();

      // Clicking Register should persist the pending invite and route to
      // /register. We poll sessionStorage via page.evaluate rather than
      // racing on navigation, because the save happens synchronously
      // before the router.push.
      await registerLink.click();
      await inviteePage.waitForURL(/\/register/, { timeout: 5000 });

      const handoff = await inviteePage.evaluate((key) => sessionStorage.getItem(key), HANDOFF_KEY);
      expect(handoff, 'handoff must be persisted to sessionStorage before /register is reached').not.toBeNull();
      const parsed = JSON.parse(handoff as string);
      expect(parsed.token).toBe(token);
      expect(parsed.groupName).toBe(group.name);

      // The invite banner should be rendered on the register form.
      await expect(inviteePage.locator('[data-testid="invite-banner"]')).toBeVisible();
      await expect(inviteePage.locator('[data-testid="invite-banner"]')).toContainText(group.name);

      // Fill and submit the registration form. Use a random email per
      // run — the seeded environment persists across tests and duplicate
      // registration is silently no-op'd to prevent enumeration (which
      // would make the auto-login step fail instead of a clean error).
      const uniqueEmail = `invitee-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@e2e.local`;
      const uniquePassword = 'invitee-Password123';
      const uniqueName = 'Invitee Via Registration';

      await inviteePage.fill('input[data-testid="name"]', uniqueName);
      await inviteePage.fill('input[data-testid="email"]', uniqueEmail);
      await inviteePage.fill('input[data-testid="password"]', uniquePassword);

      // The register POST must carry invite_token so the backend skips the
      // closed-mode gate and email verification. Hook the request before
      // submitting so we can inspect the payload without a race.
      const registerResp = inviteePage.waitForResponse(
        (r) => r.url().includes('/api/v1/register') && r.request().method() === 'POST',
      );
      await inviteePage.click('button[data-testid="register-button"]');
      const registered = await registerResp;
      expect(registered.status(), await registered.text()).toBe(200);

      const registerPayload = JSON.parse(registered.request().postData() || '{}');
      expect(registerPayload.invite_token, 'POST /register body must include invite_token').toBe(token);

      // After register, RegisterView auto-logs the user in and accepts the
      // invite. Wait for the accept response — that's the signal that the
      // handoff dance succeeded end-to-end.
      const acceptResp = await inviteePage.waitForResponse(
        (r) => r.url().includes(`/api/v1/invites/${token}/accept`) && r.request().method() === 'POST',
        { timeout: 15000 },
      );
      expect(acceptResp.status(), await acceptResp.text()).toBe(201);

      // Landing page: router.replace('/') happens after groupStore picks
      // up the new membership. Settle the URL before asserting DOM state.
      await inviteePage.waitForURL((url) => url.pathname === '/', { timeout: 15000 });

      // Authoritative membership check from the admin session.
      const acceptBody = await acceptResp.json();
      const newUserId = acceptBody.data.attributes.member_user_id as string;
      const membersResp = await request.get(`/api/v1/groups/${group.id}/members`, {
        headers: {
          'Accept': 'application/vnd.api+json',
          'Authorization': `Bearer ${adminAuth.accessToken}`,
        },
      });
      expect(membersResp.status()).toBe(200);
      const membership = (await membersResp.json()).data.find(
        (m: { attributes: { member_user_id: string } }) => m.attributes.member_user_id === newUserId,
      );
      expect(membership, 'newly registered user must be a member of the invited group').toBeDefined();
      expect(membership.attributes.role).toBe('user');

      // The handoff entry must be cleared after success — a stale token
      // sitting in sessionStorage would cause unrelated follow-up logins
      // to try (and fail) to accept an already-used invite.
      const afterHandoff = await inviteePage.evaluate((key) => sessionStorage.getItem(key), HANDOFF_KEY);
      expect(afterHandoff, 'sessionStorage entry must be cleared after successful accept').toBeNull();
    } finally {
      await inviteeContext.close();
      await deleteGroup(request, adminAuth, group);
    }
  });
});
