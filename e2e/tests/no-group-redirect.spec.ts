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
 * The "drives group creation" test is the lone exception: it tests the UI
 * onboarding flow rather than the backend response, and creating a real
 * group during a parallel test run would mutate orphan's membership and
 * race the redirect tests in sibling browser workers (chromium / firefox /
 * webkit run in parallel against the same backend, so they'd see orphan
 * with one group and skip the /no-group redirect). It keeps its page.route
 * mocks for that reason.
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

  test('NoGroupView drives group creation and returns the user to /', async ({ page }) => {
    // This test stays mocked: a real POST /api/v1/groups would mutate
    // orphan's membership state and race the redirect tests above when the
    // suite runs across browser projects in parallel (chromium, firefox,
    // webkit all share the same backend orphan user). The contract this
    // test guards is "form submit → router takes the user out of /no-group",
    // which is a frontend reaction that doesn't need a real backend write.
    //
    // Under #1592 RootRedirect requires `user.default_group_id` to point at
    // a current membership before sending the user to `/g/<slug>`; the
    // legacy "first group with a slug" fallback is gone. So we also stub
    // GET /auth/me to advertise the freshly created group as the user's new
    // default once the POST has succeeded — which is what the real backend's
    // EnsureDefaultGroup does after a CreateGroup call.
    await page.goto('/');
    await expect(page).toHaveURL(/\/no-group$/);

    const createdGroup = {
      id: 'mock-grp-1261',
      type: 'groups',
      attributes: {
        slug: 'mock-slug-1261',
        name: 'Onboarding Test Group',
        icon: '📦',
        status: 'active',
        group_currency: 'USD',
        created_by: 'mock-user',
        created_at: '2026-04-20T00:00:00Z',
        updated_at: '2026-04-20T00:00:00Z',
      },
    };
    let createSucceeded = false;
    await page.route(GROUPS_URL_GLOB, (route) => {
      const method = route.request().method();
      if (method === 'POST') {
        createSucceeded = true;
        return route.fulfill({
          status: 201,
          contentType: 'application/vnd.api+json',
          body: JSON.stringify({ data: createdGroup }),
        });
      }
      if (method === 'GET') {
        return route.fulfill({
          status: 200,
          contentType: 'application/vnd.api+json',
          body: JSON.stringify({ data: createSucceeded ? [createdGroup] : [] }),
        });
      }
      return route.continue();
    });
    // After the create-group POST succeeds, /auth/me returns the user with
    // default_group_id pointing at the new group. That mirrors the
    // EnsureDefaultGroup behaviour the real backend runs in
    // GroupService.CreateGroup (#1592).
    await page.route('**/api/v1/auth/me', async (route) => {
      const upstream = await route.fetch();
      const body = await upstream.json();
      if (createSucceeded && body && typeof body === 'object') {
        body.default_group_id = createdGroup.id;
      }
      return route.fulfill({
        response: upstream,
        body: JSON.stringify(body),
        contentType: 'application/json',
      });
    });
    // setCurrentGroup loads members for the active group — stub it to avoid
    // hitting the real backend with a mock group id it's never heard of.
    await page.route(`**/api/v1/groups/${createdGroup.id}/members`, (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/vnd.api+json',
        body: JSON.stringify({ data: [] }),
      }),
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
});
