/**
 * E2E tests for issue #1261 — authenticated users with zero groups must be
 * redirected to the `/no-group` onboarding view from every protected route
 * so they never see broken / 403 pages that assume a selected group.
 *
 * The redirect-path tests authenticate as `orphan@test-org.com`, the seeded
 * zero-membership user added in #1277. The earlier iteration of this spec
 * stubbed `GET /api/v1/groups` with `page.route` because the seeded admin
 * can't leave the default group (last-admin invariant) and a freshly
 * registered user requires email verification — neither yields a real
 * zero-group session in the e2e environment. The orphan fixture closes
 * that gap so the redirect tests now exercise the actual backend response
 * path.
 *
 * The two group-creation tests are the exception: they test the UI
 * onboarding flows rather than the backend response, and creating a real
 * group during a parallel test run would mutate orphan's membership and
 * race the redirect tests in sibling browser workers (chromium / firefox /
 * webkit run in parallel against the same backend, so they'd see orphan
 * with one group and skip the /no-group redirect). They keep their
 * page.route mocks for that reason.
 */
import { Page, expect, test } from '@playwright/test';
import waitOn from 'wait-on';
import { login, ORPHAN_TEST_CREDENTIALS } from './includes/auth.js';
import { BASE_URL } from '../setup/urls.js';

const GROUPS_URL_GLOB = '**/api/v1/groups';

async function loginAsOrphan(page: Page): Promise<void> {
  await page.goto('/login');
  await login(page, undefined, ORPHAN_TEST_CREDENTIALS);
  // Post-login: RootRedirect (no default_group_id, no memberships) routes
  // through the router guard, which lands the user on /no-group.
}

function mockGroup(id: string, slug: string, name: string) {
  return {
    id,
    type: 'groups',
    attributes: {
      slug,
      name,
      icon: '📦',
      status: 'active',
      group_currency: 'USD',
      created_by: 'mock-user',
      created_at: '2026-04-20T00:00:00Z',
      updated_at: '2026-04-20T00:00:00Z',
    },
  };
}

// Stubs orphan's first group creation without writing to the backend:
// POST /groups answers 201 with `group`, and GET /groups returns it once
// the POST has succeeded.
//
// Under #1592 RootRedirect requires `user.default_group_id` to point at a
// current membership before sending the user to `/g/<slug>`; the legacy
// "first group with a slug" fallback is gone. So GET /auth/me also
// advertises the new group as the user's default once the POST has
// succeeded, which is what the real backend's EnsureDefaultGroup does in
// GroupService.CreateGroup.
//
// The create refetches /groups and /auth/me, and the page navigates to the
// new group once both have answered. The /groups answer is held until the
// /auth/me one is through, so the new group list reaches the page while
// that navigation is still pending (the lazy dashboard route is loading).
// That is the window in which GroupProvider can pin ?g= on the page the
// user is leaving and send them back to it (#2815).
async function mockFirstGroupCreation(
  page: Page,
  group: ReturnType<typeof mockGroup>,
): Promise<void> {
  let createSucceeded = false;
  let authMeRefetched!: () => void;
  const authMeRefetchedPromise = new Promise<void>((resolve) => {
    authMeRefetched = resolve;
  });
  await page.route(GROUPS_URL_GLOB, async (route) => {
    const method = route.request().method();
    if (method === 'POST') {
      createSucceeded = true;
      return route.fulfill({
        status: 201,
        contentType: 'application/vnd.api+json',
        body: JSON.stringify({ data: group }),
      });
    }
    if (method === 'GET') {
      if (createSucceeded) {
        await authMeRefetchedPromise;
        // Let the page handle the /auth/me answer before this one lands.
        await page.evaluate(() => new Promise((resolve) => setTimeout(resolve, 100)));
      }
      return route.fulfill({
        status: 200,
        contentType: 'application/vnd.api+json',
        body: JSON.stringify({ data: createSucceeded ? [group] : [] }),
      });
    }
    return route.continue();
  });
  await page.route('**/api/v1/auth/me', async (route) => {
    const upstream = await route.fetch();
    const body = await upstream.json();
    const refetchAfterCreate = createSucceeded;
    if (refetchAfterCreate && body && typeof body === 'object') {
      body.default_group_id = group.id;
    }
    await route.fulfill({
      response: upstream,
      body: JSON.stringify(body),
      contentType: 'application/json',
    });
    if (refetchAfterCreate) authMeRefetched();
  });
  // The shell loads members for the active group. Stub it to avoid
  // hitting the real backend with a mock group id it has never heard of.
  await page.route(`**/api/v1/groups/${group.id}/members`, (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/vnd.api+json',
      body: JSON.stringify({ data: [] }),
    }),
  );
}

