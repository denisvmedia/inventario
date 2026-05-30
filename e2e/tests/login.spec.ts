import { expect, test } from '@playwright/test';

/**
 * Login unhappy-path journey (#1038).
 *
 * The happy path + most edge cases are covered by the vitest suite
 * (frontend/src/pages/auth/__tests__/LoginPage.test.tsx). This e2e asserts
 * the one journey that's only meaningful against the real backend: a 401 on
 * POST /api/v1/auth/login surfaces the inline error banner and leaves the
 * user on /login (no navigation, no token).
 *
 * Uses a NON-EXISTENT email on purpose — a wrong password against the seeded
 * admin could trip the account-lock / rate-limit guard and flake the many
 * other specs that log in as admin@test-org.com. An unknown email still
 * returns 401 (the handler records a bad-password login event and rejects)
 * without touching any real account.
 */

test.describe('Login — unhappy paths', () => {
  test('bad credentials show an inline error and keep the user on /login', async ({ page }) => {
    await page.goto('/login');
    await page.getByTestId('email').fill('nobody-1038@test-org.com');
    await page.getByTestId('password').fill('definitely-the-wrong-password');

    const loginResponse = page.waitForResponse(
      (response) => response.url().includes('/api/v1/auth/login'),
      { timeout: 20_000 },
    );
    await page.getByTestId('login-button').click();
    expect((await loginResponse).status()).toBe(401);

    // Inline destructive banner appears and we are NOT redirected.
    await expect(page.getByTestId('server-error')).toBeVisible();
    await expect(page).toHaveURL(/\/login(\?.*)?$/);
    expect(await page.evaluate(() => localStorage.getItem('inventario_token'))).toBeNull();

    // Editing a field clears the stale banner (LoginPage resets serverError
    // on form.watch) — the form is usable again for a retry.
    await page.getByTestId('password').fill('another-attempt');
    await expect(page.getByTestId('server-error')).toBeHidden();
  });
});
