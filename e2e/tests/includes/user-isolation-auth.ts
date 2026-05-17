import { Page, expect, BrowserContext } from '@playwright/test';
import { gotoScoped } from './group-url.js';

/**
 * Test user interface for user isolation testing
 */
export interface TestUser {
  email: string;
  password: string;
  name: string;
  context?: BrowserContext;
  page?: Page;
}

/**
 * Pre-seeded test users for isolation testing
 */
export const SEEDED_TEST_USERS: TestUser[] = [
  {
    email: 'admin@test-org.com',
    password: 'TestPassword123',
    name: 'Test Administrator'
  },
  {
    email: 'user2@test-org.com',
    password: 'TestPassword123',
    name: 'Test User 2'
  }
];

/**
 * Logs in a user with email and password
 */
export async function loginUser(page: Page, email: string, password: string): Promise<void> {
  await page.goto('/login');

  // Check if page contains 'Settings Required' and fail fast if found
  const settingsRequired = page.locator('h2:has-text("Settings Required")');
  if (await settingsRequired.isVisible()) {
    throw new Error('Settings Required page found - test environment not properly configured');
  }

  // Wait for login form to be visible
  await page.waitForSelector('input[type="email"]', { timeout: 10000 });

  // Fill in credentials using the same selectors as working auth.ts
  await page.fill('input[type="email"]', email);
  await page.fill('input[type="password"]', password);

  // Submit the form
  await page.click('button[type="submit"]');

  // Wait for login to complete and redirect (same logic as auth.ts)
  await page.waitForFunction(
    () => {
      return window.location.pathname !== '/login' ||
             (document.querySelector('h1')?.textContent?.includes('Welcome to Inventario') === true);
    },
    { timeout: 10000 }
  );
}

/**
 * Logs out the current user
 */
export async function logoutUser(page: Page): Promise<void> {
  try {
    const userMenu = page.locator('[data-testid="user-menu"]').or(page.locator('button:has-text("Logout")'));

    if (await userMenu.isVisible()) {
      await userMenu.click();

      const logoutButton = page
        .locator('[data-testid="sign-out"]')
        .or(page.locator('button:has-text("Sign out")'))
        .or(page.locator('button:has-text("Logout")'))
        .or(page.locator('a:has-text("Logout")'));
      if (await logoutButton.isVisible()) {
        await logoutButton.click();
      }
    }

    // Wait for logout to complete
    await page.waitForFunction(
      () => window.location.pathname === '/login',
      { timeout: 5000 }
    );
  } catch (error) {
    console.warn('Logout failed or not needed:', error);
  }
}

/**
 * Gets pre-seeded test users for isolation testing
 */
export async function getTestUsers(testName: string, count: number = 2): Promise<TestUser[]> {
  if (count > SEEDED_TEST_USERS.length) {
    throw new Error(`Requested ${count} users but only ${SEEDED_TEST_USERS.length} are available`);
  }
  
  return SEEDED_TEST_USERS.slice(0, count);
}

/**
 * Sets up isolated browser contexts for multiple users
 */
export async function setupUserContexts(browser: any, users: TestUser[]): Promise<TestUser[]> {
  const updatedUsers: TestUser[] = [];
  
  for (const user of users) {
    const context = await browser.newContext();
    const page = await context.newPage();
    
    updatedUsers.push({
      ...user,
      context,
      page
    });
  }
  
  return updatedUsers;
}

/**
 * Logs in all users in their respective contexts in parallel.
 * Each user has its own BrowserContext + Page, so the logins are
 * independent — serialising them ate up to 60s of the 120s test
 * budget on webkit-macos and was the dominant factor in the
 * "Direct URL access" test timeout pattern. Running them
 * concurrently keeps the multi-user setup cost flat at ~one login.
 */
export async function loginAllUsers(users: TestUser[]): Promise<void> {
  await Promise.all(
    users
      .filter((u) => !!u.page)
      .map((u) => loginUser(u.page as Page, u.email, u.password)),
  );
}