test.describe('No-group redirects (#1261, real fixture #1277)', () => {
  test.beforeAll(async () => {
    // global-setup already waits for the stack, but other tests may have
    // restarted services in between — re-probe so the first goto doesn't
    // race a still-warming server.
    await waitOn({
      resources: [BASE_URL],
      timeout: 15000,
      interval: 250,
      window: 1000,
      tcpTimeout: 1000,
    });
  });

  test.beforeEach(async ({ page }) => {
    await loginAsOrphan(page);
  });

  for (const target of ['/', '/locations', '/commodities', '/files', '/exports']) {
    test(`zero-group user visiting ${target} is redirected to /no-group`, async ({ page }) => {
      await page.goto(target);
      await expect(page).toHaveURL(/\/no-group$/, { timeout: 10000 });
      await expect(page.locator('[data-testid="no-group-view"]')).toBeVisible();
    });
  }

  test('zero-group user can still reach /profile (exempt route)', async ({ page }) => {
    await page.goto('/profile');
    await expect(page).toHaveURL(/\/profile$/);
    // The guard must not have bounced us to /no-group.
    await expect(page.locator('[data-testid="no-group-view"]')).toHaveCount(0);
  });

  test('zero-group user can still reach /groups/new (exempt route)', async ({ page }) => {
    await page.goto('/groups/new');
    await expect(page).toHaveURL(/\/groups\/new$/);
    await expect(page.locator('[data-testid="no-group-view"]')).toHaveCount(0);
  });

  test('invite-accept route is reachable with zero groups (exempt route)', async ({ page }) => {
    // The invite URL itself is public; with zero groups the guard must not
    // intercept it, or a user accepting their first invite could never get in.
    await page.goto('/invite/definitely-not-a-real-token');
    await expect(page).toHaveURL(/\/invite\//);
    await expect(page.locator('[data-testid="no-group-view"]')).toHaveCount(0);
  });

  test('NoGroupView renders guided onboarding copy', async ({ page }) => {
    await page.goto('/');
    await expect(page).toHaveURL(/\/no-group$/);
    await expect(page.locator('h1')).toContainText('Welcome to Inventario');
    await expect(page.locator('[data-testid="no-group-create-button"]')).toBeVisible();
  });

  test('NoGroupView renders the illustrative stats teaser strip (#1390)', async ({ page }) => {
    // The three teaser cards land above the create-group CTA so the
    // user sees the "tracked inventory" framing before being asked to
    // commit. Numbers are static fixtures — the test only confirms
    // they render, not the exact copy.
    await page.goto('/');
    await expect(page).toHaveURL(/\/no-group$/);
    await expect(page.locator('[data-testid="no-group-teaser"]')).toBeVisible();
    await expect(page.locator('[data-testid="no-group-teaser-items"]')).toBeVisible();
    await expect(page.locator('[data-testid="no-group-teaser-warranties"]')).toBeVisible();
    await expect(page.locator('[data-testid="no-group-teaser-value"]')).toBeVisible();
  });

  test('NoGroupView drives group creation and returns the user to /', async ({ page }) => {
    // This test stays mocked: a real POST /api/v1/groups would mutate
    // orphan's membership state and race the redirect tests above when the
    // suite runs across browser projects in parallel (chromium, firefox,
    // webkit all share the same backend orphan user). The contract this
    // test guards is "form submit → router takes the user out of /no-group",
    // which is a frontend reaction that doesn't need a real backend write.
    await page.goto('/');
    await expect(page).toHaveURL(/\/no-group$/);
    await mockFirstGroupCreation(
      page,
      mockGroup('mock-grp-1261', 'mock-slug-1261', 'Onboarding Test Group'),
    );

    await page.locator('[data-testid="no-group-create-button"]').click();
    await page.locator('[data-testid="no-group-name-input"]').fill('Onboarding Test Group');
    await page.locator('[data-testid="no-group-submit"]').click();

    // After the React cutover (#1404) the dashboard lives under /g/:slug,
    // so the post-create router redirect lands on the new slug rather than
    // the bare "/" the Vue port used. Either is "out of /no-group", which
    // is what this test guards.
    await expect(page).toHaveURL(/\/g\/mock-slug-1261|\/$/, { timeout: 10000 });
    await expect(page.locator('[data-testid="no-group-view"]')).toHaveCount(0);
  });

  test('group switcher "Create new group" lands on the new group (#2815)', async ({ page }) => {
    // Mocked for the same reason as the test above. This path goes through
    // the sidebar switcher to /groups/new, where CreateGroupPage creates the
    // group and navigates to /g/<slug>. The refetched group list must not
    // pin ?g= on /groups/new while that navigation is still pending, or the
    // user ends up back on the form at /groups/new?g=<slug>.
    await page.goto('/');
    await expect(page).toHaveURL(/\/no-group$/);
    await mockFirstGroupCreation(
      page,
      mockGroup('mock-grp-2815', 'mock-slug-2815', 'Switcher Test Group'),
    );

    await page.locator('.group-selector').click();
    await page.getByRole('menuitem', { name: 'Create new group' }).click();
    await expect(page).toHaveURL(/\/groups\/new$/);

    await page.locator('[data-testid="group-name-input"]').fill('Switcher Test Group');
    await page.locator('[data-testid="create-group-submit"]').click();

    await expect(page).toHaveURL(/\/g\/mock-slug-2815$/, { timeout: 10000 });
    await expect(page.locator('[data-testid="create-group-page"]')).toHaveCount(0);
  });
});
