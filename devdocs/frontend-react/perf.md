# Frontend React perf — bundle size + Lighthouse gates

The `frontend-react/` perf gates land in PR #1437 (#1420). Two gates run on every PR; both can fail the merge.

## Bundle size — `size-limit`

Config: `frontend-react/.size-limit.json`. Three groups:

| Group                                   | Limit (gzip) | Why                                                                                              |
| --------------------------------------- | ------------ | ------------------------------------------------------------------------------------------------ |
| Initial JS (main + jsx-runtime + button) | 200 KB       | First-paint cost. Today: ~169 KB — ~30 KB of headroom for the auth pages (#1407) + dashboard widgets (#1408). |
| CSS                                     | 12 KB        | Tailwind v4 + tokens. Today: ~9 KB.                                                              |
| Lazy real pages (Dashboard / NotFound / RootRedirect) | 3 KB | Code-split chunks; tiny by design. Today: ~1.2 KB.                                              |

Run locally:

```bash
cd frontend-react
npm run build
npm run size       # gate
npm run size:why   # interactive bundle inspector — opens a webpack-bundle-analyzer-style report
```

CI: `.github/workflows/frontend-react-size.yml` runs on every PR via `andresz1/size-limit-action`, which posts a comment with the byte-level diff between PR and base branch. On master pushes the action records the new baseline.

### When to bump the limit

- **Don't** silently raise a limit to make a red PR green. The gate exists to prompt a "is this regression worth it?" conversation in the PR.
- **Do** bump when a meaningful feature legitimately costs more bytes (a charting lib for #1408, a rich editor for files). Cite the feature in the commit message and update this doc's "Why" column.
- **Tighten** before cutover (#1423). The issue's plan: tighten by 10% before flipping the default to React. The current generous limits assume the migration window; once feature parity is in, drop the headroom.

### Where to look when size goes up

1. `npm run size:why` — opens an interactive bundle inspector. Look for new dependencies that landed in the same PR.
2. `dist/assets/index-*.js` — the entry chunk. If a feature page added eagerly (not via `React.lazy()`), it lands here.
3. `vite.config.ts` — confirm `build.rolldownOptions.output.codeSplitting` is on; sometimes an unintentional side-effect import breaks a chunk.

## Lighthouse — LHCI

Config: `frontend-react/lighthouserc.cjs`. Thresholds (per #1420 AC):

| Category        | Threshold | Failure means …                                                          |
| --------------- | --------- | ------------------------------------------------------------------------ |
| performance     | ≥ 0.85    | TTI / LCP / CLS regressed. Most often a new big bundle or an unoptimised image. |
| accessibility   | ≥ 0.95    | A new heading-order issue, contrast failure, missing label. Lighthouse + jest-axe + @axe-core/playwright tend to catch the same things; LHCI is the in-browser sanity check. |
| best-practices  | ≥ 0.90    | Console errors, deprecated APIs, mixed content. Often a logging stmt left in production code. |
| seo             | off       | The app is auth-walled; SEO heuristics don't translate.                  |
| pwa             | off       | We don't ship a manifest.                                                |

URLs LHCI hits today (all public placeholder routes — they render without a backend):

- `/login`, `/register`, `/forgot-password` — `PlaceholderPage` stubs from #1404.
- `/some-nonexistent-route` — the styled NotFound page.

These cover the basic shell render; LHCI on logged-in pages (dashboard, items, settings) lands once #1407 ships login and the workflow can drive a Puppeteer auth script.

Run locally:

```bash
cd frontend-react
npm run build
npm run lhci
```

LHCI starts `vite preview` on port 4173 itself and tears it down when done. The HTML reports land under `frontend-react/.lighthouseci/`.

CI: `.github/workflows/frontend-react-lhci.yml` runs the same flow via `treosh/lighthouse-ci-action`. With `temporaryPublicStorage: true` the action uploads each HTML report and the PR comment links to them — so reviewers can drill into the audit without re-running locally.

### When a Lighthouse score drops

1. Click the report link in the PR comment. Lighthouse names the failing audit at the top.
2. The "Diagnostics" section is the practical action list — it ranks suggestions by estimated savings.
3. If the regression is genuine (new dependency, intentional UX change), justify it in the PR body and bump the threshold here. Bumping a threshold downward is a separate review-worthy change.
4. If LHCI is flaky (perf scores wobble run-to-run on shared CI hardware), bump `numberOfRuns` in `lighthouserc.cjs` from 1 → 3 and Lighthouse will average them.

## Why the page list is small today

The AC mentions five pages — dashboard, items list, item detail, files list, settings — but those are all auth-gated, and the auth pages themselves are placeholders until #1407 lands. LHCI on placeholders gives a stable baseline today; the real coverage extends as feature pages ship.

The plan:

1. **#1407 (auth pages)** swaps `/login` from a placeholder to a real form. LHCI score should stay high (form is small); if it drops, that's a signal to triage.
2. **#1407 + this PR's follow-up** adds a Puppeteer auth script (a `puppeteerScript` in `lighthouserc.cjs`) that LHCI runs once before measuring authenticated routes. Then the URL list grows to include `/g/:slug/` and friends.
3. **#1420's "tighten by 10%"** is a separate one-line PR before cutover that drops the budgets and thresholds in lockstep with the legacy parity declaration.

## Out of scope

- Mobile perf — we ship a desktop-first app; LHCI runs the desktop preset. A separate mobile run can land if a mobile use-case appears.
- A historical perf dashboard — Lighthouse CI can sync to a hosted server (`@lhci/server`) but we're not running one. Temporary public storage is the simplest approach until we have a need.
