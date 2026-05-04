# Components

Where components live, what each layer is allowed to do, and how to add a
new one.

## Three layers

```
frontend/src/components/
├── ui/              # vendored shadcn primitives (Radix-based)
├── routing/         # composition-only — ProtectedRoute, GroupRequiredRoute, RouteTitle, UngroupedRedirect
├── auth/            # cross-page composites for the auth pages
├── coming-soon/     # ComingSoonBanner / ComingSoonPage registry
├── dashboard/       # widgets shared across dashboard surfaces
├── exports/         # cross-page composites for exports/restore
├── files/           # file-list helpers
├── groups/          # group-settings composites
├── items/           # commodity composites
├── locations/       # locations / areas composites
├── tags/            # tag pill, color picker, badges
├── search/          # search composites
└── <Standalone>.tsx # AppSidebar, AppLogo, CommandPalette, GroupSelector, ModeToggle, …

frontend/src/features/<name>/
└── (api.ts, hooks.ts, keys.ts, schemas.ts — no .tsx)
```

Three categories with three rule sets:

### `components/ui/` — shadcn primitives

Vendored copies of shadcn's new-york style primitives (Button, Dialog,
Sheet, DropdownMenu, Sidebar, Sonner toaster, …). Owned upstream by
shadcn and mirrored 1:1 against the internal design mock.

Rules:

- **Never edit a primitive directly.** If you need a variant, extend via
  `cva` or wrap. If shadcn upstream changes, re-run `npx shadcn@latest add
  <component>` from `frontend/` to refresh the file.
- **Mirror the mock.** When the design mock bumps a primitive, copy 1:1.
  Document the diff in the PR if you must deviate (e.g. swapping
  `next-themes` out of `sonner.tsx`).
- **No `forwardRef`.** React 19 + Tailwind v4 lets primitives accept `ref`
  as a prop. Match the existing files.
- **One primitive per file** named after the component. Re-exports go via
  the file's named exports — there is no barrel.
- **Excluded from coverage.** `src/components/ui/**` is in
  `vitest.config.ts`'s `coverage.exclude`. Test composites that consume
  them, not the primitives themselves.

When to add a new primitive: only when you need a Radix primitive the
project doesn't already vendor. `npx shadcn@latest add <name>` is the
on-ramp; the CLI will copy the file in and add the import. Pick the
component name from <https://ui.shadcn.com/docs/components>.

### `components/<Name>.tsx` (and folders) — cross-feature components

Used by 2+ feature slices, or owns a domain concept that should look the
same everywhere (e.g. `WarrantyBadge`, `CurrencyCombobox`, `AppSidebar`,
`CommandPalette`, `TagPill`).

Rules:

- **No data fetching inside.** Composite components accept props; the
  feature page above them does the `useQuery` / `useMutation`. Exceptions
  are explicit cross-feature shells (`AppSidebar` reads `useCurrentUser` /
  `useCurrentGroup` because every page renders it).
- **Accept `className?`** for layout flexibility at the call site, and
  merge it with `cn(base, className)` from `@/lib/utils`.
- **Don't accept `style`** unless the value is dynamic from data (e.g. a
  tag's color token). Tokens win over inline styles — see
  [styles-and-tokens.md](styles-and-tokens.md).
- **Never import from `pages/`.** Page → component is one-way; the moment
  a component needs a page, it has become a page wrapper.
- **Stay dumb.** Props in, JSX out. If state lives across renders, lift it
  to the page that owns the route boundary.
- **A folder is a component family.** Group related composites under
  `components/<feature>/` (`exports/ExportRow.tsx`, `exports/WizardSteps.tsx`)
  rather than spraying `ExportRow.tsx` directly into `components/`.

### `pages/<Page>.tsx` — route components

One file (or one folder) per route in `app/router.tsx`. A page does:

