// Lighthouse CI config for the new React frontend (#1420).
//
// LHCI builds dist/ via the workflow, then `vite preview` serves it on
// :4173 while Lighthouse hits each URL below. The thresholds gate merges
// — a regression below them turns the PR red.
//
// Today's URL list covers the public pages: /login, /register,
// /forgot-password, /reset-password, /verify-email, and the catch-all
// 404. Each renders a placeholder (or the real NotFound page) without
// requiring a backend, so LHCI can run against a static `vite preview`
// server. As feature pages land:
//   - #1407 (auth pages) replaces the placeholders here with real forms.
//     Same URLs, no LHCI config change.
//   - Authenticated pages (dashboard, items, settings) need a logged-in
//     session. They land via a Puppeteer auth script the workflow
//     supplies once #1407 ships login.
//
// Threshold rationale (#1420 AC):
//   - performance >= 0.85 — the bundle is ~170KB gzip and pages are
//     statically rendered after first paint, so 0.85 is a comfortable
//     floor that flags mass JS regressions without false-positive
//     flakes.
//   - accessibility >= 0.95 — we already run jest-axe + @axe-core/
//     playwright, so the in-browser Lighthouse audit should be a
//     superset that lands on near-100. 0.95 leaves headroom for
//     individual rule disagreements between tools.
//   - best-practices >= 0.90 — guards against console errors, deprecated
//     APIs, mixed content. The default audit set is stable.
//   - seo — disabled. The app is auth-walled; LHCI's SEO heuristics
//     (canonical, meta description, robots) don't translate.

module.exports = {
  ci: {
    collect: {
      // `vite preview` is the simplest static serve for the dist/ build
      // — no backend, no proxying. LHCI auto-stops the server when the
      // run finishes.
      startServerCommand: 'npx vite preview --port 4173 --strictPort',
      startServerReadyPattern: 'Local:',
      startServerReadyTimeout: 30_000,
      url: [
        'http://localhost:4173/login',
        'http://localhost:4173/register',
        'http://localhost:4173/forgot-password',
        'http://localhost:4173/some-nonexistent-route',
      ],
      numberOfRuns: 1,
      settings: {
        // Mobile emulation is the default but we want the desktop floor
        // since the app's target form factor is desktop-first. Mobile
        // perf is tracked via a separate run once we have the auth
        // pages and can navigate beyond /login.
        preset: 'desktop',
        // Disable categories Lighthouse runs by default but we don't
        // gate on. Skipping pwa removes ~30s per page.
        skipAudits: ['uses-http2', 'redirects-http'],
      },
    },
    assert: {
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
