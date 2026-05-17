/**
 * E2E for the in-app feedback dialog (#1387).
 *
 * Drives the full flow: open Settings → Help, click the "Send feedback"
 * row, fill in the form, submit, and assert the success toast.
 *
 * The backend defaults to the stub email service in the e2e harness so
 * the POST /api/v1/feedback request goes all the way through the
 * handler + template + stub sender without hitting anything external.
 */
import { test } from '../fixtures/app-fixture.js';
import { expect } from '@playwright/test';
import { navigateWithAuth } from './includes/auth.js';

test.describe('Send feedback dialog (#1387)', () => {
  test('user submits feedback and sees the success toast', async ({ page, recorder }) => {
    await navigateWithAuth(page, '/settings', recorder);
    await expect(page.locator('h1')).toBeVisible();

    // Open the Help section in the settings rail.
    await page.locator('[data-testid="settings-nav-help"]').click();
    await expect(page.locator('[data-testid="section-help"]')).toBeVisible();

    // Click the "Send feedback" row — it opens the FeedbackDialog.
    await page.locator('[data-testid="help-row-feedback"]').click();
    const dialog = page.locator('[data-testid="feedback-dialog"]');
    await expect(dialog).toBeVisible();
    await recorder.takeScreenshot('feedback-01-dialog-open');

    // Pick the "Bug" radio chip and type a message.
    await dialog.locator('[data-testid="feedback-type-bug"]').click();
    await dialog
      .locator('[data-testid="feedback-message"]')
      .fill('E2E: report from the playwright fixture user.');

    // Submit. The stub email service accepts the request synchronously
    // and the dialog should close on the 202 response.
    await dialog.locator('[data-testid="feedback-submit"]').click();
    await expect(dialog).toBeHidden();

    // Success toast (sonner) confirms the BE accepted the submission.
    // The toast copy is in en/feedback.json → toasts.success.
    await expect(page.getByText('Thanks — your feedback was sent.')).toBeVisible();
    await recorder.takeScreenshot('feedback-02-success-toast');
  });
});
