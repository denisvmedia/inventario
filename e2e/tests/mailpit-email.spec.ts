/**
 * E2E coverage for the transactional email pipeline that delivers into
 * Mailpit (issue #1282). The docker-compose stack wires the app to
 * Mailpit as its SMTP catch-all (EMAIL_PROVIDER=smtp, SMTP_HOST=mailpit),
 * but no spec previously read an email out of it — verification URLs,
 * reset links, welcome + password-changed notifications, and the
 * multipart sanity of outgoing mail were all un-asserted at the wire
 * level. The SMTP sender unit test stubs the protocol with a hand-rolled
 * listener; registration.spec.ts only clicks the UI with fake tokens.
 *
 * This suite closes that gap by driving the real flows — register /
 * verify-email, forgot-password / reset-password — through the public
 * API, fetching the resulting emails from Mailpit, and asserting both
 * the delivered URLs activate accounts / change passwords and the
 * rendered messages carry the expected From/Subject/multipart shape.
 *
 * When Mailpit isn't reachable (dev-mode `npm run stack` starts a host
 * `go run` with the stub provider; the webkit macOS CI lane runs the
 * binary with no SMTP config either), a beforeAll probe flips a module
 * flag and every test skips. In CI this suite runs only on the Linux
 * lane that brings up docker-compose. See MAILPIT_URL override in
 * e2e/tests/includes/mailpit.ts.
 */
import { test, expect, APIRequestContext } from '@playwright/test';
import { BACKEND_URL } from '../setup/urls.js';
import {
  MAILPIT_URL,
  extractLink,
  fetchMessage,
  isMailpitReachable,
  listMessages,
  tokenFromURL,
  waitForEmailTo,
} from './includes/mailpit.js';

// Module-scoped because tests run in parallel and each worker gets its
// own fresh APIRequestContext; the reachability probe only needs to
// happen once per worker, not per test.
let mailpitReachable = false;

test.beforeAll(async ({ request }) => {
  mailpitReachable = await isMailpitReachable(request);
  if (!mailpitReachable) {
    // eslint-disable-next-line no-console
    console.warn(
      `Mailpit not reachable at ${MAILPIT_URL}; mailpit-email.spec.ts will skip. ` +
        `Expected when running without docker-compose (e.g. the webkit macOS lane or dev-mode stub).`,
    );
  }
});

test.beforeEach(async () => {
  test.skip(
    !mailpitReachable,
    `Mailpit not reachable at ${MAILPIT_URL}; skipping email-delivery tests.`,
  );
});

/**
 * Build a fresh unique recipient address per test. Mailpit's inbox is a
 * single global mailbox shared across all parallel workers; scoping each
 * test to its own `To` address means we never call DELETE on the shared
 * inbox (which would race sibling workers and drop their mail) and
 * listMessages+filter-by-To always gives us this test's slice.
 */
