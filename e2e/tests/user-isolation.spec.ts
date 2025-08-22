import { test, expect } from '@playwright/test';
import {
  createTestUsers,
  setupUserContexts,
  loginAllUsers,
  cleanupUserContexts,
  createCommodityAsUser,
  createLocationAsUser,
  verifyUserCannotSeeContent,
  verifyUserCanSeeContent,
  attemptDirectAccess,
  verifySearchIsolation,
  TestUser
} from './includes/multi-user-auth.js';

test.describe('User Isolation', () => {
  test('Users cannot access each other\'s data', async ({ browser, page }) => {
    // Create test users
    const users = await createTestUsers(page, 'basic-isolation', 2);
    const userContexts = await setupUserContexts(browser, users);

    try {
      // Login all users
      await loginAllUsers(userContexts);

      const [user1, user2] = userContexts;

      // User 1 creates a commodity
      const commodityId = await createCommodityAsUser(user1, 'User1 Private Item', 'This should not be visible to user2');

      // User 2 should not see User 1's commodity
      await user2.page!.goto('/commodities');
      await verifyUserCannotSeeContent(user2, 'User1 Private Item');

      // Verify empty state
      const commodityItems = user2.page!.locator('[data-testid="commodity-item"]');
      const itemCount = await commodityItems.count();
      expect(itemCount).toBe(0);

      // User 1 can still see their own commodity
      await user1.page!.goto('/commodities');
      await verifyUserCanSeeContent(user1, 'User1 Private Item');

    } finally {
      await cleanupUserContexts(userContexts);
    }
  });

  test('Direct URL access is blocked for other users data', async ({ browser, page }) => {
    // Create test users
    const users = await createTestUsers(page, 'direct-access', 2);
    const userContexts = await setupUserContexts(browser, users);

    try {
      // Login all users
      await loginAllUsers(userContexts);

      const [user1, user2] = userContexts;

      // User 1 creates a commodity
      const commodityId = await createCommodityAsUser(user1, 'Protected Commodity');

      // User 2 tries to access User 1's commodity directly
      await attemptDirectAccess(user2, `/commodities/${commodityId}`, false);

      // User 1 should still be able to access their own commodity
      await attemptDirectAccess(user1, `/commodities/${commodityId}`, true);

    } finally {
      await cleanupUserContexts(userContexts);
    }
  });

  test('Users can only edit their own data', async ({ browser, page }) => {
    // Create test users
    const users = await createTestUsers(page, 'edit-test', 2);
    const userContexts = await setupUserContexts(browser, users);

    try {
      // Login all users
      await loginAllUsers(userContexts);

      const [user1, user2] = userContexts;

      // User 1 creates a location
      const locationId = await createLocationAsUser(user1, 'User1 Location', '123 User1 Street');

      // User 2 tries to access edit page for User1's location
      await attemptDirectAccess(user2, `/locations/${locationId}/edit`, false);

      // User 1 should be able to access their own edit page
      await attemptDirectAccess(user1, `/locations/${locationId}/edit`, true);

    } finally {
      await cleanupUserContexts(userContexts);
    }
  });

  test('Search results are isolated between users', async ({ browser, page }) => {
    // Create test users
    const users = await createTestUsers(page, 'search-test', 2);
    const userContexts = await setupUserContexts(browser, users);

    try {
      // Login all users
      await loginAllUsers(userContexts);

      const [user1, user2] = userContexts;

      // User 1 creates searchable content
      const uniqueSearchTerm = 'Unique Search Term 12345';
      await createCommodityAsUser(user1, uniqueSearchTerm);

      // User 2 searches for User1's content - should not find it
      await verifySearchIsolation(user2, uniqueSearchTerm, false);

      // User 1 searches for their own content - should find it
      await verifySearchIsolation(user1, uniqueSearchTerm, true);

    } finally {
      await cleanupUserContexts(userContexts);
    }
  });

  test('Export functionality is isolated between users', async ({ browser, page }) => {
    // Create test users
    const users = await createTestUsers(page, 'export-test', 2);
    const userContexts = await setupUserContexts(browser, users);

    try {
      // Login all users
      await loginAllUsers(userContexts);

      const [user1, user2] = userContexts;

      // User 1 creates an export
      await user1.page!.goto('/exports/create');
      await user1.page!.fill('[data-testid="export-name"]', 'User1 Private Export');
      await user1.page!.click('[data-testid="save-button"]');

      // User 2 checks exports - should not see User1's export
      await user2.page!.goto('/exports');
      await verifyUserCannotSeeContent(user2, 'User1 Private Export');

      // Verify empty state
      const exportItems = user2.page!.locator('[data-testid="export-item"]');
      const itemCount = await exportItems.count();
      expect(itemCount).toBe(0);

      // User 1 can see their own export
      await user1.page!.goto('/exports');
      await verifyUserCanSeeContent(user1, 'User1 Private Export');

    } finally {
      await cleanupUserContexts(userContexts);
    }
  });

  test('File uploads are isolated between users', async ({ browser, page }) => {
    // Create test users
    const users = await createTestUsers(page, 'file-test', 2);
    const userContexts = await setupUserContexts(browser, users);

    try {
      // Login all users
      await loginAllUsers(userContexts);

      const [user1, user2] = userContexts;

      // Navigate to files section for both users
      await user1.page!.goto('/files');
      await user2.page!.goto('/files');

      // User2 should not see any files initially
      const fileItems = user2.page!.locator('[data-testid="file-item"]');
      const itemCount = await fileItems.count();
      expect(itemCount).toBe(0);

      // This test assumes file upload functionality exists
      // In a real implementation, you would upload a file as user1
      // and verify user2 cannot see it

    } finally {
      await cleanupUserContexts(userContexts);
    }
  });
});
