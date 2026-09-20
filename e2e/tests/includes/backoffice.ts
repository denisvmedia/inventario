/**
 * Back-office (platform-operator) plane helpers (#1785 / #2100).
 *
 * The back-office plane is deliberately isolated from the tenant plane:
 * its own login URL (`/backoffice/login`), its own endpoint prefix
 * (`/api/v1/backoffice/auth/*`), `aud=backoffice` tokens carrying an
 * `admin_id` claim, and its own localStorage key. A tenant token is
 * rejected at `/api/v1/admin/*` and vice versa, so specs that drive the
 * admin surface must go through these helpers rather than `login()`.
 */
import { expect, type APIRequestContext, type Page } from '@playwright/test';

/**
 * Platform-admin operator. Seeded by debug/seeddata (gated behind
 * INVENTARIO_SEED_BACKOFFICE_FIXTURE, which setup-stack.ts sets) with
 * MFA disabled — a browser suite cannot enrol TOTP, and an
 * `mfa_enforced=true` row fails login closed with 501.
 *
 * platform_admin is the only role allowed to start an impersonation
 * session; see BACKOFFICE_SUPPORT_CREDENTIALS for the other side.
 */
export const BACKOFFICE_OPERATOR_CREDENTIALS = {
  email: 'operator@backoffice.test',
  password: 'TestPassword123',
};

/**
 * Support-agent operator — read-mostly. Exists so the suite can assert
 * that RequirePlatformAdmin refuses it at impersonation start.
 */
export const BACKOFFICE_SUPPORT_CREDENTIALS = {
  email: 'support@backoffice.test',
  password: 'TestPassword123',
};

export type OperatorSession = { token: string; operatorId: string };

/**
 * Authenticate against the back-office API (no browser). Returns the
 * `aud=backoffice` access token and the operator id.
 *
 * Back-office routes are bearer-only — there is no CSRF middleware on
 * `/api/v1/admin/*` — so the token alone credentials every admin call.
 */
export async function operatorApiLogin(
  request: APIRequestContext,
  credentials: { email: string; password: string } = BACKOFFICE_OPERATOR_CREDENTIALS,
): Promise<OperatorSession> {
  const resp = await request.post('/api/v1/backoffice/auth/login', {
    headers: { 'Content-Type': 'application/json' },
    data: credentials,
  });
  // A 501 here means the seeded fixture drifted to mfa_enforced=true;
  // say so rather than letting a missing token fail far downstream.
  expect(
    resp.status(),
    `backoffice login ${credentials.email} (501 = fixture needs MFA enrolment)`,
  ).toBe(200);
  const body = await resp.json();
  expect(body.access_token, `access_token for ${credentials.email}`).toBeTruthy();
  expect(body.user?.id, `operator id for ${credentials.email}`).toBeTruthy();
  return { token: body.access_token, operatorId: body.user.id };
}

/** Sign in as a back-office operator through the real login form. */
export async function loginAsOperator(
  page: Page,
  credentials: { email: string; password: string } = BACKOFFICE_OPERATOR_CREDENTIALS,
): Promise<void> {
  await page.goto('/backoffice/login');
  await expect(page.getByTestId('backoffice-login-page')).toBeVisible();
  await page.getByTestId('backoffice-email').fill(credentials.email);
  await page.getByTestId('backoffice-password').fill(credentials.password);
  const loginResp = page.waitForResponse(
    (r) =>
      r.url().includes('/api/v1/backoffice/auth/login') && r.request().method() === 'POST',
  );
  await page.getByTestId('backoffice-login-button').click();
  expect((await loginResp).status(), 'backoffice login').toBe(200);
  // The page navigates to the ?redirect= target, defaulting to the
  // back-office landing.
  await expect(page.getByTestId('admin-shell-top-bar')).toBeVisible();
}

/** Read the back-office access token the frontend stashed after login. */
export async function operatorPageToken(page: Page): Promise<string> {
  return page.evaluate(() => localStorage.getItem('backoffice_access_token') || '');
}

export function bearer(token: string): Record<string, string> {
  return { Authorization: `Bearer ${token}` };
}
