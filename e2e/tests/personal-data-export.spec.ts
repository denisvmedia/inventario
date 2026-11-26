/**
 * E2E coverage for "Download my data" in Account settings (#2177). The scope
 * of the archive, the daily limit, expiry, owner-only access and the restore
 * refusal are covered in go/backup/export/personal_service_test.go and
 * go/apiserver/personal_data_exports_test.go. This spec drives the card end
 * to end: request a copy, wait for the export worker to finish it, download
 * it through the signed link, and check the archive opens with the manifest
 * that names it a personal data export.
 *
 * The server takes one request per user per day. A retry within the same
 * stack finds the card disabled with the copy from the first attempt, so the
 * spec requests only when the button allows it and then works with whatever
 * copy is listed.
 */
import zlib from 'node:zlib';
import fs from 'node:fs';
import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';
import { TEST_CREDENTIALS } from './includes/auth.js';
import { isMailpitReachable, waitForEmailTo } from './includes/mailpit.js';

// firstTarMember reads the name and body of the first member of a tar
// stream: a 512-byte header (name at 0..100, octal size at 124..136), then
// the body.
function firstTarMember(tar: Buffer): { name: string; body: string } {
  const name = tar.subarray(0, 100).toString('utf8').replace(/\0.*$/s, '');
  const size = parseInt(tar.subarray(124, 136).toString('utf8').replace(/\0.*$/s, '').trim(), 8);
  return { name, body: tar.subarray(512, 512 + size).toString('utf8') };
}

test.describe('Download my data (#2177)', () => {
  test('requests a copy, waits for it and downloads it', async ({ page, request }) => {
    test.setTimeout(180_000);
    await page.goto('/settings');
    const card = page.getByTestId('personal-data-export');
    await expect(card).toBeVisible({ timeout: 10_000 });
    await expect(card).toContainText('one copy a day');

    const requestButton = page.getByTestId('personal-data-export-request');
    // The button is disabled while the list loads.
    await expect(card).toHaveAttribute('aria-busy', 'false');

    let requested = false;
    if (await requestButton.isEnabled()) {
      const created = page.waitForResponse(
        (r) => r.url().endsWith('/api/v1/users/me/data-exports') && r.request().method() === 'POST',
      );
      await requestButton.click();
      expect((await created).status()).toBe(201);
      requested = true;
      // The daily limit now holds: the button is disabled and says when the
      // next request is possible.
      await expect(page.getByTestId('personal-data-export-next')).toBeVisible();
      await expect(requestButton).toBeDisabled();
    }

    // The list polls while a copy is being prepared.
    const row = page.getByTestId('personal-data-export-row').first();
    await expect(row).toBeVisible();
    await expect(row).toHaveAttribute('data-status', 'completed', { timeout: 120_000 });
    await expect(row).toContainText('deleted on');

    const [download] = await Promise.all([
      page.waitForEvent('download'),
      row.getByTestId('personal-data-export-download').click(),
    ]);
    expect(download.suggestedFilename()).toMatch(/^inventario-personal-data-\d{4}-\d{2}-\d{2}\.tar\.gz$/);

    const archive = fs.readFileSync(await download.path());
    // gzip magic bytes.
    expect(archive[0]).toBe(0x1f);
    expect(archive[1]).toBe(0x8b);
    const manifest = firstTarMember(zlib.gunzipSync(archive));
    expect(manifest.name).toBe('manifest.json');
    const parsed = JSON.parse(manifest.body);
    expect(parsed.kind).toBe('personal_data_export');
    expect(parsed.restorable).toBe(false);
    expect(parsed.subjectUserId).toBeTruthy();

    // The ready email goes to the account's address when the stack has
    // Mailpit (the docker-compose lane); it links to Account settings.
    if (requested && (await isMailpitReachable(request))) {
      const mail = await waitForEmailTo(request, TEST_CREDENTIALS.email, {
        subject: /data is ready to download/i,
        timeoutMs: 30_000,
      });
      expect(mail.Text).toContain('/settings');
    }
  });
});
