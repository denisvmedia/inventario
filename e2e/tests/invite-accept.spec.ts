/**
 * E2E coverage for the invite *accept* flow (issue #1245).
 *
 * The previously existing e2e specs (invite-flow.spec.ts, groups.spec.ts)
 * exercised invite *creation*, *info lookup*, and *revocation*, but never
 * POST /api/v1/invites/{token}/accept — the core business transition. PR
 * #1224 added the CAS-based MarkUsed path, tenant verification, and the
 * compensating revert on membership failure; those behaviours were locked
 * in at the service level (go/services/group_service_test.go) but the
 * HTTP/browser contract was not.
 *
 * The tests below target the HTTP contract that the frontend depends on,
 * plus one browser-driven happy path that drives InviteAcceptView end to
 * end as a real invitee would. The seeded test environment has exactly
 * one tenant (test-org) with two users — admin@test-org.com (primary,
 * the fixture's logged-in user) and user2@test-org.com (secondary, a
 * member of their own default group but NOT of admin's) — which is
 * enough for every case except "cross-tenant invite". That case's
 * service-level contract ("invite belonging to another tenant responds
 * indistinguishably from 404") is covered here by a fake-token case,
 * since cross-tenant requires a second hostname and is guarded at the
 * service layer by an explicit TenantID comparison unit-tested in
 * group_service_test.go. The "expired invite" case from the issue is
 * intentionally *not* covered here: the public createInvite endpoint
 * always uses the 24h default with no override, so there is no way to
 * fabricate a short-expiry invite end-to-end without a dedicated debug
 * hook. That behaviour is likewise covered at the service layer.
 */
import { expect, APIRequestContext } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';

// user2 is the "second test identity" referenced in issue #1245. The seed
// guarantees it is in the same tenant as admin@test-org.com but NOT a
// member of any group the primary user owns — exactly the state an
// invitee is in.
const USER_B = {
  email: 'user2@test-org.com',
  password: 'testpassword123',
};

interface ApiAuth {
  accessToken: string;
  csrfToken: string;
  userId: string;
}

/**
 * Authenticate user2 via the public /auth/login endpoint and return the
 * credentials required for follow-up API calls. This is deliberately
 * coarser than driving the login form in a second browser context —
 * invitee-as-API is exactly what the concurrent-accept and already-used
 * tests need, and it sidesteps a full context spin-up for those paths.
 */
async function loginAsUserB(request: APIRequestContext): Promise<ApiAuth> {
  const resp = await request.post('/api/v1/auth/login', {
    headers: { 'Content-Type': 'application/json', 'Accept': 'application/json' },
    data: { email: USER_B.email, password: USER_B.password },
  });
  expect(resp.status(), await resp.text()).toBe(200);
  const body = await resp.json();
  return {
    accessToken: body.access_token as string,
    csrfToken: body.csrf_token as string,
    userId: body.user.id as string,
  };
}

/**
 * Create a fresh throwaway group owned by admin, returning the id/name
 * pair callers need for cleanup. Every test creates its own group so
 * parallel runs and repeat invocations don't collide on state, and so
 * admin's seeded default group (which carries the seed data) is never
 * modified.
 */
async function createThrowawayGroup(
  request: APIRequestContext,
  adminAuth: { accessToken: string; csrfToken: string },
  label: string,
): Promise<{ id: string; name: string }> {
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
  return { id: body.data.id as string, name };
}

/**
 * Create a single-use invite for `groupId` as admin and return its
 * public token and db id. The HTTP endpoint always uses a 24h default
 * expiry; there is no knob to shorten it from the caller side.
 */
async function createInvite(
  request: APIRequestContext,
  adminAuth: { accessToken: string; csrfToken: string },
  groupId: string,
): Promise<{ inviteId: string; token: string }> {
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
  return {
    inviteId: body.data.id as string,
    token: body.data.attributes.token as string,
  };
}

async function acceptInviteAs(
  request: APIRequestContext,
  auth: { accessToken: string; csrfToken: string },
  token: string,
) {
  return request.post(`/api/v1/invites/${token}/accept`, {
    headers: {
      'Content-Type': 'application/vnd.api+json',
      'Accept': 'application/vnd.api+json',
      'Authorization': `Bearer ${auth.accessToken}`,
      'X-CSRF-Token': auth.csrfToken,
    },
  });
}