/**
 * Cleans up all user contexts
 */
export async function cleanupUserContexts(users: TestUser[]): Promise<void> {
  for (const user of users) {
    if (user.context) {
      await user.context.close();
    }
  }
}

/**
 * Verifies that a user is logged in by checking for authenticated content.
 * After cutover #1423 the React shell renders the user-menu only when
 * authenticated (`AppSidebar` mounted under guarded routes), so its
 * presence is a single, reliable signal.
 */
export async function verifyUserLoggedIn(page: Page): Promise<void> {
  // Skip networkidle (flake-prone on webkit-macos when React Query
  // refetches) and anchor directly on the user-menu testid the Shell
  // renders only when authenticated. 10s is enough for the post-login
  // shell mount across all three browsers.
  const hasUserMenu = await page
    .locator('[data-testid="user-menu"]')
    .isVisible({ timeout: 10000 });
  if (!hasUserMenu) {
    throw new Error('User does not appear to be logged in — no [data-testid="user-menu"] in the shell');
  }

  const hasLoginForm = await page
    .locator('form')
    .filter({ hasText: 'Login' })
    .isVisible({ timeout: 500 });
  if (hasLoginForm) {
    throw new Error('User does not appear to be logged in — login form is visible');
  }
}

/**
 * Verifies that a user is logged out by checking for login form
 */
export async function verifyUserLoggedOut(page: Page): Promise<void> {
  await expect(page.locator('input[type="email"]')).toBeVisible();
  await expect(page.locator('input[type="password"]')).toBeVisible();
}

/**
 * Switches between users by logging out current user and logging in new user
 */
export async function switchUser(page: Page, newUser: TestUser): Promise<void> {
  await logoutUser(page);
  await loginUser(page, newUser.email, newUser.password);
}

/**
 * Creates a commodity as a specific user via the JSON:API endpoints
 * directly. This used to drive the multi-step CommodityFormDialog UI,
 * but on a busy host backend (lots of pre-seeded groups + slow
 * networkidle waits between steps) the form dance routinely pushed
 * the test past Playwright's 120s budget. We don't need the UI
 * exercise here — the contract under test is data isolation between
 * users, which the backend enforces; the API path is what actually
 * carries that guarantee.
 *
 * Flow: pull the user's auth tokens from the page they already
 * logged into, look up their default group + first location/area
 * (creating any missing prereqs via the same API), then POST
 * /api/v1/g/<slug>/commodities. Returns the new commodity's id +
 * unique name so the verifyUser*SeeContent helpers can assert on it.
 */
