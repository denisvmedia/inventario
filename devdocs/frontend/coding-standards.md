# Coding standards

The contract every `.ts` / `.tsx` file in `frontend/` follows. Linted where
practical, otherwise enforced by review. If a rule disagrees with the
[visual contract](styles-and-tokens.md) or the design mock, the visual contract wins — flag the conflict in the PR.

## TypeScript

- **Strict mode is on** — set in `frontend/tsconfig.app.json` and
  `frontend/tsconfig.node.json` (the root `frontend/tsconfig.json` is a
  solution file that only references them). Never weaken `strict` to land a
  PR; fix the typing instead.
- **`any` is a warning, not an error** — `@typescript-eslint/no-explicit-any:
  warn` (see `frontend/eslint.config.js`). Treat each `any` as a TODO and
  prefer `unknown` + a narrowing guard, or a typed schema (`zod`,
  generated DTO) at the boundary.
- **No `forwardRef`.** Tailwind v4 + React 19 lets you accept `ref` as a
  normal prop via `React.ComponentProps<>`. Pattern (mirrors shadcn primitives
  in `src/components/ui/`):
  ```tsx
  export function Thing({
    className,
    ...props
  }: React.ComponentProps<"div">) {
    return <div className={cn("...", className)} {...props} />
  }
  ```
- **No default exports.** Named exports only — they survive renames cleanly
  and play well with auto-import. The single exception is files Vite or a
  build tool requires to default-export (see `frontend/vite.config.ts`).
- **Discriminated unions over enums.** TS `enum` adds runtime weight and
  doesn't tree-shake; use `as const` arrays + a derived type:
  ```ts
  export const DENSITIES = ["comfortable", "cozy", "compact"] as const
  export type Density = (typeof DENSITIES)[number]
  ```

## File and directory layout

```
frontend/src/
├── app/              # composition root: providers, router, Shell
├── components/
│   ├── ui/           # shadcn primitives (vendored — see components.md)
│   ├── routing/      # ProtectedRoute, GroupRequiredRoute, RouteTitle, …
│   └── <Other>.tsx   # cross-feature components
├── features/<name>/  # feature slice: api.ts, hooks.ts, keys.ts, schemas.ts
├── hooks/            # cross-feature hooks (useDensity, useConfirm, …)
├── i18n/             # react-i18next config + en/cs/ru bundles
├── lib/              # cn(), http, auth-storage, group-context, server-error, …
├── pages/            # route components — one folder per top-level route
├── test/             # Vitest setup, render helper, MSW handlers
└── types/            # generated DTOs + hand-written shared types
```

Rules of thumb:

- **One concern per file.** A page does layout + wiring; business logic and
  data shaping live in `features/<name>/{api,hooks,schemas}.ts`. A shared
  visual concept lives in `components/<Name>.tsx`.
- **Feature slice owns its types.** Re-export DTOs through
  `features/<name>/api.ts` so callers don't pull from `types/api.d.ts`
  directly.
- **Tests sit next to source** under `__tests__/` (or as `<name>.test.tsx`
  next to the file). The Vitest config picks up both shapes.

## Naming

| Kind | Convention | Example |
| --- | --- | --- |
| React component file | `PascalCase.tsx` | `LocationsListPage.tsx` |
| Hook file | `useCamelCase.ts(x)` | `useDensity.tsx` |
| Other module | `kebab-or-snake.ts` | `auth-storage.ts`, `query-client.ts` |
| Component name | `PascalCase` matching filename | `export function LocationsListPage()` |
| Hook | starts with `use` | `useCurrentUser`, `useLogin` |
| Type | `PascalCase` | `LoginInput`, `CurrentUser` |
| Constant | `UPPER_SNAKE_CASE` for closed enumerations, `camelCase` otherwise | `DENSITIES`, `BASE_URL` |
| Query key factory | `<feature>Keys` | `authKeys`, `commodityKeys` |
| Translation key | `namespace:dot.path` | `auth:validation.emailRequired` |

## Import order

ESLint doesn't enforce order today, but every existing file follows this
pattern (matches Prettier's default sort and what eslint-plugin-import would
produce):