/** Delete a group as admin using the required confirm_word protocol. */
async function deleteGroup(
  request: APIRequestContext,
  adminAuth: { accessToken: string; csrfToken: string },
  group: { id: string; name: string },
) {
  const resp = await request.delete(`/api/v1/groups/${group.id}`, {
    headers: {
      'Content-Type': 'application/vnd.api+json',
      'Accept': 'application/vnd.api+json',
      'Authorization': `Bearer ${adminAuth.accessToken}`,
      'X-CSRF-Token': adminAuth.csrfToken,
    },
    data: { confirm_word: group.name, password: 'testpassword123' },
  });
  expect(resp.status()).toBe(204);
}

test.describe('Invite accept flow (#1245)', () => {

  test('happy path API — user B accepts an invite and becomes a member with role=user', async ({ page, request }) => {
    // Admin creates the invite from the fixture's authenticated session.
    const adminToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const adminCsrf = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');
    expect(adminToken, 'app-fixture must leave admin authenticated').not.toBe('');
    const adminAuth = { accessToken: adminToken, csrfToken: adminCsrf };

    const group = await createThrowawayGroup(request, adminAuth, 'Invite Accept Happy API');

    try {
      const { token } = await createInvite(request, adminAuth, group.id);

      // user2 is a separate authenticated session — log in via the public
      // endpoint to obtain their bearer token (the UI happy-path test below
      // drives the same path via a real browser context; here we stay at
      // the HTTP layer so the contract assertions are unambiguous).
      const userB = await loginAsUserB(request);

      const acceptResp = await acceptInviteAs(request, userB, token);
      expect(acceptResp.status(), await acceptResp.text()).toBe(201);

      // The response body IS the new membership. Role defaults to "user"
      // (admin of the group is the inviter; invitees join as non-admins).
      const acceptBody = await acceptResp.json();
      expect(acceptBody.data.type).toBe('memberships');
      expect(acceptBody.data.attributes.group_id).toBe(group.id);
      expect(acceptBody.data.attributes.member_user_id).toBe(userB.userId);
      expect(acceptBody.data.attributes.role).toBe('user');

      // Cross-check via GET /members so a regression in the accept
      // response body still fails the test — the authoritative state is
      // the membership registry.
      const membersResp = await request.get(`/api/v1/groups/${group.id}/members`, {
        headers: {
          'Accept': 'application/vnd.api+json',
          'Authorization': `Bearer ${adminAuth.accessToken}`,
        },
      });
      expect(membersResp.status()).toBe(200);
      const membersBody = await membersResp.json();
      const newMembership = membersBody.data.find(
        (m: { attributes: { member_user_id: string } }) => m.attributes.member_user_id === userB.userId,
      );
      expect(newMembership, 'user2 must appear in the group\'s member list').toBeDefined();
      expect(newMembership.attributes.role).toBe('user');
    } finally {
      await deleteGroup(request, adminAuth, group);
    }
  });

  test('happy path UI — user B visits /invite/:token and clicks Join', async ({ page, request, browser }) => {
    const adminToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const adminCsrf = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');
    expect(adminToken).not.toBe('');
    const adminAuth = { accessToken: adminToken, csrfToken: adminCsrf };

    const group = await createThrowawayGroup(request, adminAuth, 'Invite Accept Happy UI');

    // Independent browser context for user2 — using the admin's session
    // would flip the assertion to "already a member" because admin owns
    // the group. A fresh context also proves the Join button works even
    // on a completely cold session (no prior snapshot/cache).
    const inviteeContext = await browser.newContext();
    const inviteePage = await inviteeContext.newPage();

    try {
      const { token } = await createInvite(request, adminAuth, group.id);

      // Drive the real login form rather than seeding localStorage so the
      // test exercises the same entry path a real invitee would (log-in
      // happens first, invite link navigation second — the deep-link case
      // where login happens AFTER visiting /invite is covered indirectly
      // because the view preserves the route param through the auth
      // redirect; we keep this test focused on the click-Join transition).
      await inviteePage.goto('/login');
      await inviteePage.waitForSelector('input[type="email"]', { timeout: 10000 });
      await inviteePage.fill('input[type="email"]', USER_B.email);
      await inviteePage.fill('input[type="password"]', USER_B.password);
      const loginResp = inviteePage.waitForResponse(
        (r) => r.url().includes('/api/v1/auth/login') && r.request().method() === 'POST',
      );
      await inviteePage.click('button[type="submit"]');
      const loginOk = await loginResp;
      expect(loginOk.status(), await loginOk.text()).toBe(200);

      // Now the authenticated branch of InviteAcceptView must render the
      // Join Group button (not the "Log in or register" branch).
      await inviteePage.goto(`/invite/${token}`);
      await inviteePage.waitForSelector('.invite-card', { state: 'visible', timeout: 10000 });

      // The React port labels the CTA "Accept invitation" rather than the
      // Vue-era "Join Group"; drive it via the stable testid instead.
      const joinBtn = inviteePage.locator('[data-testid="invite-accept-btn"]');
      await expect(joinBtn).toBeVisible({ timeout: 10000 });

      // The Join handler awaits the POST /accept call, then fetches the
      // group list and calls setCurrentGroup(slug), which does a full
      // reload via groupStore. Wait for the reload so the router has
      // settled before asserting side effects.
      const acceptResp = inviteePage.waitForResponse(
        (r) => r.url().includes(`/api/v1/invites/${token}/accept`) && r.request().method() === 'POST',
      );
      await joinBtn.click();
      const accepted = await acceptResp;
      expect(accepted.status(), await accepted.text()).toBe(201);

      // After accept, the router lands on '/'. A quick assertion on the
      // URL guards against a future change that accidentally leaves the
      // invitee stranded on the invite page after a successful join.
      await inviteePage.waitForURL((url) => !url.pathname.startsWith('/invite/'), { timeout: 15000 });

      // Authoritative membership check via the API from the admin session.
      const userBUserId = (await accepted.json()).data.attributes.member_user_id as string;
      const membersResp = await request.get(`/api/v1/groups/${group.id}/members`, {
        headers: {
          'Accept': 'application/vnd.api+json',
          'Authorization': `Bearer ${adminAuth.accessToken}`,
        },
      });
      expect(membersResp.status()).toBe(200);
      const membership = (await membersResp.json()).data.find(
        (m: { attributes: { member_user_id: string } }) => m.attributes.member_user_id === userBUserId,
      );
      expect(membership, 'user2 should be a member after clicking Join').toBeDefined();
      expect(membership.attributes.role).toBe('user');
    } finally {
      await inviteeContext.close();
      await deleteGroup(request, adminAuth, group);
    }
  });

  test('already-used invite — second accept by same user gets 422', async ({ page, request }) => {
    const adminToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const adminCsrf = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');
    const adminAuth = { accessToken: adminToken, csrfToken: adminCsrf };

    const group = await createThrowawayGroup(request, adminAuth, 'Invite Already Used');

    try {
      const { token } = await createInvite(request, adminAuth, group.id);

      // First accept by user2 must succeed — that's what marks the invite
      // as used and sets up the "already used" state under test.
      const userB = await loginAsUserB(request);
      const firstResp = await acceptInviteAs(request, userB, token);
      expect(firstResp.status(), await firstResp.text()).toBe(201);

      // Second accept with the SAME token by the SAME user now hits the
      // "invite already used" path first, which also maps to 422. That
      // still proves the single-use contract — the invite cannot redeem
      // a second time. The distinct "token is dead even for a fresh
      // user" case is covered by the "revoked invite" test below, which
      // exercises the registry.NotFound branch.
      const secondResp = await acceptInviteAs(request, userB, token);
      expect(secondResp.status(), await secondResp.text()).toBe(422);
    } finally {
      await deleteGroup(request, adminAuth, group);
    }
  });

  test('concurrent accept — two parallel POSTs: one 201, one 422', async ({ page, request }) => {
    const adminToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const adminCsrf = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');
    const adminAuth = { accessToken: adminToken, csrfToken: adminCsrf };

    const group = await createThrowawayGroup(request, adminAuth, 'Invite Concurrent');

    try {
      const { token } = await createInvite(request, adminAuth, group.id);
      const userB = await loginAsUserB(request);

      // Fire both requests *before* awaiting either. This is what drives
      // the MarkUsed CAS: both reads pass the IsUsed check, then only one
      // wins the CAS. The loser gets ErrInviteAlreadyUsed (422). The
      // exact 422-vs-422 distinction between ErrInviteAlreadyUsed and
      // ErrAlreadyMember depends on scheduling, but the external
      // contract — one 201 + one 422 — is deterministic.
      const [r1, r2] = await Promise.all([
        acceptInviteAs(request, userB, token),
        acceptInviteAs(request, userB, token),
      ]);
      const statuses = [r1.status(), r2.status()].sort((a, b) => a - b);
      expect(statuses, `expected [201, 422] but got ${statuses.join(', ')} (bodies: ${await r1.text()} | ${await r2.text()})`).toEqual([201, 422]);

      // The 201 response must carry a real membership; if both requests
      // raced through Create the DB would now have two rows for the same
      // (group, user) pair. Assert the membership list shows exactly one
      // entry for user2 to catch that regression.
      const membersResp = await request.get(`/api/v1/groups/${group.id}/members`, {
        headers: {
          'Accept': 'application/vnd.api+json',
          'Authorization': `Bearer ${adminAuth.accessToken}`,
        },
      });
      expect(membersResp.status()).toBe(200);
      const membersBody = await membersResp.json();
      const myRows = membersBody.data.filter(
        (m: { attributes: { member_user_id: string } }) => m.attributes.member_user_id === userB.userId,
      );
      expect(myRows).toHaveLength(1);
    } finally {
      await deleteGroup(request, adminAuth, group);
    }
  });

  test('revoked invite — accept after revoke returns 404', async ({ page, request }) => {
    const adminToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const adminCsrf = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');
    const adminAuth = { accessToken: adminToken, csrfToken: adminCsrf };

    const group = await createThrowawayGroup(request, adminAuth, 'Invite Revoked');

    try {
      const { inviteId, token } = await createInvite(request, adminAuth, group.id);

      // Revoke hard-deletes the row, so GetByToken on the accept path
      // returns registry.ErrNotFound → 404. This makes the revoke action
      // effective end-to-end, not just a soft-invalidation.
      const revokeResp = await request.delete(`/api/v1/groups/${group.id}/invites/${inviteId}`, {
        headers: {
          'Accept': 'application/vnd.api+json',
          'Authorization': `Bearer ${adminAuth.accessToken}`,
          'X-CSRF-Token': adminAuth.csrfToken,
        },
      });
      expect(revokeResp.status()).toBe(204);

      const userB = await loginAsUserB(request);
      const acceptResp = await acceptInviteAs(request, userB, token);
      expect(acceptResp.status(), await acceptResp.text()).toBe(404);
    } finally {
      await deleteGroup(request, adminAuth, group);
    }
  });

  test('already-a-member — creator accepting their own invite returns 422', async ({ page, request }) => {
    const adminToken = await page.evaluate(() => localStorage.getItem('inventario_token') || '');
    const adminCsrf = await page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');
    const adminAuth = { accessToken: adminToken, csrfToken: adminCsrf };

    // Admin is automatically a member (admin role) of any group they create,
    // so accepting their own invite must hit ErrAlreadyMember — 422.
    const group = await createThrowawayGroup(request, adminAuth, 'Invite Already Member');

    try {
      const { token } = await createInvite(request, adminAuth, group.id);

      const acceptResp = await acceptInviteAs(request, adminAuth, token);
      expect(acceptResp.status(), await acceptResp.text()).toBe(422);

      // The invite must still be unused — a failed "already-member" check
      // runs before the MarkUsed CAS, so the token stays alive and
      // another user could still redeem it. Asserting this guards against
      // a regression that marks the invite consumed prematurely.
      const infoResp = await request.get(`/api/v1/invites/${token}`, {
        headers: { 'Accept': 'application/vnd.api+json' },
      });
      expect(infoResp.status()).toBe(200);
      const infoBody = await infoResp.json();
      expect(infoBody.data.attributes.used).toBe(false);
    } finally {
      await deleteGroup(request, adminAuth, group);
    }
  });

  test('unknown token — POST /accept returns 404 (indistinguishable from cross-tenant)', async ({ request }) => {
    // The service-layer cross-tenant branch returns registry.ErrNotFound
    // on purpose, so the HTTP response is identical to this "token never
    // existed" case. A real cross-tenant test would require a second
    // tenant (= second hostname) which the e2e stack cannot provision;
    // the tenant-verification logic itself is unit-tested in
    // go/services/group_service_test.go. Pinning the 404 here is still
    // valuable: it guards against a change that starts leaking the
    // distinction (e.g. returning 403 for cross-tenant) or that
    // accidentally exposes invite existence to unauthenticated callers.
    const userB = await loginAsUserB(request);

    const acceptResp = await acceptInviteAs(request, userB, 'this-token-does-not-exist-0123456789');
    expect(acceptResp.status(), await acceptResp.text()).toBe(404);
  });

  test('unauthenticated POST /accept is rejected with 401', async ({ request }) => {
    // The accept route is the only invite endpoint behind the
    // userMiddlewares chain (info is public, revoke is admin-only under
    // /groups). Asserting 401 without a bearer locks in that wrapping —
    // otherwise a future refactor that flattens the route definition
    // could silently make accept callable anonymously.
    const resp = await request.post('/api/v1/invites/some-token/accept', {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json',
      },
    });
    expect(resp.status()).toBe(401);
  });
});
