/**
 * E2E tests for issue #1261 — authenticated users with zero groups must be
 * redirected to the `/no-group` onboarding view from every protected route
 * so they never see broken / 403 pages that assume a selected group.
 *
 * We simulate the zero-group state by intercepting GET /api/v1/groups and
 * returning an empty collection. Actually leaving the seeded admin's only
 * group is not an option: admin is the sole admin, so the leave endpoint
 * rejects the request, and deleting the group would wipe the default seed
 * data that the rest of the E2E suite depends on.
 */
import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';

const GROUPS_URL_GLOB = '**/api/v1/groups';

async function mockEmptyGroups(page: Parameters<Parameters<typeof test>[2]>[0]['page']) {
  await page.route(GROUPS_URL_GLOB, (route) => {
    if (route.request().method() === 'GET') {
      return route.fulfill({
        status: 200,
        contentType: 'application/vnd.api+json',
        body: JSON.stringify({ data: [] }),
      });
    }
    return route.continue();
  });
}

test.describe('No-group redirects (#1261)', () => {
  test.beforeEach(async ({ page }) => {
    await mockEmptyGroups(page);

    // After #1300 the groupStore doesn't keep a localStorage snapshot, so
    // there's nothing to scrub — a reload is enough to force ensureLoaded()
    // to re-run against the (mocked) empty /api/v1/groups response.
    await page.reload();
    // Wait for the post-reload auth + groups bootstrap to finish before the
    // test-specific navigation, otherwise assertions can race the redirect.
    await page.waitForLoadState('networkidle');
  });

  for (const target of ['/', '/locations', '/commodities', '/files', '/exports', '/system']) {
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
    await page.goto('/');
    await expect(page).toHaveURL(/\/no-group$/);

    // Swap the mock so POST /api/v1/groups "succeeds" and the subsequent GET
    // returns the newly-created group. After that the router guard sees
    // hasGroups=true and lets the user through to /.
    const createdGroup = {
      id: 'mock-grp-1261',
      type: 'groups',
      attributes: {
        slug: 'mock-slug-1261',
        name: 'Onboarding Test Group',
        icon: '📦',
        status: 'active',
        main_currency: 'USD',
        created_by: 'mock-user',
        created_at: '2026-04-20T00:00:00Z',
        updated_at: '2026-04-20T00:00:00Z',
      },
    };
    let createSucceeded = false;
    await page.unroute(GROUPS_URL_GLOB);
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

    await expect(page).toHaveURL(/\/$/, { timeout: 10000 });
    await expect(page.locator('[data-testid="no-group-view"]')).toHaveCount(0);
  });
});
