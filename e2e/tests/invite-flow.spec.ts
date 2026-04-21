import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';

test.describe('Invite Flow', () => {

  test('invite page shows group info for valid token', async ({ page, request }) => {
    const authToken = await page.evaluate(() => {
      return localStorage.getItem('inventario_token') || '';
    });
    const csrfToken = await page.evaluate(() => {
      return sessionStorage.getItem('inventario_csrf_token') || '';
    });

    if (!authToken) {
      test.skip();
      return;
    }

    // Get groups
    const groupsResponse = await request.get('/api/v1/groups', {
      headers: {
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
      },
    });

    const groupsBody = await groupsResponse.json();
    if (!groupsBody.data || groupsBody.data.length === 0) {
      test.skip();
      return;
    }

    const groupId = groupsBody.data[0].id;
    const groupName = groupsBody.data[0].attributes.name;

    // Create an invite. This test explicitly validates invite creation, so any
    // non-201 must fail the test rather than be silently skipped (which would
    // hide real regressions like 500 / 403 / 422).
    const inviteResponse = await request.post(`/api/v1/groups/${groupId}/invites`, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
        'X-CSRF-Token': csrfToken,
      },
    });

    expect(inviteResponse.status(), await inviteResponse.text()).toBe(201);

    const inviteBody = await inviteResponse.json();
    const token = inviteBody.data.attributes.token;

    // Navigate to the invite page
    await page.goto(`/invite/${token}`);

    // The .invite-card div is always present — its content switches from
    // "Loading invite..." to either the error branch or the group header
    // once groupService.getInviteInfo resolves. Polling via toContainText
    // waits for that transition instead of racing a textContent read
    // against an in-flight fetch.
    await expect(page.locator('.invite-card')).toContainText(groupName, { timeout: 10000 });

    // Clean up — revoke the invite. Assert 204 so cleanup failures (e.g. 403/422)
    // do not silently leave state behind while reporting a green test.
    const inviteId = inviteBody.data.id;
    const revokeResponse = await request.delete(`/api/v1/groups/${groupId}/invites/${inviteId}`, {
      headers: {
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
        'X-CSRF-Token': csrfToken,
      },
    });
    expect(revokeResponse.status()).toBe(204);
  });

  test('invite page handles invalid token gracefully', async ({ page }) => {
    await page.goto('/invite/this-is-not-a-valid-token');

    // .invite-card renders "Loading invite..." first, then flips to the
    // error branch ("This invite link is not valid.") once the service
    // call rejects. Wait for that terminal text via the auto-retrying
    // expect() rather than a one-shot textContent, which races the
    // in-flight fetch and intermittently reads the loading state.
    await expect(page.locator('.invite-card')).toContainText('not valid', { timeout: 10000 });
  });
});
