/**
 * E2E coverage for the profile photo (#1382), the issue's acceptance
 * criterion: upload a JPEG on /profile/edit, see it on /profile and in the
 * group's members list, then remove it and get the initials back.
 *
 * The photo is served from a signed avatar_url, so seeing the <img> actually
 * render (naturalWidth > 0) also proves the URL the API handed out is one the
 * signed route serves to this session.
 *
 * Runs as the shared admin. Other specs do not look at the admin's avatar,
 * and the photo is removed through the API before and after the test, so a
 * failure part-way through does not leave it set for the next run.
 */
import path from 'node:path';

import { expect, type APIRequestContext, type Locator } from '@playwright/test';

import { test } from '../fixtures/app-fixture.js';
import { extractApiAuth, resolveActiveGroup, type ApiAuth } from './includes/commodities-api.js';

const photoFixture = path.join('fixtures', 'files', 'image.jpg');

async function removePhotoViaAPI(request: APIRequestContext, auth: ApiAuth): Promise<void> {
  const resp = await request.delete('/api/v1/auth/me/avatar', {
    headers: {
      Authorization: `Bearer ${auth.accessToken}`,
      'X-CSRF-Token': auth.csrfToken,
    },
  });
  // 204 whether or not there was a photo to remove.
  expect(resp.status(), await resp.text()).toBe(204);
}

async function currentUserId(request: APIRequestContext, auth: ApiAuth): Promise<string> {
  const resp = await request.get('/api/v1/auth/me', {
    headers: { Authorization: `Bearer ${auth.accessToken}` },
  });
  expect(resp.ok(), await resp.text()).toBe(true);
  return ((await resp.json()) as { id: string }).id;
}

// expectPhoto waits for the avatar to show a loaded image, not just an <img>
// with a src: Radix renders the image only once the browser has decoded it.
async function expectPhoto(avatar: Locator): Promise<void> {
  const image = avatar.getByTestId('user-avatar-image');
  await expect(image).toBeVisible();
  await expect(image).toHaveAttribute('src', /\/api\/v1\/avatars\//);
  expect(await image.evaluate((img: HTMLImageElement) => img.naturalWidth)).toBeGreaterThan(0);
  await expect(avatar.getByTestId('user-avatar-initials')).toHaveCount(0);
}

async function expectInitials(avatar: Locator): Promise<void> {
  await expect(avatar.getByTestId('user-avatar-initials')).toBeVisible();
  await expect(avatar.getByTestId('user-avatar-image')).toHaveCount(0);
}

test.describe('Profile photo (#1382)', () => {
  test('upload a JPEG, see it on the profile and in the members list, remove it', async ({
    page,
    request,
  }) => {
    const auth = await extractApiAuth(page);
    const group = await resolveActiveGroup(request, auth);
    const memberAvatar = page.getByTestId(`member-avatar-${(await currentUserId(request, auth)).slice(0, 8)}`);
    await removePhotoViaAPI(request, auth);

    try {
      // Upload on the edit page.
      await page.goto('/profile/edit');
      const editAvatar = page.getByTestId('profile-photo-avatar');
      await expectInitials(editAvatar);
      await expect(page.getByTestId('profile-photo-remove')).toHaveCount(0);

      const uploaded = page.waitForResponse(
        (r) => r.url().endsWith('/api/v1/auth/me/avatar') && r.request().method() === 'POST',
      );
      await page.getByTestId('profile-photo-input').setInputFiles(photoFixture);
      expect((await uploaded).status()).toBe(200);
      await expectPhoto(editAvatar);
      await expect(page.getByTestId('profile-photo-remove')).toBeVisible();

      // The sidebar's user menu picks it up without a reload.
      await expectPhoto(page.getByTestId('user-menu-avatar'));

      // The profile page.
      await page.goto('/profile');
      await expectPhoto(page.getByTestId('profile-avatar'));

      // The members list of a group the user is in.
      await page.goto(`/g/${encodeURIComponent(group.slug)}/members`);
      await expectPhoto(memberAvatar);

      // Remove it again from the edit page.
      await page.goto('/profile/edit');
      await page.getByTestId('profile-photo-remove').click();
      const dialog = page.getByTestId('confirm-dialog');
      await expect(dialog).toBeVisible();
      const removed = page.waitForResponse(
        (r) => r.url().endsWith('/api/v1/auth/me/avatar') && r.request().method() === 'DELETE',
      );
      await dialog.getByRole('button', { name: 'Remove', exact: true }).click();
      expect((await removed).status()).toBe(204);
      await expectInitials(page.getByTestId('profile-photo-avatar'));
      await expect(page.getByTestId('profile-photo-remove')).toHaveCount(0);

      // Initials everywhere again.
      await page.goto('/profile');
      await expectInitials(page.getByTestId('profile-avatar'));
      await page.goto(`/g/${encodeURIComponent(group.slug)}/members`);
      await expectInitials(memberAvatar);
    } finally {
      await removePhotoViaAPI(request, auth);
    }
  });
});
