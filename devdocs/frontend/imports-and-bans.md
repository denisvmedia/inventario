# Imports and bans

The dependencies the React frontend deliberately doesn't ship and why.
Keep this list in sync with `frontend/package.json` and the rationale
in the issues that drove each ban.

## The bans

| Banned | Why | Use instead |
| --- | --- | --- |
| `next-themes` | Pretends the app is Next.js (cookies, server-side rendering hooks). The frontend is a Vite SPA. | Tiny custom theme provider in `src/components/theme-provider.tsx` (writes the `.dark` class on `<html>`). |
| `@base-ui/react` | The design mock shipped both `radix-ui` and `@base-ui/react`. Two headless primitive libraries doing the same job is one too many. | `radix-ui` (the umbrella package). |
| `@tailwindcss/animate` | Replaced by `tw-animate-css` (works with Tailwind v4's `@theme inline`). | `tw-animate-css` — already in `package.json`, imported at the top of `src/index.css`. |
| `@fortawesome/react-fontawesome` (and any FA icon set), `primeicons`, `react-icons`, `@heroicons/react`, `@material-ui/icons` | One icon library. Mixing icon libraries breaks the visual rhythm and bloats the bundle. | `lucide-react`. See [icons.md](icons.md). |
| Bolt scaffolding artifacts (`bolt-*` packages, `BoltGlobals`, `bolt:` data attributes, the literal string `"Bolt"` in `<title>`) | Leftover from the React scaffold's origin. They have no runtime purpose and act as a tripwire. | Delete on sight. The Go embed test (`go/apiserver/frontend_embed_test.go`) asserts the bundled HTML's title is `Inventario` and contains no Bolt artifacts. |
| Vue, Vue Router, Pinia, PrimeVue, PrimeFlex, vue-i18n, sass | Legacy frontend dependencies, deleted at cutover (#1423, PR #1457). | The React stack — see [README.md](README.md). |
| Framer Motion, react-spring, popmotion | Animation needs are met by `tw-animate-css` + Tailwind utilities; a runtime animation library is dead weight. | `animate-in`, `fade-in-0`, `slide-in-from-top-2`, etc. (see [styles-and-tokens.md](styles-and-tokens.md)). |
| `axios`, `superagent`, `ky` | We have one fetch wrapper (`src/lib/http.ts`) that owns CSRF, group-rewriting, refresh, and JSON:API content type — none of which a generic HTTP library handles for us. | `src/lib/http.ts` via the feature slice's `api.ts`. See [data.md](data.md). |
| `@base-ui/react`'s `Combobox`, `react-select`, `downshift`, headless menus other than Radix | Same ground covered by Radix + `cmdk`. | `radix-ui` for menus / popovers, `cmdk` for searchable lists (already in `Command`/`CommandPalette`). |
| `clsx` (separate import) | shadcn's `cn()` already wraps `clsx` + `twMerge`. | `cn()` from `@/lib/utils`. |
| `enzyme`, `react-testing-renderer` shallow, `jest` | The test stack is Vitest + RTL. | `@testing-library/react` via `renderWithProviders`. See [testing.md](testing.md). |

## Detection

The conventions above are enforced by:

1. **PR review.** `package.json` adds get scrutiny — every new
   dependency justifies itself in the PR body.
2. **Embed smoke tests.** `go/apiserver/frontend_embed_test.go` asserts
   the bundled HTML's `<title>` is `Inventario` and contains no Bolt
   artifacts. Catches accidental scaffolding leftovers and HTML edits
   that swap in something else.
3. **Lighthouse `best-practices`.** Console errors from a banned-but-
   somehow-installed library (e.g. `next-themes` complaining about a
   missing context) tank the score and trip the gate.
4. **Bundle-size gate.** A regression past the entry-bundle budget
   surfaces the new dependency in `npm run size:why`. See [perf.md](perf.md).

## Wiring `no-restricted-imports`

The ESLint config (`frontend/eslint.config.js`) doesn't currently set
`no-restricted-imports`, so the bans live as conventions enforced by
review + the smoke tests above. If you want to add the rule, drop the
following block into the typescript override:

```js
{
  files: ["**/*.{ts,tsx}"],
  rules: {
    "no-restricted-imports": ["error", {
      paths: [
        { name: "next-themes", message: "Use src/components/theme-provider.tsx — Vite SPA, not Next.js." },
        { name: "@base-ui/react", message: "Locked to radix-ui; do not introduce a second headless library." },
        { name: "@tailwindcss/animate", message: "Replaced by tw-animate-css." },
        { name: "axios", message: "Use src/lib/http.ts via the feature slice's api.ts." },
        { name: "clsx", message: "Use cn() from @/lib/utils." },
      ],
      patterns: [
        { group: ["@fortawesome/*", "primeicons", "react-icons*", "@heroicons/*"], message: "lucide-react only — see devdocs/frontend/icons.md." },
      ],
    }],
  },
}
```

It's an opt-in tightening — drop it in once the team agrees the
conventions are stable, and never weaken a `paths`/`patterns` entry to
land a PR.

## When the ban needs to break

A genuine new requirement (e.g. a charting library that shadcn-chart +
recharts can't satisfy) is allowed to bend the bans, but never silently:

1. Open an issue describing the need, the alternative considered, and
   the bundle-size impact.
2. Get explicit approval before adding the dep.
3. Update this doc with the new entry and the reason it's allowed.

## What's *not* a ban

- **`@tanstack/react-query-devtools`** — installed and mounted under
  `import.meta.env.DEV` only in `src/app/providers.tsx`. The dev-only
  gate is the contract; don't import it outside that gate.
- **`@radix-ui/react-*`** — fine, but prefer the `radix-ui` umbrella so
  versions stay consistent.
- **shadcn primitive copies** — `npx shadcn@latest add <component>`
  always allowed; the result lands in `src/components/ui/` and is
  edited only via re-running the CLI.
- **OpenAPI codegen** — `openapi-typescript` runs via `npm run codegen`
  to refresh `src/types/api.d.ts`. It's a build-time tool, not a
  runtime dep.
- **Renovate-driven version bumps** — pinned versions update via
  Renovate PRs; review the changelog and merge.