export async function createCommodityAsUser(user: TestUser, commodityName: string, _description?: string): Promise<{ name: string; id: string }> {
  if (!user.page) {
    throw new Error('User page not available');
  }
  // Suppress unused-var warning while keeping the public signature
  // backwards-compatible — the old UI helper accepted `description`
  // but the React form's BasicsStep doesn't expose a description
  // field, so it was a no-op even there.
  void _description;

  const uniqueCommodityName = `${commodityName}-${Date.now()}`;
  const uniqueShortName = uniqueCommodityName.slice(-20);

  const accessToken = await user.page.evaluate(() => localStorage.getItem('inventario_token') || '');
  const csrfToken = await user.page.evaluate(() => sessionStorage.getItem('inventario_csrf_token') || '');
  if (!accessToken) throw new Error('createCommodityAsUser: user page has no access token in localStorage — was loginUser awaited?');

  const apiHeaders = {
    'Content-Type': 'application/vnd.api+json',
    Accept: 'application/vnd.api+json',
    Authorization: `Bearer ${accessToken}`,
    'X-CSRF-Token': csrfToken,
  } as const;

  // Resolve the user's active group via /api/v1/groups so the helper
  // works for users whose default_group_id is set OR clear (the BE
  // returns the user's group memberships either way).
  const groupsResp = await user.page.request.get('/api/v1/groups', { headers: apiHeaders });
  if (!groupsResp.ok()) {
    throw new Error(`createCommodityAsUser: GET /groups → ${groupsResp.status()} ${await groupsResp.text()}`);
  }
  const groupsText = await groupsResp.text();
  let groupsBody: { data?: Array<{ id: string; attributes: Record<string, unknown> }> };
  try {
    groupsBody = JSON.parse(groupsText);
  } catch (err) {
    throw new Error(`createCommodityAsUser: GET /groups returned non-JSON body (${groupsText.slice(0, 80)}...): ${(err as Error).message}`);
  }
  const group = groupsBody.data?.[0];
  if (!group?.attributes?.slug) {
    throw new Error('createCommodityAsUser: user has no usable group slug');
  }
  const slug = group.attributes.slug as string;
  const groupCurrency = (group.attributes.group_currency as string) || 'USD';
  const apiBase = `/api/v1/g/${encodeURIComponent(slug)}`;

  // Reuse an existing location if any, otherwise create one.
  let locationId: string;
  const locationsResp = await user.page.request.get(`${apiBase}/locations`, { headers: apiHeaders });
  const locationsText = await locationsResp.text();
  let locationsBody: { data?: Array<{ id: string }> };
  try { locationsBody = JSON.parse(locationsText); }
  catch (err) { throw new Error(`createCommodityAsUser: GET /locations non-JSON (${locationsText.slice(0,80)}): ${(err as Error).message}`); }
  if (locationsBody.data?.length > 0) {
    locationId = locationsBody.data[0].id;
  } else {
    const createLoc = await user.page.request.post(`${apiBase}/locations`, {
      headers: apiHeaders,
      data: {
        data: {
          type: 'locations',
          attributes: { name: 'Test Location', address: 'Test Address' },
        },
      },
    });
    if (!createLoc.ok()) {
      throw new Error(`createCommodityAsUser: POST /locations → ${createLoc.status()} ${await createLoc.text()}`);
    }
    locationId = (await createLoc.json()).data.id;
  }

  // Reuse an existing area inside that location, otherwise create one.
  // The flat `/areas` endpoint accepts `?location_id=` to scope the
  // result to a single location (#1473) — without it we'd over-fetch
  // every area in the group and have to filter by `location_id`
  // attribute client-side.
  let areaId: string;
  const areasResp = await user.page.request.get(
    `${apiBase}/areas?location_id=${encodeURIComponent(locationId)}`,
    { headers: apiHeaders },
  );
  const areasText = await areasResp.text();
  let areasBody: { data?: Array<{ id: string }> };
  try { areasBody = JSON.parse(areasText); }
  catch (err) { throw new Error(`createCommodityAsUser: GET /areas non-JSON (${areasText.slice(0,80)}): ${(err as Error).message}`); }
  if (areasBody.data?.length > 0) {
    areaId = areasBody.data[0].id;
  } else {
    const createArea = await user.page.request.post(`${apiBase}/areas`, {
      headers: apiHeaders,
      data: {
        data: {
          type: 'areas',
          attributes: { name: 'Test Area', location_id: locationId },
        },
      },
    });
    if (!createArea.ok()) {
      throw new Error(`createCommodityAsUser: POST /areas → ${createArea.status()} ${await createArea.text()}`);
    }
    areaId = (await createArea.json()).data.id;
  }

  // POST /commodities with the same envelope CommodityFormDialog
  // submits — BE-side schema doesn't care whether the request comes
  // from the React form or curl, only that the required fields are
  // present and self-consistent.
  const createResp = await user.page.request.post(`${apiBase}/commodities`, {
    headers: apiHeaders,
    data: {
      data: {
        type: 'commodities',
        attributes: {
          name: uniqueCommodityName,
          short_name: uniqueShortName,
          type: 'other',
          status: 'in_use',
          area_id: areaId,
          count: 1,
          purchase_date: '2026-01-01',
          original_price: 0,
          original_price_currency: groupCurrency,
          current_price: 0,
          // BE enforces converted_original_price === 0 when the
          // purchase currency matches the group_currency.
          converted_original_price: 0,
          draft: false,
        },
      },
    },
  });
  if (!createResp.ok()) {
    throw new Error(`createCommodityAsUser: POST /commodities → ${createResp.status()} ${await createResp.text()}`);
  }
  const created = await createResp.json();
  return {
    name: uniqueCommodityName,
    id: created.data.id as string,
  };
}