1. Reads URL params via `useParams` / `useSearchParams`.
2. Calls the feature slice's hooks (`useCommodities`, `useCommodity`, …).
3. Composes UI primitives + cross-feature components into a layout.
4. Owns top-level state that the route boundary scopes (open dialog,
   wizard step, draft form values).

Rules:

- **Code-split via `React.lazy`** in `app/router.tsx`. Real pages are
  always lazy; placeholder pages share `PlaceholderPage` and stay eager.
- **Page wrapper:** start every page with the standard outer shell (mock
  pattern):
  ```tsx
  <div className="flex flex-col gap-6 p-6 max-w-2xl mx-auto w-full">…</div>
  ```
  Use `max-w-2xl` for settings/detail and `max-w-4xl` for list/data pages.

## Variants via `cva`

shadcn primitives encode variants using `class-variance-authority` (`cva`).
Mirror that pattern when extending — never reach for prop-driven `if/else`
class concatenation:

```tsx
const buttonVariants = cva("base classes", {
  variants: {
    variant: { default: "…", outline: "…", ghost: "…" },
    size: { default: "h-9", sm: "h-8", lg: "h-10", icon: "size-9" },
  },
  defaultVariants: { variant: "default", size: "default" },
})
```

Domain-specific badges follow the same shape — `WarrantyBadge` reads
`WARRANTY_STATUS_CONFIG[status]` from a config map and applies the matching
classes. Don't inline ternaries on color or icon choice.

## Compose, don't theme-drill

A new "X but with a green border" is almost never a new component. Reach
for a className override at the call site first:

```tsx
<Card className="border-status-active/40">…</Card>
```

When the override repeats 3+ times, extract a thin wrapper (still in
`components/`) — never duplicate primitive markup.

## Adding a new component — step by step

1. **Decide the layer:**
   - shadcn primitive → `npx shadcn@latest add <name>` from `frontend/`.
   - Cross-feature composite → `src/components/<Name>.tsx` (or
     `src/components/<feature>/<Name>.tsx`).
   - Page-local helper → keep it in the page file until it's used twice.
2. **Write the type:**
   ```tsx
   interface FooProps {
     value: string
     onChange: (next: string) => void
     className?: string
   }
   ```
   Avoid `style?` unless you genuinely need it.
3. **Write the named export:**
   ```tsx
   export function Foo({ value, onChange, className }: FooProps) {
     return <div className={cn("base classes", className)}>…</div>
   }
   ```
4. **Add a test** under `__tests__/Foo.test.tsx` using
   `renderWithProviders`. Cover the empty / loaded / error states; run
   `axe(container)` if the component renders interactive UI. See
   [testing.md](testing.md).
5. **Translate every string** through `useTranslation()`. See [i18n.md](i18n.md).
6. **Run `npm run i18n:check`** to confirm the catalog stays in sync, and
   `npm run lint && npm run typecheck && npm run test` before opening the PR.

## Anti-patterns

- **Class-name template literals.** Use `cn()` from `@/lib/utils`, not
  `` `${base} ${active ? "..." : "..."}` `` — it deduplicates Tailwind
  conflicts and is what shadcn primitives use.
- **Re-implementing a primitive.** If `<Button>` exists, don't write `<button
  className="border rounded …">`. Same for `<Input>`, `<Dialog>`,
  `<DropdownMenu>`.
- **`window.confirm()`.** Use `<AlertDialog>` for destructive flows, or
  `useConfirm()` (`src/hooks/useConfirm.tsx`) which wraps an
  `AlertDialog` in a promise.
- **Pulling DOM refs to coerce focus.** Radix primitives expose
  `onOpenChange`, `onSelect`, `onCloseAutoFocus`, etc. Read the primitive's
  props before reaching for an imperative ref.
- **Inline color or spacing styles.** `style={{ color: "#f59e0b" }}` is a
  ban on sight; use the token (`text-chart-1`) and update `index.css` if
  the token doesn't exist.
