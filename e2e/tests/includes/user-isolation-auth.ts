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
    password: 'testpassword123',
    name: 'Test Administrator'
  },
  {
    email: 'user2@test-org.com',
    password: 'testpassword123',
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
 * Logs in all users in their respective contexts
 */
export async function loginAllUsers(users: TestUser[]): Promise<void> {
  for (const user of users) {
    if (user.page) {
      await loginUser(user.page, user.email, user.password);
    }
  }
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
  await page.waitForLoadState('networkidle', { timeout: 5000 });

  const hasUserMenu = await page
    .locator('[data-testid="user-menu"]')
    .isVisible({ timeout: 3000 });
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
 * Creates a commodity as a specific user
 * Returns an object with the unique commodity name and ID
 */
export async function createCommodityAsUser(user: TestUser, commodityName: string, description?: string): Promise<{ name: string; id: string }> {
  if (!user.page) {
    throw new Error('User page not available');
  }

  // Make commodity name unique by adding timestamp
  const uniqueCommodityName = `${commodityName}-${Date.now()}`;

  // Post-cutover (#1423) the flow is flatter: ensure a Location + Area
  // exist (creating them on the locations page if necessary), then drive
  // the multi-step CommodityFormDialog from /commodities. The dialog's
  // own `commodity-area` select is the source of truth for area binding;
  // we don't need to be on an area-detail page first.
  await gotoScoped(user.page, '/locations');

  // Create a location if none exists.
  const hasLocations = (await user.page.locator('[data-testid="location-card"]').count()) > 0;
  if (!hasLocations) {
    await user.page.click('[data-testid="locations-add-button"]');
    await user.page.waitForSelector('[data-testid="location-form-dialog"]');
    await user.page.fill('#location-name', 'Test Location');
    await user.page.fill('#location-address', '');
    await user.page.click('[data-testid="location-form-submit"]');
    await user.page.waitForSelector('[data-testid="location-card"]:has-text("Test Location")');
  }

  // Create an area if none exists. AreaFormDialog opens via the inline
  // `location-card-add-area` button on the parent location.
  const hasAreas = (await user.page.locator('[data-testid="location-card-area"]').count()) > 0;
  if (!hasAreas) {
    await user.page.locator('[data-testid="location-card-add-area"]').first().click();
    await user.page.waitForSelector('[data-testid="area-form-dialog"]');
    await user.page.fill('#area-name', 'Test Area');
    await user.page.click('[data-testid="area-form-submit"]');
    await user.page.waitForSelector('[data-testid="location-card-area"]:has-text("Test Area")');
  }

  // Capture the area name we'll bind the new commodity to.
  const areaName = (await user.page
    .locator('[data-testid="location-card-area"]')
    .first()
    .innerText()).trim();

  // Drive the multi-step CommodityFormDialog from /commodities.
  await gotoScoped(user.page, '/commodities');
  await user.page.waitForSelector('[data-testid="page-commodities"]');
  await user.page.click('[data-testid="commodities-add-button"]');
  await user.page.waitForSelector('[data-testid="commodity-form-dialog"]');

  // Step 1 — Basics (name, short_name, count, type, area).
  await user.page.fill('#commodity-name', uniqueCommodityName);
  await user.page.fill('#commodity-short-name', uniqueCommodityName);
  await user.page.fill('#commodity-count', '1');
  // The type <option> for "Other" carries an emoji icon prefix in its
  // text — match the option element by partial text and forward its value
  // to selectOption.
  const otherValue = await user.page
    .locator('#commodity-type option', { hasText: 'Other' })
    .first()
    .getAttribute('value');
  if (!otherValue) throw new Error('No <option> matching "Other" inside #commodity-type');
  await user.page.selectOption('#commodity-type', otherValue);
  // Bind to the only known area.
  const areaValue = await user.page
    .locator('#commodity-area option', { hasText: areaName })
    .first()
    .getAttribute('value');
  if (areaValue) {
    await user.page.selectOption('#commodity-area', areaValue);
  }

  // Step 1 → 2 (Purchase) → 3 (Warranty) → 4 (Extras) → 5 (Files) → Submit.
  await user.page.click('[data-testid="commodity-form-next"]'); // basics → purchase
  await user.page.click('[data-testid="commodity-form-next"]'); // purchase → warranty
  await user.page.click('[data-testid="commodity-form-next"]'); // warranty → extras
  await user.page.click('[data-testid="commodity-form-next"]'); // extras → files
  await user.page.click('[data-testid="commodity-form-submit"]');

  // Wait for redirect to the new commodity's detail page.
  await user.page.waitForURL(/\/commodities\/[0-9a-fA-F-]{36}/, { timeout: 10000 });
  const commodityUrl = user.page.url();
  const commodityId = commodityUrl.split('/').pop() || '';

  return {
    name: uniqueCommodityName,
    id: commodityId,
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
    user.page.click('[data-testid="location-form-submit"]'),
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
 * Verifies that a user cannot see specific content
 * Navigates to the area where commodities are displayed to check visibility
 */
export async function verifyUserCannotSeeContent(user: TestUser, contentText: string): Promise<void> {
  if (!user.page) {
    throw new Error('User page not available');
  }

  // Navigate to locations page and try to find the content
  await gotoScoped(user.page, '/locations');
  await user.page.waitForLoadState('networkidle', { timeout: 5000 });

  // Locations list always renders areas inline under each card in
  // React, so we don't need to click into a detail page to surface
  // commodity content — just wait for the locations page to settle, then
  // hop to /commodities for the user-scoped item view.
  const firstLocation = user.page.locator('[data-testid="location-card"]').first();
  if (await firstLocation.isVisible({ timeout: 2000 })) {
    await gotoScoped(user.page, '/commodities');
    await user.page.waitForLoadState('networkidle', { timeout: 5000 });
  }

  // Check that the content is not visible anywhere on the page
  await expect(user.page.locator(`text=${contentText}`)).not.toBeVisible();
}

/**
 * Verifies that a user can see specific content
 * Navigates to the area where commodities are displayed to check visibility
 */
export async function verifyUserCanSeeContent(user: TestUser, contentText: string): Promise<void> {
  if (!user.page) {
    throw new Error('User page not available');
  }

  // Navigate to locations page and try to find the content
  await gotoScoped(user.page, '/locations');
  await user.page.waitForLoadState('networkidle', { timeout: 5000 });

  // Locations list always renders areas inline under each card in
  // React, so we don't need to click into a detail page to surface
  // commodity content — just wait for the locations page to settle, then
  // hop to /commodities for the user-scoped item view.
  const firstLocation = user.page.locator('[data-testid="location-card"]').first();
  if (await firstLocation.isVisible({ timeout: 2000 })) {
    await gotoScoped(user.page, '/commodities');
    await user.page.waitForLoadState('networkidle', { timeout: 5000 });
  }

  await expect(user.page.locator(`text=${contentText}`).first()).toBeVisible();
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
  await user.page.waitForLoadState('networkidle', { timeout: 5000 });

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
 * Verifies that search results are properly isolated
 * This navigates to the area where commodities are displayed to check visibility
 */
export async function verifySearchIsolation(user: TestUser, searchTerm: string, shouldFind: boolean = false): Promise<void> {
  if (!user.page) {
    throw new Error('User page not available');
  }

  // Navigate to locations page
  await gotoScoped(user.page, '/locations');
  await user.page.waitForLoadState('networkidle', { timeout: 5000 });

  // Locations list always renders areas inline under each card in
  // React, so we don't need to click into a detail page to surface
  // commodity content — just wait for the locations page to settle, then
  // hop to /commodities for the user-scoped item view.
  const firstLocation = user.page.locator('[data-testid="location-card"]').first();
  if (await firstLocation.isVisible({ timeout: 2000 })) {
    await gotoScoped(user.page, '/commodities');
    await user.page.waitForLoadState('networkidle', { timeout: 5000 });
  }

  if (shouldFind) {
    // User should be able to see their own content
    await expect(user.page.locator(`text=${searchTerm}`).first()).toBeVisible({ timeout: 5000 });
  } else {
    // User should NOT be able to see other user's content
    const searchResults = user.page.locator(`text=${searchTerm}`);
    await expect(searchResults).not.toBeVisible();
  }
}
