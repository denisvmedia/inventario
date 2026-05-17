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
import { gotoScoped } from './includes/group-url.js';

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

      // User 1 creates a full-database export through the React wizard.
      // Step 1's default scope is `full_database` (initial WizardState in
      // ExportNewPage); we just click Next, fill the description on step 2,
      // and submit. No conditional skip: if the feature regresses, the
      // test fails loudly.
      await gotoScoped(user1.page!, '/exports/new');
      await user1.page!.getByTestId('wizard-step-1-content').waitFor({ state: 'visible', timeout: 10000 });
      await user1.page!.getByTestId('wizard-next').click();
      await user1.page!.getByTestId('wizard-step-2-content').waitFor({ state: 'visible' });
      await user1.page!.getByTestId('wizard-description').fill(exportDescription);
      // Wait for wizard-submit to be enabled — RHF/zod's validation
      // resolver runs in a microtask after `fill`, so on webkit-macos
      // the click can land while the button is still disabled. The
      // click is then absorbed silently, no POST fires, and the
      // following `waitForURL` times out at 30s with us still on
      // /exports/new.
      const submitBtn = user1.page!.getByTestId('wizard-submit');
      await expect(submitBtn).toBeEnabled({ timeout: 10000 });
      // Pair the click with a waitForResponse on the actual POST /exports
      // request so we observe the network round-trip directly instead of
      // racing the post-success navigation against React Router.
      const exportResponsePromise = user1.page!.waitForResponse(
        (response) =>
          new URL(response.url()).pathname.endsWith('/exports') &&
          response.request().method() === 'POST' &&
          response.status() >= 200 && response.status() < 300,
        { timeout: 30000 },
      );
      await submitBtn.click();
      await exportResponsePromise;
      // Landing on the detail page proves the backend accepted the export.
      await user1.page!.waitForURL(/\/exports\/[0-9a-fA-F-]{36}/, { timeout: 30000 });

      // User 2 must not see User 1's export on the shared exports list.
      // Anchor on the actual GET /exports response (same response-anchored
      // pattern verifyUser*SeeContent uses for /commodities) instead of
      // `networkidle`, which is flake-prone on webkit-macos.
      const u2ExportsResp = user2.page!.waitForResponse(
        (response) =>
          new URL(response.url()).pathname.endsWith('/exports') &&
          response.request().method() === 'GET' &&
          response.status() === 200,
        { timeout: 30000 },
      );
      await gotoScoped(user2.page!, '/exports');
      await u2ExportsResp;
      await user2.page!.getByTestId('page-exports').waitFor({ state: 'visible', timeout: 10000 });
      await expect(user2.page!.locator(`text=${exportDescription}`)).toHaveCount(0);

      // Sanity check: User 1 can still see their own export.
      const u1ExportsResp = user1.page!.waitForResponse(
        (response) =>
          new URL(response.url()).pathname.endsWith('/exports') &&
          response.request().method() === 'GET' &&
          response.status() === 200,
        { timeout: 30000 },
      );
      await gotoScoped(user1.page!, '/exports');
      await u1ExportsResp;
      await user1.page!.getByTestId('page-exports').waitFor({ state: 'visible', timeout: 10000 });
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
      // The backend rewrites the title to `<uniqueBase>-<unix-seconds>` (no
      // .jpg), so match on the unique prefix rather than the full filename.
      const uniqueBase = `user1-isolation-${Date.now()}`;
      const uniqueFileName = `${uniqueBase}.jpg`;
      const fixturePath = path.join('fixtures', 'files', 'image.jpg');

      // User 1 uploads a file via the React Files page upload dialog. No
      // conditional skip: upload is the precondition this test needs, so if
      // it can't happen the test must fail, not silently pass.
      await gotoScoped(user1.page!, '/files');
      await user1.page!.getByTestId('page-files').waitFor({ state: 'visible', timeout: 10000 });
      await user1.page!.getByTestId('files-upload-cta').click();
      await user1.page!.getByTestId('files-upload-dialog').waitFor({ state: 'visible' });
      await user1.page!.getByTestId('files-upload-input').setInputFiles({
        name: uniqueFileName,
        mimeType: 'image/jpeg',
        buffer: fs.readFileSync(fixturePath),
      });
      await user1.page!.getByTestId('files-upload-next').click();
      const [uploadResponse] = await Promise.all([
        user1.page!.waitForResponse(
          (resp) => resp.url().includes('/uploads/file') && resp.request().method() === 'POST',
          { timeout: 30000 }
        ),
        user1.page!.getByTestId('files-upload-start').click(),
      ]);
      expect(uploadResponse.status()).toBe(201);
      // Close the dialog once the upload completes so the post-upload list
      // assertions below aren't shadowed by the modal overlay.
      await user1.page!.getByTestId('files-upload-close').click();
      await user1.page!.getByTestId('files-upload-dialog').waitFor({ state: 'hidden' });

      // User 2 must not see User 1's file on the shared files list.
      await gotoScoped(user2.page!, '/files');
      await user2.page!.waitForLoadState('networkidle', { timeout: 10000 });
      await expect(user2.page!.locator(`text=${uniqueBase}`)).toHaveCount(0);

      // Sanity check: User 1 can still see their own file.
      await gotoScoped(user1.page!, '/files');
      await user1.page!.waitForLoadState('networkidle', { timeout: 10000 });
      await expect(user1.page!.locator(`text=${uniqueBase}`).first()).toBeVisible();

    } finally {
      await cleanupUserContexts(userContexts);
    }
  });
});