/**
 * Creates a location as a specific user. Post-cutover (#1423) the
 * LocationsListPage drives creation through `LocationFormDialog`; we
 * extract the new id from the POST response (the page never navigates
 * to a /locations/{id}/edit URL — there is no inline Edit button on the
 * card).
 */
export async function createLocationAsUser(user: TestUser, locationName: string, address?: string): Promise<string> {
  if (!user.page) {
    throw new Error('User page not available');
  }

  await gotoScoped(user.page, '/locations');
  await user.page.click('[data-testid="locations-add-button"]');
  await user.page.waitForSelector('[data-testid="location-form-dialog"]');

  await user.page.fill('#location-name', locationName);
  await user.page.fill('#location-address', address ?? '');

  // Same RHF-enabled wait + form.requestSubmit() pattern as the shared
  // includes/locations.ts createLocation helper. Click() through the
  // location-form-submit button reliably drops the `submit` event in a
  // Radix Dialog Portal on webkit-macos (the button is re-painted during
  // dialog mount, the event dispatcher loses the click→submit chain).
  // requestSubmit() synthesises the event directly on the form element.
  const submitButton = user.page.locator('[data-testid="location-form-submit"]');
  await expect(submitButton).toBeEnabled({ timeout: 10000 });

  // Submit + capture the POST response so we get the canonical id without
  // a follow-up DOM lookup. Endpoint paths land on
  // `/api/v1/g/{slug}/locations` after the Location Groups refactor.
  const [createResponse] = await Promise.all([
    user.page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname.endsWith('/locations') &&
        response.request().method() === 'POST' &&
        response.status() === 201,
      { timeout: 30000 },
    ),
    user.page.evaluate(() => {
      const form = document.getElementById('location-form') as HTMLFormElement | null;
      if (!form) throw new Error('location-form not found');
      form.requestSubmit();
    }),
  ]);

  const createBody = await createResponse.json();
  const locationId = createBody?.data?.id;
  if (!locationId) {
    throw new Error(`createLocationAsUser: POST response missing data.id (body: ${JSON.stringify(createBody)})`);
  }

  await user.page.waitForSelector(
    `[data-testid="location-card"][data-location-id="${locationId}"]`,
  );

  return locationId;
}

/**
 * Verifies that a user cannot see specific content. Navigates to the
 * /commodities list filtered by the content as the search query — the
 * default page is sorted alphabetically with a fixed page size, so a
 * fresh commodity with a "U…" prefix routinely lands on page 2 in a
 * non-empty seed and gets a false-negative "not visible" hit on a bare
 * /commodities visit. The search-scoped URL avoids the pagination trap.
 */
export async function verifyUserCannotSeeContent(user: TestUser, contentText: string): Promise<void> {
  if (!user.page) {
    throw new Error('User page not available');
  }

  // Anchor on the actual /commodities response instead of `networkidle`,
  // which on webkit-macos can settle either side of the React Query
  // refetch and produce flaky empty/late renders. Waiting for the
  // GET response means we assert against the rendered output of a
  // known network round-trip, not a polling guess.
  await gotoAndWaitCommodities(user.page, contentText);

  // The list either renders an empty-state ("Nothing here yet") or
  // some other commodity that didn't match — either way the matching
  // text must not be visible anywhere in the rendered page.
  await expect(user.page.locator(`text=${contentText}`)).toHaveCount(0);
}

