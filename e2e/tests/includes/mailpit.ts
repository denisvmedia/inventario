/**
 * Helpers for asserting against Mailpit, the local SMTP catch-all the
 * docker-compose stack wires into the app (EMAIL_PROVIDER=smtp,
 * SMTP_HOST=mailpit:1025). The transactional email flows (verification,
 * password reset, password-changed, welcome) all deliver here during e2e
 * runs that bring up the full compose stack.
 *
 * These helpers never touch browser state: they hit Mailpit's HTTP API via
 * Playwright's APIRequestContext so tests can assert on delivered mail
 * directly. When Mailpit isn't reachable (e.g. dev-mode `npm run stack`
 * starts a host-local `go run` backend that defaults to the stub provider,
 * or the webkit macOS lane which runs the binary without SMTP), the spec
 * that uses them probes reachability once in beforeAll and skips all tests.
 *
 * Mailpit API reference: https://mailpit.axllent.org/docs/api-v1/
 */
import { APIRequestContext, expect } from '@playwright/test';

/**
 * Base URL for Mailpit's HTTP API. In docker-compose, the Mailpit UI is
 * published on host port 8025 (see docker-compose.yaml `mailpit` service);
 * override MAILPIT_URL if a different topology is in use.
 */
export const MAILPIT_URL = process.env.MAILPIT_URL ?? 'http://localhost:8025';

/**
 * Envelope summary returned by GET /api/v1/messages. Many fields Mailpit
 * emits are unused here; only the pieces each test actually asserts on are
 * typed.
 */
export interface MailpitSummary {
  ID: string;
  From: { Name: string; Address: string };
  To: { Name: string; Address: string }[];
  Subject: string;
  Created: string;
}

/**
 * Full message body returned by GET /api/v1/message/{id}. Includes both
 * text and HTML parts plus the raw header map — enough to assert multipart
 * structure, subject, and From/Reply-To headers.
 */
export interface MailpitMessage extends MailpitSummary {
  Text: string;
  HTML: string;
  Headers: Record<string, string[]>;
}

/**
 * Probe Mailpit's info endpoint. Returns true when Mailpit responds within
 * the timeout, false otherwise (network error, non-2xx, timeout). Used by
 * specs to decide whether to run or skip the email-delivery tests.
 */
export async function isMailpitReachable(request: APIRequestContext): Promise<boolean> {
  try {
    const res = await request.get(`${MAILPIT_URL}/api/v1/info`, { timeout: 3000 });
    return res.ok();
  } catch {
    return false;
  }
}

/**
 * Delete every message in the Mailpit inbox. Tests call this in beforeEach
 * so `waitForEmailTo` never trips on leftovers from a previous test. The
 * compose `mailpit` service uses in-memory storage (MP_MAX_MESSAGES=500) so
 * the delete is cheap.
 */
export async function clearInbox(request: APIRequestContext): Promise<void> {
  const res = await request.delete(`${MAILPIT_URL}/api/v1/messages`);
  expect(res.ok(), `clearInbox expected 2xx, got ${res.status()}: ${await res.text()}`).toBe(true);
}

/**
 * Poll Mailpit until a message arrives that matches every provided filter.
 * Filters are AND-ed: `to` is required, `subject` is matched as a RegExp
 * when supplied. Polls every 250ms up to `timeoutMs` (default 10s, aligned
 * with the ~seconds delivery latency of the in-memory email queue).
 *
 * Returns the full message (with Text/HTML/Headers) — not just the summary
 * — so callers can extract links, inspect headers, and assert on bodies
 * without a second fetch.
 */
export async function waitForEmailTo(
  request: APIRequestContext,
  to: string,
  opts: { subject?: RegExp; timeoutMs?: number } = {},
): Promise<MailpitMessage> {
  const timeoutMs = opts.timeoutMs ?? 10_000;
  const deadline = Date.now() + timeoutMs;
  const targetAddress = to.toLowerCase();

  let lastError = '';
  while (Date.now() < deadline) {
    const summaries = await listMessages(request);
    const match = summaries.find((msg) => {
      const toHit = msg.To.some((recipient) => recipient.Address.toLowerCase() === targetAddress);
      if (!toHit) return false;
      if (opts.subject && !opts.subject.test(msg.Subject)) return false;
      return true;
    });
    if (match) {
      return await fetchMessage(request, match.ID);
    }
    lastError = `no match for to=${to} subject=${opts.subject?.source ?? '<any>'} among ${summaries.length} messages`;
    await sleep(250);
  }
  throw new Error(`waitForEmailTo timed out after ${timeoutMs}ms: ${lastError}`);
}

/**
 * List all messages currently in Mailpit. Exposed so tests can assert
 * "no second email was sent" without waiting for a timeout — e.g. after
 * a duplicate-registration silent-drop.
 */
export async function listMessages(request: APIRequestContext): Promise<MailpitSummary[]> {
  const res = await request.get(`${MAILPIT_URL}/api/v1/messages`);
  expect(res.ok(), `listMessages expected 2xx, got ${res.status()}: ${await res.text()}`).toBe(true);
  const body = (await res.json()) as { messages?: MailpitSummary[] };
  return body.messages ?? [];
}

/**
 * Fetch a single message by Mailpit ID.
 */
export async function fetchMessage(request: APIRequestContext, id: string): Promise<MailpitMessage> {
  const res = await request.get(`${MAILPIT_URL}/api/v1/message/${id}`);
  expect(res.ok(), `fetchMessage expected 2xx, got ${res.status()}: ${await res.text()}`).toBe(true);
  return (await res.json()) as MailpitMessage;
}

/**
 * Extract the first URL from the text body that matches `pattern`. The
 * rendered text templates (go/services/email_templates/*.txt.tmpl) put
 * each actionable link on its own line, so the matcher just needs to find
 * a contiguous non-whitespace span matching the URL pattern.
 */
export function extractLink(text: string, pattern: RegExp): string {
  const match = text.match(pattern);
  if (!match) {
    throw new Error(`link matching ${pattern.source} not found in body: ${text.slice(0, 400)}`);
  }
  return match[0];
}

/**
 * Extract the `token` query parameter from a URL — used by tests that
 * want to hit the API verify/reset endpoints directly instead of driving
 * the browser.
 */
export function tokenFromURL(url: string): string {
  const parsed = new URL(url);
  const token = parsed.searchParams.get('token');
  if (!token) {
    throw new Error(`url has no token query parameter: ${url}`);
  }
  return token;
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
