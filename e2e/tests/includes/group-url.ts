import { Page } from '@playwright/test';

/**
 * Returns the group-scoped URL prefix for data-plane routes, e.g.
 * `/api/v1/g/<slug>`. The frontend's axios instance injects this prefix via
 * an interceptor for `/api/v1/locations|commodities|files|exports|…`; tests
 * that drive the raw `page.request` / `request` fixtures bypass that axios
 * instance, so they have to prepend it themselves.
 *
 * Reads the same `currentGroupSlug` localStorage key the axios interceptor
 * uses (set by the groupStore on setCurrentGroup / restoreFromStorage).
 * Throws if no slug is present — tests that need group-scoped URLs must
 * have a group selected, and silently producing an unprefixed URL would
 * only re-introduce the 404 failure mode this helper exists to avoid.
 */
export async function groupApiBase(page: Page): Promise<string> {
  const slug = await page.evaluate(() => localStorage.getItem('currentGroupSlug'));
  if (!slug) {
    throw new Error('groupApiBase: currentGroupSlug is not set in localStorage — tests using group-scoped endpoints must have an active group selected');
  }
  return `/api/v1/g/${slug}`;
}