/**
 * Verifies that a user can see specific content. Same search-scoped
 * navigation as verifyUserCannotSeeContent so the assertion isn't
 * gated on alphabetical pagination.
 */
export async function verifyUserCanSeeContent(user: TestUser, contentText: string): Promise<void> {
  if (!user.page) {
    throw new Error('User page not available');
  }

  await gotoAndWaitCommodities(user.page, contentText);

  await expect(user.page.locator(`text=${contentText}`).first()).toBeVisible();
}

// gotoAndWaitCommodities navigates to the search-scoped commodities
// list and blocks until the underlying GET /commodities lands. This
// replaces the brittle `waitForLoadState('networkidle', {timeout:5000})`
// pattern that races with React Query's refetch on slower runners.
async function gotoAndWaitCommodities(page: Page, contentText: string): Promise<void> {
  const responsePromise = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname.endsWith('/commodities') &&
      response.request().method() === 'GET' &&
      response.status() === 200,
    { timeout: 30000 },
  );
  await gotoScoped(page, `/commodities?q=${encodeURIComponent(contentText)}`);
  await responsePromise;
}

/**
 * Attempts to access a URL and verifies the response
 */
export async function attemptDirectAccess(user: TestUser, url: string, shouldSucceed: boolean = false): Promise<void> {
  if (!user.page) {
    throw new Error('User page not available');
  }

  // Flat data paths (/commodities/<id>, /locations/<id>/edit, …) are
  // rewritten to /g/<slug>/... using the *current* user's slug, so this
  // exercises the isolation guard: user2 hitting user1's commodity id
  // under user2's own scope must 404.
  await gotoScoped(user.page, url);
  // waitForLoadState('networkidle') is flake-prone on webkit-macos —
  // React Query's background refetches can keep the network busy past
  // the 5s budget, dropping us through to the assertion against a
  // partially-rendered page. The subsequent waitForSelector calls
  // already wait up to 30s for the page-level testids that confirm a
  // settled route, so we can rely on those instead of networkidle.

  if (shouldSucceed) {
    // React pages anchor on `data-testid="page-<route>"`. Match any of
    // the URL-shapes that this helper actually exercises (commodity /
    // location / area detail, plus the list pages the user might land on
    // when the URL is a flat data path).
    await user.page.waitForSelector(
      [
        '[data-testid="page-commodity-detail"]',
        '[data-testid="page-location-detail"]',
        '[data-testid="page-area-detail"]',
        '[data-testid="page-commodities"]',
        '[data-testid="page-locations"]',
      ].join(', '),
    );
  } else {
    // React surfaces "resource not found" / "blocked" through one of
    // several stable testids depending on which page the URL resolves
    // to. Match any of them; the asserting test gets the same signal
    // regardless of route.
    await user.page.waitForSelector(
      [
        '[data-testid="page-not-found"]',
        '[data-testid="commodity-detail-not-found"]',
        '[data-testid="commodity-detail-error"]',
        '[data-testid="location-detail-error"]',
        '[data-testid="area-detail-error"]',
      ].join(', '),
    );
  }
}

/**
 * Verifies that search results are properly isolated. Drives
 * /commodities directly with the URL search query so we don't fight
 * default-page alphabetical pagination in a populated DB.
 */
export async function verifySearchIsolation(user: TestUser, searchTerm: string, shouldFind: boolean = false): Promise<void> {
  if (!user.page) {
    throw new Error('User page not available');
  }

  // Same response-anchored wait as verifyUserCan*SeeContent — the
  // `networkidle` heuristic is unreliable on webkit-macos.
  await gotoAndWaitCommodities(user.page, searchTerm);

  if (shouldFind) {
    await expect(user.page.locator(`text=${searchTerm}`).first()).toBeVisible({ timeout: 5000 });
  } else {
    await expect(user.page.locator(`text=${searchTerm}`)).toHaveCount(0);
  }
}
