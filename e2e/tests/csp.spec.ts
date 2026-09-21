/**
 * Content-Security-Policy guard (#2524).
 *
 * A CSP that is too strict does not fail a Go test, a lint, or a type check.
 * It fails in the browser, silently, on whatever the policy forgot — a font, a
 * blob: preview, a <style> some library appends on mount. The only thing that
 * can see it is a browser, so the guard lives here.
 *
 * Two signals, because they catch different things. `securitypolicyviolation`
 * fires on the document for every blocked load, which covers the resources the
 * page itself asks for. Console errors catch what the event misses in some
 * engines, and cost nothing to collect.
 *
 * The surfaces below are the ones an unauthenticated visitor and a signed-in
 * user actually see on a first pass. A page that is not walked here is a page
 * this guard says nothing about — add one rather than assume.
 */
import { test, expect, Page } from '@playwright/test';
import { login } from './includes/auth.js';

declare global {
  interface Window {
    __cspViolations?: string[];
  }
}

/**
 * Collect violations from both signals. The init script runs before any page
 * script, so a violation during the very first paint is still recorded.
 */
async function watchForViolations(page: Page): Promise<() => string[]> {
  const consoleHits: string[] = [];
  page.on('console', (msg) => {
    const text = msg.text();
    if (/content security policy|refused to (load|execute|apply|connect)/i.test(text)) {
      consoleHits.push(`console: ${text}`);
    }
  });

  await page.addInitScript(() => {
    window.__cspViolations = [];
    document.addEventListener('securitypolicyviolation', (event) => {
      window.__cspViolations?.push(
        `${event.violatedDirective} blocked ${event.blockedURI || '(inline)'} on ${event.documentURI}`,
      );
    });
  });

  return () => consoleHits;
}

async function violationsOn(page: Page, consoleHits: () => string[]): Promise<string[]> {
  const fromEvents = await page.evaluate(() => window.__cspViolations ?? []);
  return [...fromEvents, ...consoleHits()];
}

test.describe('Content-Security-Policy', () => {
  test('the public surfaces load without a violation', async ({ page }) => {
    const consoleHits = await watchForViolations(page);

    for (const path of ['/', '/login', '/register', '/forgot-password']) {
      await page.goto(path);
      // Wait for the app to have mounted, not just for the document: the
      // styles that libraries inject arrive with the first render.
      await page.waitForLoadState('networkidle');
      expect(await violationsOn(page, consoleHits), `CSP violations on ${path}`).toEqual([]);
    }
  });

  test('the signed-in surfaces load without a violation', async ({ page }) => {
    const consoleHits = await watchForViolations(page);

    // Straight to /login: every test gets a fresh context, so the browser is
    // not signed in and the form renders. (Routing through "/" and logging out
    // first cost two minutes on WebKit waiting for a user menu that a
    // signed-out page never shows.)
    await page.goto('/login');
    await login(page);
    await page.waitForLoadState('networkidle');

    expect(await violationsOn(page, consoleHits), 'CSP violations after sign-in').toEqual([]);
  });

  test('the policy is actually being sent', async ({ page }) => {
    // Without this the two tests above pass loudest when the header is
    // missing entirely, which is the one result they must not report as good.
    const response = await page.goto('/login');
    expect(response, 'no response for /login').not.toBeNull();
    const policy = response?.headers()['content-security-policy'] ?? '';
    expect(policy, 'Content-Security-Policy header missing').not.toEqual('');
    expect(policy).toContain("frame-ancestors 'none'");
    expect(policy).toContain("object-src 'none'");
  });
});
