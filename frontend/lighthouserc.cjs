// Lighthouse CI config for the new React frontend (#1420).
//
// LHCI builds dist/ via the workflow, then serves it through its own
// built-in static server (`staticDistDir`) with SPA fallback so unknown
// paths resolve to index.html and React handles the route. The
// thresholds gate merges — a regression below them turns the PR red.
//
// `staticDistDir` is preferred over a `startServerCommand`-driven
// `vite preview` because LHCI's headless Chrome was hitting a
// CHROME_INTERSTITIAL_ERROR when navigating to vite-preview-served
// URLs in CI; the built-in static server doesn't have that issue.
//
// Today's URL list covers four public pages: /login, /register,
// /forgot-password (placeholder routes from #1404), and a catch-all
// path that resolves to the styled NotFound page. Each renders
// without requiring a backend, so LHCI can run against the static
// dist/ directly. Per-feature React PRs add their own URLs as they
// land:
//   - #1407 (auth pages) replaces the placeholders here with real forms.
//     URL list unchanged; thresholds still apply.
//   - Authenticated pages (dashboard, items, settings) need a logged-in
//     session. They land via a Puppeteer auth script the workflow
//     supplies once #1407 ships login.
//
// Threshold rationale (#1420 AC):
//   - performance >= 0.85 — bundle is ~170KB gzip; pages are
//     statically rendered after first paint, so 0.85 flags mass
//     regressions without false-positive flakes.
//   - accessibility >= 0.95 — we already run jest-axe + @axe-core/
//     playwright; LHCI is the in-browser superset audit. 0.95 leaves
//     headroom for cross-tool rule disagreements.
//   - best-practices >= 0.90 — guards against console errors,
//     deprecated APIs, mixed content.
//   - seo / pwa — gated off in `assert.assertions` below; SEO
//     heuristics don't translate for an auth-walled app and we don't
//     ship a PWA manifest.

module.exports = {
  ci: {
    collect: {
      // LHCI serves dist/ itself with SPA fallback for unknown routes.
      // No vite-preview, no docker, no auth. `isSinglePageApplication`
      // makes the built-in static server fall back to index.html for
      // any path that doesn't exist on disk (otherwise /login would
      // 404 since the build only emits index.html + assets/).
      staticDistDir: './dist',
      isSinglePageApplication: true,
      url: [
        'http://localhost/login',
        'http://localhost/register',
        'http://localhost/forgot-password',
        'http://localhost/some-nonexistent-route',
      ],
      // 3 runs × 4 URLs is the de-flake floor for shared GitHub runners.
      // LHCI's default assertion `aggregationMethod` is `median`, so a
      // single noisy run no longer reds the PR. With 1 run we'd seen
      // /login swing from 0.79 to ≥0.85 between back-to-back commits
      // without any code change touching auth pages — that's variance,
      // not a regression. ~4 minutes of CI vs the cost of phantom
      // failures: pay the time. If perf becomes a real bottleneck,
      // gate this list to changed pages instead of dialing it back.
      numberOfRuns: 3,
      settings: {
        // Mobile emulation is the default; we want desktop because the
        // app is desktop-first. Mobile perf gets its own run once we
        // have authenticated pages worth measuring.
        preset: 'desktop',
      },
    },
    assert: {
      // LHCI defaults aggregationMethod to `median` for `error`-level
      // assertions, so the threshold is checked against the median of
      // the 3 collected runs above — a single bad run can't fail the
      // build, but a real regression will because the median moves.
      assertions: {
        'categories:performance': ['error', { minScore: 0.85 }],
        'categories:accessibility': ['error', { minScore: 0.95 }],
        'categories:best-practices': ['error', { minScore: 0.9 }],
        // SEO heuristics don't translate for an auth-walled app.
        'categories:seo': 'off',
        // PWA is opt-in; we're not shipping a manifest.
        'categories:pwa': 'off',
      },
    },
    upload: {
      // Temporary public storage gives the PR comment a reachable URL
      // to a full Lighthouse report. No infra to host anything
      // ourselves; reports expire after a few days, which is fine for
      // CI signal-only use.
      target: 'temporary-public-storage',
    },
  },
}
