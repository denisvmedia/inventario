import { expect } from '@playwright/test';
import { test } from '../fixtures/app-fixture.js';

test.describe('Group Data Isolation', () => {

  test('group-scoped API routes require a valid group slug', async ({ page, request }) => {
    const authToken = await page.evaluate(() => {
      return localStorage.getItem('inventario_token') || '';
    });

    if (!authToken) {
      test.skip();
      return;
    }

    // Try accessing locations with a non-existent group slug
    const response = await request.get('/api/v1/g/nonexistent-slug-12345678/locations', {
      headers: {
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
      },
    });

    // Should get 404 (group not found) or 403 (not a member)
    expect([403, 404]).toContain(response.status());
  });

  test('group-scoped locations endpoint works with valid group', async ({ page, request }) => {
    const authToken = await page.evaluate(() => {
      return localStorage.getItem('inventario_token') || '';
    });

    if (!authToken) {
      test.skip();
      return;
    }

    // Get user's groups
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

    const groupSlug = groupsBody.data[0].attributes.slug;

    // Access locations via group-scoped route
    const locationsResponse = await request.get(`/api/v1/g/${groupSlug}/locations`, {
      headers: {
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
      },
    });

    expect(locationsResponse.status()).toBe(200);

    const locationsBody = await locationsResponse.json();
    expect(locationsBody.data).toBeDefined();
    expect(Array.isArray(locationsBody.data)).toBeTruthy();
  });

  test('group-scoped commodities endpoint works with valid group', async ({ page, request }) => {
    const authToken = await page.evaluate(() => {
      return localStorage.getItem('inventario_token') || '';
    });

    if (!authToken) {
      test.skip();
      return;
    }

    // Get user's groups
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

    const groupSlug = groupsBody.data[0].attributes.slug;

    // Access commodities via group-scoped route
    const response = await request.get(`/api/v1/g/${groupSlug}/commodities`, {
      headers: {
        'Accept': 'application/vnd.api+json',
        'Authorization': `Bearer ${authToken}`,
      },
    });

    expect(response.status()).toBe(200);
  });
});