1. Node / browser builtins (rare in the FE).
2. External packages (`react`, `@tanstack/react-query`, `lucide-react`, …).
3. Internal absolute imports via the `@/` alias, grouped by depth or feature
   coherence:
   - `@/components/ui/*`
   - `@/components/*`
   - `@/features/*`
   - `@/hooks/*`
   - `@/lib/*`
   - `@/pages/*`
   - `@/types/*`
4. Relative imports (`./api`, `./schemas`).
5. CSS / asset imports.

Within a group, alphabetize unless logical grouping reads better (e.g.
keep `useForm`, `Controller`, `useFieldArray` from `react-hook-form`
together).

## Console policy

Production code must not log. The Lighthouse `best-practices` audit (see
[perf.md](perf.md)) fails on console errors and warnings.

| Allowed | Where |
| --- | --- |
| `console.error` for unrecoverable boot failures (e.g. missing CSRF token) | `src/main.tsx`, `src/app/providers.tsx` |
| `console.warn` for missing-i18n keys (gated to dev) | `src/i18n/i18next.config.ts` |
| `console.*` in tests, scripts/, and `*.config.*` | anywhere outside `src/**/*.{ts,tsx}` shipping to dist |

If you're tempted to `console.log` from a component, use a toast
(`useAppToast`) for user-visible feedback or a Sentry-style logger when
that lands. Don't ship `console.log` to silence a "hmm, why didn't this
fire" curiosity — write a test instead.

## Comments

- Default to no comments. A well-named identifier already explains *what*
  the code does.
- Comment **why** when:
  - There's a non-obvious constraint (`// localStorage; cross-tab via
    'storage' event`).
  - There's a workaround for a specific bug (link the issue).
  - The code violates a local convention on purpose.
- Never write multi-paragraph docstrings on internal helpers. One short
  line is the cap.
- Don't reference the current task / PR / fix in code comments. That
  context belongs in the PR body and rots in the source over time.

The existing source has more "why" comments than this rule prescribes —
those are deliberately load-bearing (they explain a refresh-race, a
single-flight token, a Rules-of-Hooks ordering trick). Don't strip them
just to match this rule; *add* new comments only when they pay rent.

## Formatting

- Prettier is the source of truth. Run `npm run format` from `frontend/`
  before pushing; CI runs `npm run format:check`.
- Tailwind class lists: keep them on one logical line per element. Don't
  break them across lines unless the element's class list exceeds the
  Prettier wrap width — let Prettier decide.
- Don't fight Prettier with `// prettier-ignore` unless you have a real
  reason (e.g. a 6-column matrix that becomes unreadable when wrapped).

## Function shape

- **Components** are named function declarations:
  ```tsx
  export function LoginPage() { … }
  ```
  not `const LoginPage = () => …`. Function declarations show up better in
  React DevTools and stack traces.
- **Hooks and utilities** can be either form — pick whichever reads better.
- **Async handlers** are `async function onSubmit(values) { … }` declared
  inside the component, not extracted to module scope (they almost always
  close over component-local state).

## Async / errors

- `async` functions throw — `useMutation` and `useQuery` capture the throw
  and surface it through `error`. Don't wrap them in `try/catch` just to
  return `null`; reach for `error` from the hook.
- At UI boundaries, normalize errors via
  `parseServerError(err, fallback)` (`src/lib/server-error.ts`). Don't
  render `err.message` directly — backends emit JSON:API, plain envelopes,
  and string bodies.
- Never swallow errors silently. If you intentionally ignore one (e.g. a
  best-effort logout), comment why right above the `catch` block.

## What not to do

- **Don't** add a new CSS file. All styling flows through `index.css`
  tokens + Tailwind utilities. See [styles-and-tokens.md](styles-and-tokens.md).
- **Don't** install a new icon library, headless UI library, or theming
  library. The bans are documented in [imports-and-bans.md](imports-and-bans.md);
  if you genuinely need something new, file an issue first.
- **Don't** weaken a coverage threshold to land a feature. Find the missing
  test instead. See [testing.md](testing.md).
- **Don't** change a translation key's path to "fix the extractor". Add a
  `preservePattern` in `frontend/i18next.config.ts` if the key is dynamic;
  the existing patterns there document the trick.
