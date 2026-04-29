import AxeBuilder from '@axe-core/playwright';
import { expect, type Page } from '@playwright/test';

interface AxeAuditOptions {
  /**
   * Severities that should fail the test. Defaults to `['serious', 'critical']`
   * — `minor` / `moderate` are warnings only because the design mock has
   * known historical noise we don't want to gate on for every spec. Tests
   * that want a stricter audit (e.g. forms, dialogs) can pass
   * `['minor', 'moderate', 'serious', 'critical']`.
   */
  failOnImpact?: Array<'minor' | 'moderate' | 'serious' | 'critical'>;
  /**
   * CSS selector(s) that scope the scan. Defaults to the whole page.
   */
  include?: string | string[];
  /**
   * CSS selector(s) explicitly excluded from the scan. Use sparingly —
   * suppressing a violation should be a last resort and the comment at
   * the call site should say why.
   */
  exclude?: string | string[];
  /**
   * Axe rule ids to disable. Same caveat as `exclude`: prefer fixing the
   * underlying issue.
   */
  disableRules?: string[];
}

/**
 * Run an axe audit against the current page and assert no violations of
 * the configured severity. Wire this into every page-level spec — `@axe`
 * is part of the issue #1419 acceptance criteria. The default severity
 * floor (`serious` + `critical`) keeps day-to-day noise out of the CI
 * signal while still catching keyboard-trap, contrast, missing-label
 * style regressions.
 */
export async function axeAudit(page: Page, options: AxeAuditOptions = {}): Promise<void> {
  const failOnImpact = options.failOnImpact ?? ['serious', 'critical'];

  let builder = new AxeBuilder({ page });
  if (options.include) {
    builder = Array.isArray(options.include)
      ? options.include.reduce((b, sel) => b.include(sel), builder)
      : builder.include(options.include);
  }
  if (options.exclude) {
    builder = Array.isArray(options.exclude)
      ? options.exclude.reduce((b, sel) => b.exclude(sel), builder)
      : builder.exclude(options.exclude);
  }
  if (options.disableRules?.length) {
    builder = builder.disableRules(options.disableRules);
  }

  const results = await builder.analyze();

  const blockers = results.violations.filter((v) =>
    failOnImpact.includes((v.impact ?? 'minor') as (typeof failOnImpact)[number])
  );

  // Surface the blocker list in the failure message so traces don't have
  // to be opened to triage which rule regressed.
  expect(
    blockers,
    `axe found ${blockers.length} blocker(s) at impact ≥ ${failOnImpact.join(
      '/'
    )}:\n${blockers.map((v) => `  - ${v.id}: ${v.description}`).join('\n')}`
  ).toEqual([]);
}
