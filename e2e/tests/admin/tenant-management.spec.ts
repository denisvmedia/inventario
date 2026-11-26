/**
 * E2E coverage for tenant management in the back office (#1037): a
 * platform admin creates a tenant, suspends and reactivates it, and
 * deletes it after typing its slug. A support agent is refused every
 * tenant write.
 *
 * The spec works on a tenant it creates itself. The seeded tenant serves
 * every request of the e2e stack, so suspending or deleting it is refused
 * (422 admin.tenant.serves_this_host) and is covered by the backend tests
 * instead.
 */
import { test, expect } from '@playwright/test';
import waitOn from 'wait-on';
import {
  BACKOFFICE_SUPPORT_CREDENTIALS,
  bearer,
  loginAsOperator,
  operatorApiLogin,
} from '../includes/backoffice.js';
import { BASE_URL } from '../../setup/urls.js';

test.describe('Back-office tenant management (#1037)', () => {
  test.beforeAll(async () => {
    await waitOn({
      resources: [BASE_URL],
      timeout: 15000,
      interval: 250,
      window: 1000,
      tcpTimeout: 1000,
    });
  });

  test('creates, suspends, reactivates and deletes a tenant', async ({ page }) => {
    await loginAsOperator(page);
    await page.goto('/admin/tenants');
    await expect(page.getByTestId('admin-tenants-page')).toBeVisible();

    // Create from the name alone; the server generates the slug.
    const name = `E2E Tenant ${Date.now()}`;
    await page.getByTestId('admin-tenant-create').click();
    await page.getByTestId('admin-tenant-name').fill(name);
    const createResp = page.waitForResponse(
      (r) => r.url().endsWith('/api/v1/admin/tenants') && r.request().method() === 'POST',
    );
    await page.getByTestId('admin-tenant-form-submit').click();
    const created = await createResp;
    expect(created.status(), 'create tenant').toBe(201);
    const tenant = (await created.json()).data as { id: string; slug: string };
    expect(tenant.slug).toMatch(/^e2e-tenant-\d+$/);

    // The new tenant opens on its detail page.
    await expect(page).toHaveURL(new RegExp(`/admin/tenants/${tenant.id}$`));
    const header = page.getByTestId('admin-tenant-header');
    await expect(header.getByRole('heading', { name })).toBeVisible();
    await expect(header.getByText('Active', { exact: true })).toBeVisible();

    // Suspend, with a reason.
    await page.getByTestId('admin-tenant-suspend').click();
    await page.getByTestId('admin-tenant-status-reason').fill('e2e suspension check (#1037)');
    await page.getByTestId('admin-tenant-status-confirm').click();
    await expect(page.getByTestId('admin-tenant-status-dialog')).toBeHidden();
    await expect(header.getByText('Suspended', { exact: true })).toBeVisible();

    // Reactivate.
    await page.getByTestId('admin-tenant-reactivate').click();
    await page.getByTestId('admin-tenant-status-reason').fill('e2e reactivation check (#1037)');
    await page.getByTestId('admin-tenant-status-confirm').click();
    await expect(page.getByTestId('admin-tenant-status-dialog')).toBeHidden();
    await expect(header.getByText('Active', { exact: true })).toBeVisible();

    // Delete: the button stays disabled until the slug is typed exactly.
    await page.getByTestId('admin-tenant-delete').click();
    const confirm = page.getByTestId('admin-tenant-delete-confirm');
    await page.getByTestId('admin-tenant-delete-slug').fill(name);
    await expect(confirm).toBeDisabled();
    await page.getByTestId('admin-tenant-delete-slug').fill(tenant.slug);
    await expect(confirm).toBeEnabled();
    const deleteResp = page.waitForResponse(
      (r) => r.url().includes(`/api/v1/admin/tenants/${tenant.id}`) && r.request().method() === 'DELETE',
    );
    await confirm.click();
    expect((await deleteResp).status(), 'delete tenant').toBe(204);

    await expect(page).toHaveURL(/\/admin\/tenants$/);
    const token = await page.evaluate(() => localStorage.getItem('backoffice_access_token') || '');
    const gone = await page.request.get(`/api/v1/admin/tenants/${tenant.id}`, {
      headers: bearer(token),
    });
    expect(gone.status(), 'deleted tenant').toBe(404);
  });

  test('a support agent may read tenants but not change them', async ({ request }) => {
    const support = await operatorApiLogin(request, BACKOFFICE_SUPPORT_CREDENTIALS);

    const list = await request.get('/api/v1/admin/tenants', { headers: bearer(support.token) });
    expect(list.status()).toBe(200);
    const tenantId = (await list.json()).data[0].id as string;

    const writes = [
      request.post('/api/v1/admin/tenants', {
        headers: { 'Content-Type': 'application/json', ...bearer(support.token) },
        data: { name: 'Support agent tenant' },
      }),
      request.patch(`/api/v1/admin/tenants/${tenantId}`, {
        headers: { 'Content-Type': 'application/json', ...bearer(support.token) },
        data: { name: 'Renamed by support' },
      }),
      request.post(`/api/v1/admin/tenants/${tenantId}/suspend`, {
        headers: { 'Content-Type': 'application/json', ...bearer(support.token) },
        data: { reason: 'e2e role-boundary check (#1037)' },
      }),
      request.delete(`/api/v1/admin/tenants/${tenantId}?confirm_slug=x`, {
        headers: bearer(support.token),
      }),
    ];
    for (const resp of await Promise.all(writes)) {
      expect(resp.status(), resp.url()).toBe(403);
      expect(await resp.text()).toContain('admin.role_required');
    }
  });
});
