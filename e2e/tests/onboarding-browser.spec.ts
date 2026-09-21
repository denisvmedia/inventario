/**
 * The onboarding loop, driven entirely through the browser against a real
 * backend (#2114 N5/N6).
 *
 * What was already covered, and why that was not enough:
 *
 *   - registration.spec.ts stubs POST /api/v1/register with page.route(), so
 *     it tests the form and the success copy, never the server's answer to a
 *     real registration.
 *   - mailpit-email.spec.ts drives register / verify / forgot / reset through
 *     the public API. It proves the emails are delivered and the tokens work;
 *     it does not touch the forms a user actually fills in.
 *
 * Between them, nobody had ever walked the path a first-time user walks: fill
 * the register form, open the emailed link, sign in, forget the password, open
 * that link, set a new one, sign in again. That is the path #2458 calls
 * "installable and usable from documentation alone", and the one where a
 * regression is most expensive — it is the first thing a stranger does.
 *
 * It is one test, not seven, because every step consumes the previous step's
 * output: a fresh account, a token from an email that only exists because of
 * the registration before it. Splitting it would mean re-registering and
 * re-verifying per test, which is slower and gives the flake more surface.
 * test.step keeps the failure message pointed at the step that broke.
 *
 * Skips when Mailpit is unreachable, like mailpit-email.spec.ts: without it
 * there is no way to read the links, and the dev-mode stack has no SMTP.
 */
import { test, expect } from '@playwright/test';
import {
  MAILPIT_URL,
  extractLink,
  isMailpitReachable,
  waitForEmailTo,
} from './includes/mailpit.js';

let mailpitReachable = false;

test.beforeAll(async ({ request }) => {
  mailpitReachable = await isMailpitReachable(request);
  if (!mailpitReachable) {
    // eslint-disable-next-line no-console
    console.warn(
      `Mailpit not reachable at ${MAILPIT_URL}; onboarding-browser.spec.ts will skip.`,
    );
  }
});

test.beforeEach(async () => {
  test.skip(
    !mailpitReachable,
    `Mailpit not reachable at ${MAILPIT_URL}; skipping the browser onboarding flow.`,
  );
});

// Mailpit's inbox is shared across parallel workers, so every run addresses
// its own recipient and filters by it rather than clearing the mailbox.
function freshEmail(): string {
  return `e2e-onboarding-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

test.describe('Onboarding through the browser', () => {
  // The whole loop is one navigation-heavy pass over two mail round-trips.
  test.slow();

  test('register, verify, sign in, reset the password, sign in again', async ({
    page,
    request,
  }) => {
    const email = freshEmail();
    const firstPassword = 'Password123!';
    const secondPassword = 'BrandNewPassword456!';
    const name = 'Onboarding Browser';

    await test.step('register through the form', async () => {
      await page.goto('/register');
      await page.getByTestId('name').fill(name);
      await page.getByTestId('email').fill(email);
      await page.getByTestId('password').fill(firstPassword);
      // The consent label carries the Terms and Privacy links (#2148), so
      // clicking the label lands on an anchor as often as not.
      await page.getByTestId('terms').click();
      await page.getByTestId('register-button').click();

      await expect(page.getByTestId('register-success')).toBeVisible({ timeout: 15_000 });
    });

    await test.step('verify through the emailed link', async () => {
      const msg = await waitForEmailTo(request, email, {
        subject: /verify your inventario account/i,
      });
      const verifyURL = extractLink(msg.Text, /https?:\/\/\S+\/verify-email\?token=\S+/);

      await page.goto(verifyURL);
      await expect(page.getByTestId('verify-success')).toBeVisible({ timeout: 15_000 });
    });

    await test.step('sign in with the password chosen at registration', async () => {
      await page.goto('/login');
      await page.getByTestId('email').fill(email);
      await page.getByTestId('password').fill(firstPassword);
      await page.getByTestId('login-button').click();

      // A brand-new account belongs to no group, so the router guard lands
      // it on the onboarding view. Reaching it is the proof of a session.
      await expect(page).toHaveURL(/\/no-group$/, { timeout: 15_000 });
      await expect(page.getByTestId('no-group-view')).toBeVisible();
    });

    await test.step('ask for a password reset from the form', async () => {
      // A fresh context rather than a logout click: what is under test is
      // the anonymous forgot-password path, which is how a user who cannot
      // sign in reaches it.
      await page.context().clearCookies();
      await page.goto('/login');
      await page.evaluate(() => {
        try {
          window.localStorage.clear()
        } catch {
          // Private mode / blocked storage — the cookie clear above is what
          // ends the session; this is belt and braces.
        }
      });

      await page.goto('/forgot-password');
      await page.getByTestId('email').fill(email);
      await page.getByTestId('submit-button').click();

      await expect(page.getByTestId('forgot-success')).toBeVisible({ timeout: 15_000 });
    });

    await test.step('set a new password through the emailed link', async () => {
      const msg = await waitForEmailTo(request, email, {
        subject: /reset your inventario password/i,
      });
      const resetURL = extractLink(msg.Text, /https?:\/\/\S+\/reset-password\?token=\S+/);

      await page.goto(resetURL);
      await expect(page.getByTestId('reset-page')).toBeVisible({ timeout: 15_000 });
      await page.getByTestId('password').fill(secondPassword);
      await page.getByTestId('confirm-password').fill(secondPassword);
      await page.getByTestId('submit-button').click();

      await expect(page.getByTestId('reset-success')).toBeVisible({ timeout: 15_000 });
    });

    await test.step('the old password no longer works', async () => {
      await page.goto('/login');
      await page.getByTestId('email').fill(email);
      await page.getByTestId('password').fill(firstPassword);
      await page.getByTestId('login-button').click();

      await expect(page.getByTestId('server-error')).toBeVisible({ timeout: 15_000 });
      await expect(page).toHaveURL(/\/login(\?.*)?$/);
    });

    await test.step('the new password does', async () => {
      await page.goto('/login');
      await page.getByTestId('email').fill(email);
      await page.getByTestId('password').fill(secondPassword);
      await page.getByTestId('login-button').click();

      await expect(page).toHaveURL(/\/no-group$/, { timeout: 15_000 });
      await expect(page.getByTestId('no-group-view')).toBeVisible();
    });
  });
});