function freshEmail(label: string): string {
  return `e2e-mailpit-${label}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

/**
 * Return only the summaries Mailpit has for a specific recipient. Used by
 * the duplicate-register assertion where we need "no further message
 * arrived after the first" — which must not be corrupted by other
 * workers delivering to different addresses in parallel.
 */
async function messagesTo(request: APIRequestContext, email: string): Promise<number> {
  const all = await listMessages(request);
  const target = email.toLowerCase();
  return all.filter((m) => m.To.some((t) => t.Address.toLowerCase() === target)).length;
}

async function registerUser(
  request: APIRequestContext,
  email: string,
  password: string,
  name: string,
): Promise<void> {
  const res = await request.post(`${BACKEND_URL}/api/v1/register`, {
    headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
    data: { email, password, name },
  });
  // Registration always responds 200 even for duplicates (anti-enumeration),
  // so a non-200 here is a real server/shape failure worth surfacing.
  expect(res.status(), await res.text()).toBe(200);
}

async function activateUser(request: APIRequestContext, email: string): Promise<void> {
  const msg = await waitForEmailTo(request, email, {
    subject: /verify your inventario account/i,
  });
  const verifyURL = extractLink(msg.Text, /https?:\/\/\S+\/verify-email\?token=\S+/);
  const token = tokenFromURL(verifyURL);
  const verifyRes = await request.get(
    `${BACKEND_URL}/api/v1/verify-email?token=${encodeURIComponent(token)}`,
  );
  expect(verifyRes.status(), await verifyRes.text()).toBe(200);
}

async function login(
  request: APIRequestContext,
  email: string,
  password: string,
): Promise<{ status: number; body: string }> {
  const res = await request.post(`${BACKEND_URL}/api/v1/auth/login`, {
    headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
    data: { email, password },
  });
  return { status: res.status(), body: await res.text() };
}

test.describe('Mailpit — email delivery', () => {
  test('register → activates user via the emailed verification link', async ({ request, page }) => {
    const email = freshEmail('verify');
    const password = 'Password123!';
    const name = 'Mailpit Verify';

    await registerUser(request, email, password, name);

    const msg = await waitForEmailTo(request, email, {
      subject: /verify your inventario account/i,
    });

    // Links are emitted into the text body one per line by
    // verification.txt.tmpl. The HTML body also contains the link but
    // parsing text is simpler and less brittle across whitespace.
    const verifyURL = extractLink(msg.Text, /https?:\/\/\S+\/verify-email\?token=\S+/);
    expect(verifyURL).toContain('/verify-email?token=');

    // Before verification: login must fail (IsActive=false). The auth
    // endpoint returns 401/403 for inactive accounts; the precise code
    // isn't the thing under test here, only that login is denied.
    const beforeLogin = await login(request, email, password);
    expect(beforeLogin.status).not.toBe(200);

    // Drive the real user flow: navigate the browser to the verification
    // URL. The SPA's /verify-email route reads ?token=X and posts to the
    // API; the page should render the success state.
    await page.goto(verifyURL);
    await expect(page.locator('.status-message.success')).toBeVisible({ timeout: 10_000 });

    // After verification: login must succeed.
    const afterLogin = await login(request, email, password);
    expect(afterLogin.status, afterLogin.body).toBe(200);
  });

  test('verification flow also delivers a welcome email', async ({ request }) => {
    const email = freshEmail('welcome');
    const password = 'Password123!';
    const name = 'Mailpit Welcome';

    await registerUser(request, email, password, name);
    // Activate via the API directly (no browser needed) so this test
    // stays focused on "welcome email gets sent post-activation".
    await activateUser(request, email);

    const welcome = await waitForEmailTo(request, email, {
      subject: /welcome to inventario/i,
    });
    // Text template renders {{.Name}} verbatim; confirm the substitution
    // made it through the queue + renderer + SMTP hop unchanged.
    expect(welcome.Text).toContain(name);
    expect(welcome.HTML).not.toBe('');
  });

  test('forgot-password → reset-password rotates the password and notifies the user', async ({ request }) => {
    const email = freshEmail('reset');
    const oldPassword = 'Password123!';
    const newPassword = 'BrandNewPassword456!';
    const name = 'Mailpit Reset';

    await registerUser(request, email, oldPassword, name);
    await activateUser(request, email);

    // The verification + welcome mails are already in the shared inbox;
    // we don't delete them (that would race parallel workers). Instead,
    // waitForEmailTo's subject filter picks out the specific reset /
    // password-changed messages by rendered Subject below.
    const forgotRes = await request.post(`${BACKEND_URL}/api/v1/forgot-password`, {
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      data: { email },
    });
    expect(forgotRes.status(), await forgotRes.text()).toBe(200);

    const resetMsg = await waitForEmailTo(request, email, {
      subject: /reset your inventario password/i,
    });
    const resetURL = extractLink(resetMsg.Text, /https?:\/\/\S+\/reset-password\?token=\S+/);
    const resetToken = tokenFromURL(resetURL);

    const resetRes = await request.post(`${BACKEND_URL}/api/v1/reset-password`, {
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      data: { token: resetToken, new_password: newPassword },
    });
    expect(resetRes.status(), await resetRes.text()).toBe(200);

    // Old password no longer works.
    const oldAttempt = await login(request, email, oldPassword);
    expect(oldAttempt.status).not.toBe(200);

    // New password works.
    const newAttempt = await login(request, email, newPassword);
    expect(newAttempt.status, newAttempt.body).toBe(200);

    // Password-changed notification arrives. The reset email is already
    // delivered; this is a separate message with a different subject.
    const changed = await waitForEmailTo(request, email, {
      subject: /inventario password was changed/i,
    });
    // The changed-at timestamp is rendered as RFC1123 UTC; just assert
    // the template substitution produced a non-empty body containing
    // the user's name.
    expect(changed.Text).toContain(name);
  });

  test('outgoing mail carries the configured From and both text + HTML parts', async ({ request }) => {
    const email = freshEmail('headers');
    const password = 'Password123!';
    const name = 'Mailpit Headers';

    await registerUser(request, email, password, name);

    const msg = await waitForEmailTo(request, email, {
      subject: /verify your inventario account/i,
    });

    // Subject is rendered from the template's subjectByTemplateType table,
    // so a non-empty match proves the renderer ran. The exact string is
    // already asserted by the subject regex in waitForEmailTo above; the
    // redundant non-empty check guards against Mailpit returning an
    // empty Subject on malformed messages.
    expect(msg.Subject.trim().length).toBeGreaterThan(0);

    // Default FROM when docker-compose env leaves EMAIL_FROM unset is
    // inventario@localhost (see docker-compose.yaml). If deployments
    // override it, the test still passes as long as the From address is
    // non-empty and looks like an address — which is the load-bearing
    // invariant: DKIM/SPF regressions that blank or mangle From would
    // trip this.
    expect(msg.From.Address).toMatch(/^\S+@\S+$/);

    // Both multipart sections must be non-empty. The async renderer
    // produces HTML + Text pairs from embedded templates; a regression
    // that drops one variant would only show up at the wire level and
    // would be invisible to the unit tests (which assert the renderer
    // return value directly, not the transport).
    expect(msg.Text.trim().length).toBeGreaterThan(0);
    expect(msg.HTML.trim().length).toBeGreaterThan(0);
    expect(msg.HTML).toMatch(/<html[\s>]/i);
  });

  test('duplicate registration is silently dropped and only the first message is sent', async ({ request }) => {
    // Exercises the anti-enumeration path: POST /register twice with the
    // same email should return 200 both times (visible) but only send a
    // single verification email (server-internal). Covers the branch
    // handleRegister takes when GetByEmail returns an existing user.
    //
    // Scoped to this test's unique email so the assertion is independent
    // of what parallel workers are delivering for other addresses.
    const email = freshEmail('dup');
    const password = 'Password123!';
    const name = 'Mailpit Duplicate';

    await registerUser(request, email, password, name);

    // Wait for the first email so the comparison below isn't racing the
    // first send.
    await waitForEmailTo(request, email, {
      subject: /verify your inventario account/i,
    });

    // Second register for the same email. The server creates no new
    // user and sends no new email; it returns the same success message.
    await registerUser(request, email, password, name);

    // Observe the mailbox for longer than the email worker queue pop
    // timeout so a delayed, buggy second send cannot slip past this
    // assertion under load.
    const noSecondEmailWindowMs = 2500;
    const pollIntervalMs = 250;
    const deadline = Date.now() + noSecondEmailWindowMs;

    while (Date.now() < deadline) {
      expect(await messagesTo(request, email)).toBe(1);
      await new Promise((r) => setTimeout(r, pollIntervalMs));
    }
    expect(await messagesTo(request, email)).toBe(1);
  });

  test('fetchMessage round-trips a delivered email by its Mailpit ID', async ({ request }) => {
    // Smoke test for the fetchMessage helper: after registration we can
    // round-trip a message ID through GET /api/v1/message/{id} and get
    // the same envelope back. Catches regressions in the helper without
    // relying on a full auth flow.
    const email = freshEmail('smoke');
    await registerUser(request, email, 'Password123!', 'Mailpit Smoke');
    const msg = await waitForEmailTo(request, email);

    const round = await fetchMessage(request, msg.ID);
    expect(round.ID).toBe(msg.ID);
    expect(round.To.some((t) => t.Address.toLowerCase() === email.toLowerCase())).toBe(true);
  });
});
