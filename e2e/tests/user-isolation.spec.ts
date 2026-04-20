import { test, expect } from '@playwright/test';
import fs from 'fs';
import path from 'path';
import {
  getTestUsers,
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
} from './includes/user-isolation-auth.js';

test.describe('User Isolation', () => {
  test('Users cannot access each other\'s data', async ({ browser, page }) => {
    // Get pre-seeded test users
    const users = await getTestUsers('basic-isolation', 2);
    const userContexts = await setupUserContexts(browser, users);

    try {
      // Login all users
      await loginAllUsers(userContexts);

      const [user1, user2] = userContexts;

      // User 1 creates a commodity
      const commodity = await createCommodityAsUser(user1, 'User1 Private Item', 'This should not be visible to user2');

      // User 2 should not see User 1's commodity
      await verifyUserCannotSeeContent(user2, commodity.name);

      // User 1 can still see their own commodity
      await verifyUserCanSeeContent(user1, commodity.name);

    } finally {
      await cleanupUserContexts(userContexts);
    }
  });

  test('Direct URL access is blocked for other users data', async ({ browser, page }) => {
    // Get pre-seeded test users
    const users = await getTestUsers('direct-access', 2);
    const userContexts = await setupUserContexts(browser, users);

    try {
      // Login all users
      await loginAllUsers(userContexts);

      const [user1, user2] = userContexts;

      // User 1 creates a commodity
      const commodity = await createCommodityAsUser(user1, 'Protected Commodity');

      // User 2 tries to access User 1's commodity directly
      await attemptDirectAccess(user2, `/commodities/${commodity.id}`, false);

      // User 1 should still be able to access their own commodity
      await attemptDirectAccess(user1, `/commodities/${commodity.id}`, true);

    } finally {
      await cleanupUserContexts(userContexts);
    }
  });

  test('Users can only edit their own data', async ({ browser, page }) => {
    // Get pre-seeded test users
    const users = await getTestUsers('edit-test', 2);
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
    // Get pre-seeded test users
    const users = await getTestUsers('search-test', 2);
    const userContexts = await setupUserContexts(browser, users);

    try {
      // Login all users
      await loginAllUsers(userContexts);

      const [user1, user2] = userContexts;

      // User 1 creates searchable content
      const baseName = 'Unique Search Term 12345';
      const commodity = await createCommodityAsUser(user1, baseName);

      // User 2 searches for User1's content - should not find it
      await verifySearchIsolation(user2, commodity.name, false);

      // User 1 searches for their own content - should find it
      await verifySearchIsolation(user1, commodity.name, true);

    } finally {
      await cleanupUserContexts(userContexts);
    }
  });

  test('Export functionality is isolated between users', async ({ browser }) => {
    // Get pre-seeded test users
    const users = await getTestUsers('export-test', 2);
    const userContexts = await setupUserContexts(browser, users);

    try {
      // Login all users
      await loginAllUsers(userContexts);

      const [user1, user2] = userContexts;

      // Unique description so the assertion can't collide with exports that
      // other tests / prior runs may have left behind on shared fixtures.
      const exportDescription = `User1 Private Export ${Date.now()}`;

      // User 1 creates a full-database export through the real form. No
      // conditional skip: if the feature is missing or the form selectors
      // regress, the test fails loudly — that's the point.
      await user1.page!.goto('/exports/new');
      await user1.page!.waitForSelector('h1:has-text("Create New Export")', { timeout: 10000 });
      await user1.page!.fill('#description', exportDescription);
      await user1.page!.click('.p-select[id="type"]');
      await user1.page!.click('.p-select-option-label:has-text("Full Database")');
      await user1.page!.click('button[type="submit"]:has-text("Create Export")');
      // Landing on the detail page proves the backend accepted the export.
      await user1.page!.waitForURL(/\/exports\/[0-9a-fA-F-]{36}/, { timeout: 30000 });

      // User 2 must not see User 1's export on the shared exports list.
      await user2.page!.goto('/exports');
      await user2.page!.waitForLoadState('networkidle', { timeout: 10000 });
      await expect(user2.page!.locator(`text=${exportDescription}`)).toHaveCount(0);

      // Sanity check: User 1 can still see their own export.
      await user1.page!.goto('/exports');
      await user1.page!.waitForLoadState('networkidle', { timeout: 10000 });
      await expect(user1.page!.locator(`text=${exportDescription}`).first()).toBeVisible();

    } finally {
      await cleanupUserContexts(userContexts);
    }
  });

  test('File uploads are isolated between users', async ({ browser }) => {
    // Get pre-seeded test users
    const users = await getTestUsers('file-test', 2);
    const userContexts = await setupUserContexts(browser, users);

    try {
      // Login all users
      await loginAllUsers(userContexts);

      const [user1, user2] = userContexts;

      // Upload the shared fixture under a unique name so the assertion has a
      // stable identifier that can't collide with anything else in the DB.
      const uniqueFileName = `user1-isolation-${Date.now()}.jpg`;
      const fixturePath = path.join('fixtures', 'files', 'image.jpg');

      // User 1 uploads a file via the real uploader. No conditional skip:
      // upload is the precondition this test needs, so if it can't happen the
      // test must fail, not silently pass.
      await user1.page!.goto('/files/create');
      await user1.page!.waitForSelector('h1:has-text("Upload Files")', { timeout: 10000 });
      await user1.page!.setInputFiles('input.file-input', {
        name: uniqueFileName,
        mimeType: 'image/jpeg',
        buffer: fs.readFileSync(fixturePath)
      });
      await user1.page!.click('.upload-actions button:has-text("Upload File")');
      // Landing on the file detail page proves the upload + entity create succeeded.
      await user1.page!.waitForURL(/\/files\/[0-9a-fA-F-]{36}/, { timeout: 30000 });

      // User 2 must not see User 1's file on the shared files list.
      await user2.page!.goto('/files');
      await user2.page!.waitForLoadState('networkidle', { timeout: 10000 });
      await expect(user2.page!.locator(`text=${uniqueFileName}`)).toHaveCount(0);

      // Sanity check: User 1 can still see their own file.
      await user1.page!.goto('/files');
      await user1.page!.waitForLoadState('networkidle', { timeout: 10000 });
      await expect(user1.page!.locator(`text=${uniqueFileName}`).first()).toBeVisible();

    } finally {
      await cleanupUserContexts(userContexts);
    }
  });
});
