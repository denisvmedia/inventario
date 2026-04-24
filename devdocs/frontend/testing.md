# Testing

This document defines how we test frontend code: Vitest for units/components, Playwright for e2e, semantic locators throughout, snapshot tests only when justified.

## Stack

- **Vitest** + `@vue/test-utils` — unit specs and component mounts. Live in `__tests__/` next to source.
- **Playwright** — end-to-end browser tests. Live in `e2e/tests/`.
- **`axe-core`** (planned for Phase 5) — automated a11y assertions inside Playwright.

## Where tests live

| Source | Test |
|---|---|
| `frontend/src/design/ui/<name>/<Name>.vue` | `frontend/src/design/ui/<name>/__tests__/<Name>.spec.ts` |
| `frontend/src/design/patterns/<Name>.vue` | `frontend/src/design/patterns/__tests__/<Name>.spec.ts` |
| `frontend/src/design/composables/use<Name>.ts` | `frontend/src/design/composables/__tests__/use<Name>.spec.ts` |
| `frontend/src/services/<name>.ts` | `frontend/src/services/__tests__/<name>.test.ts` |
| `frontend/src/stores/<name>.ts` | `frontend/src/stores/__tests__/<name>.spec.ts` |
| `frontend/src/views/<feature>/<View>.vue` | `frontend/src/views/<feature>/__tests__/<View>.spec.ts` |
| Browser flow | `e2e/tests/<flow>.spec.ts` (Playwright) |

`*.spec.ts` for Vue/Pinia/Playwright; `*.test.ts` for plain TS service tests. Both suffixes are picked up by Vitest; the convention separates them by intent.

## Vitest — minimum coverage per primitive / pattern

Every new component must be tested for:

1. **Render** — mounts without errors with default props.
2. **Variants** — each `variant` and `size` from `cva` renders the expected class string.
3. **Props** — required props and `disabled` / `loading` state behave correctly.
4. **Slots** — default slot renders; named slots render when provided.
5. **Emits** — interactive actions emit the documented event with the documented payload.
6. **a11y essentials** — for interactive elements: has accessible name; respects disabled/aria-disabled.

Composables additionally test:

- Returned reactivity is correctly typed (compile-time).
- State transitions trigger the documented side effects.
- Cleanup on unmount when applicable.

## Locator policy

**Semantic locators only.** This applies to all new tests (Vitest mounts and Playwright).

Allowed:

- `getByRole('button', { name: 'Save' })`
- `getByLabel('Email')`
- `getByText('Welcome to Inventario')`
- `getByPlaceholderText('Search files…')`
- `getByTestId('user-menu')` — when role/label/text are insufficient

Forbidden in new tests:

- CSS selectors: `page.locator('.commodity-card')`, `wrapper.find('.btn-primary')`.
- XPath.
- Implementation-detail attributes (`page.locator('[data-v-xxxxxx]')`).

ESLint enforces this for new test files. Existing tests are migrated as part of the view they exercise (see [`migration-conventions.md`](./migration-conventions.md)).

### Class anchors stay — but tests do not target them

Patterns retain legacy CSS classes (`.commodity-card`, `.location-card`, `.file-card`, `.file-item`, `.export-row`) so existing Playwright tests keep working. **New** tests target the semantic role / label / testid instead. This dual policy is the only reason both styles coexist; after Phase 6 the unmigrated tests will have been replaced.

## `data-testid` policy

Use `data-testid` only when role/label/text are genuinely insufficient (e.g. anonymous container that needs a stable hook). Patterns that accept a `testId` prop forward it to their outermost element.

Naming: `kebab-case` describing the *role*, not the implementation. `data-testid="user-menu"` (good) — `data-testid="dropdown-menu-1"` (bad).

The current testid contract (must keep working through migration):

- `data-testid="user-menu"` — header user menu trigger
- `data-testid="current-role"` — header role badge
- `data-testid="current-user"` — header user name display
- `data-testid="current-group"` — header group selector display

Adding a new testid is a deliberate API decision. Document it in the pattern's JSDoc.

## Mocking

- **API**: stub `axios` via `vi.mock('@/services/api')` or stub the specific service module. Avoid hitting `globalThis.fetch`.
- **Router / route params**: use `useRouter` / `useRoute` with `vi.mock('vue-router')` and supply a typed stub.
- **Pinia stores**: import the store and call its actions directly; don't mock the store wholesale unless the test is purely about the consumer.
- **Time**: `vi.useFakeTimers()` for any code that uses `setTimeout` / `setInterval` / debounces. Always pair with `vi.useRealTimers()` in `afterEach`.

## Snapshot tests

Use sparingly. Only when:

- A visual regression is the actual concern (status pills with multiple statuses in one frame, theme tokens applied to a card).
- Done via `toMatchImageSnapshot` (Playwright component test) or DOM `toMatchInlineSnapshot` for small structures.

Do **not** snapshot text content — assertions on `getByText` / `toHaveText` are clearer and survive copy edits.

## Playwright e2e

Specs live in `e2e/tests/<flow>.spec.ts`. Conventions:

- One spec per user-visible flow (e.g. `commodity-simple-crud.spec.ts`).
- Use semantic locators (`page.getByRole(…)`, `page.getByLabel(…)`).
- Reset state between tests via the existing fixtures in `e2e/fixtures/`.
- Test the happy path + at least one error path per flow.

The current baseline is **21 specs** under `e2e/tests/`. The migration must keep all of them green at every PR (`make test-e2e`).

## Commands

```sh
make lint-frontend   # lint
make test-frontend   # vitest run (CI mode)
npm --prefix frontend run test:watch   # vitest --watch (local dev)
make test-e2e        # playwright (requires running backend; see e2e/README.md)
```

The CI runs the same chain. A PR that fails any of these does not merge.

## Coverage

`make test-frontend` reports coverage to `frontend/coverage/`. Per-PR target:

- New patterns: ≥ 90% lines, ≥ 80% branches.
- New composables: 100% lines, ≥ 90% branches.
- View specs: cover happy path + at least one error path; pixel-level coverage is not a goal here, e2e covers integration.

Drop in baseline coverage > 2% at the project level requires reviewer sign-off.
