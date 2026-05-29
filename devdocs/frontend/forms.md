# Forms

`react-hook-form` + `zod` everywhere. Server errors normalize through one
helper. The pattern is mirrored across every form in the app — copy from a
nearby page rather than inventing a new shape.

## The stack

| Concern | Library | Where it lives |
| --- | --- | --- |
| Form state, validation, submission | `react-hook-form` (`useForm`) | `react-hook-form` |
| Schema-driven validation | `zod` (`zodResolver`) | `zod`, `@hookform/resolvers` |
| Server error normalization | `parseServerError(err, fallback)` | `src/lib/server-error.ts` |
| Mutation | `useMutation` from a feature slice | `features/<name>/hooks.ts` |
| Translation | `useTranslation()` + `t("namespace:key")` | `react-i18next` |

## Schema

Schemas live next to the feature, not next to the page:

```
frontend/src/features/auth/schemas.ts
frontend/src/features/commodities/schemas.ts
frontend/src/features/locations/schemas.ts
frontend/src/features/groups/schemas.ts
frontend/src/features/tags/schemas.ts
```

Convention — error messages are **i18n keys, not English strings**:

```ts
// frontend/src/features/auth/schemas.ts
import { z } from "zod"

export const loginSchema = z.object({
  email: z.string().min(1, "auth:validation.emailRequired"),
  password: z.string().min(1, "auth:validation.passwordRequired"),
})
export type LoginInput = z.infer<typeof loginSchema>
```

The `message` field carries an i18n key like `auth:validation.emailRequired`.
RHF surfaces it through `errors[name]?.message` and the page resolves it via
`t(message)` at render time. This keeps schemas pure (no React, no i18n
context) and lets tests assert against the key without booting i18n.

> **Don't change** an existing key without adding the new one to
> `auth:validation.*` (or the right namespace) and to the
> `preservePatterns` list in `frontend/i18next.config.ts` — see
> [i18n.md](i18n.md).

When validation needs cross-field rules, use `superRefine`:

```ts
export const resetPasswordSchema = z
  .object({
    password: z.string().min(8, "auth:validation.passwordMinLength"),
    confirmPassword: z.string().min(1, "auth:validation.passwordConfirmRequired"),
  })
  .superRefine((value, ctx) => {
    if (value.password !== value.confirmPassword) {
      ctx.addIssue({
        code: "custom",
        path: ["confirmPassword"],
        message: "auth:validation.passwordsMismatch",
      })
    }
  })
export type ResetPasswordInput = z.infer<typeof resetPasswordSchema>
```

## Page

```tsx
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { useLogin } from "@/features/auth/hooks"
import { loginSchema, type LoginInput } from "@/features/auth/schemas"
import { parseServerError } from "@/lib/server-error"

export function LoginPage() {
  const { t } = useTranslation()
  const loginMutation = useLogin()
  const [serverError, setServerError] = useState<string | null>(null)

  const form = useForm<LoginInput>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: "", password: "" },
    mode: "onSubmit",
  })

  // Reset the server error whenever the user edits a field, so a stale
  // notice doesn't sit on top of valid input.
  useEffect(() => {
    const sub = form.watch(() => { if (serverError) setServerError(null) })
    return () => sub.unsubscribe()
  }, [form, serverError])

  async function onSubmit(values: LoginInput) {
    setServerError(null)
    try {
      await loginMutation.mutateAsync(values)
    } catch (err) {
      setServerError(parseServerError(err, t("auth:login.errorGeneric")))
    }
  }

  return (
    <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
      {serverError && (
        <Alert variant="destructive">
          <AlertDescription>{serverError}</AlertDescription>
        </Alert>
      )}

      <div className="space-y-1.5">
        <Label htmlFor="email">{t("auth:login.email")}</Label>
        <Input id="email" type="email" {...form.register("email")} />
        {form.formState.errors.email && (
          <p className="text-xs text-destructive">
            {t(form.formState.errors.email.message ?? "")}
          </p>
        )}
      </div>

      {/* …password field with the same shape… */}

      <Button
        type="submit"
        disabled={loginMutation.isPending || form.formState.isSubmitting}
      >
        {t("auth:login.submit")}
      </Button>
    </form>
  )
}
```

## Server-error surfacing

Backends emit three error envelopes:

```ts
// JSON:API
{ errors: [{ detail: "Invalid credentials", title: "Unauthorized" }] }

// Plain envelope
{ error: "Email already taken" }
{ message: "Email already taken" }

// Plain string body
"Email already taken"
```

Two layered helpers in `@/lib/server-error` normalise these:

- `parseServerError(err, fallback)` collapses all three envelopes into a
  single string, falling back to the supplied default for 5xx HTML
  responses or unknown shapes. The low-level primitive.
- `classifyServerError(err, fallback)` returns `{ kind, message }` where
  `kind` is `network | validation | conflict | unknown` — a hint that
  drives the banner's headline + whether a Retry affordance shows.

The **blessed form-level surface is `<ServerErrorBanner>`**
(`@/components/ServerErrorBanner`): one visual contract (icon + typed
title + message + conditional Retry) so users learn the affordances once.
Feed it a `classifyServerError` result; it renders nothing for `null`:

```tsx
import { ServerErrorBanner } from "@/components/ServerErrorBanner"
import { classifyServerError, type ClassifiedServerError } from "@/lib/server-error"

const [serverError, setServerError] = useState<ClassifiedServerError | null>(null)

try {
  await mutation.mutateAsync(values)
} catch (err) {
  setServerError(classifyServerError(err, t("auth:login.errorGeneric")))
}

// …in the form:
<ServerErrorBanner error={serverError} testId="…-server-error" />
```

For a bespoke message on a known status (e.g. a 422 "current password is
incorrect"), construct the classified value directly:
`setServerError({ kind: "validation", message: t("…") })` — see
`EditProfilePage`.

Rules:

- **Always pass a fallback** — never `classifyServerError(err, "")`. The
  fallback is what a 500 with an HTML body or a network error becomes.
  Use a translated string (`t("...")`).
- **Prefer `<ServerErrorBanner>`** for form-level server errors. The bare
  `<Alert variant="destructive">` + `parseServerError` string in some
  older auth pages (LoginPage) predates the typed banner; new forms use
  the banner. Field-level zod errors stay on `<FieldError>`, not the
  banner.
- **Reset on edit.** Watch the form via `form.watch()` and clear the
  banner when the user edits — see the LoginPage example.

## Submit-button gating

Submit buttons are gated on three things, in this order:

```tsx
disabled={loginMutation.isPending || form.formState.isSubmitting}
```

- `mutation.isPending` — request in flight.
- `form.formState.isSubmitting` — RHF's own promise hasn't resolved.
- Custom: `disabled={!form.formState.isValid}` only on `mode: "onChange"`
  forms where you genuinely want a live-validating gate. The default
  `mode: "onSubmit"` shows errors after the first submit attempt — that's
  what most pages use.

Don't add `disabled={!email || !password}` ad-hoc. Let zod gate via
`isValid` if you switch to `onChange`, or just let the submit attempt
fire and let the schema render the errors.

## Field components

Use the shadcn primitives directly — `Input`, `Label`, `Checkbox`,
`Select`, `Textarea`. One blessed piece is **not** a raw element: the
field-level error. Render it with `<FieldError>` (`@/components/FieldError`),
never an ad-hoc `<p className="text-xs text-destructive">`. The standard
field shape is:

```tsx
import { FieldError } from "@/components/FieldError"

const err = form.formState.errors.fieldId

<div className="space-y-1.5">
  <Label htmlFor="field-id">Label</Label>
  <Input
    id="field-id"
    aria-invalid={!!err}
    aria-describedby={err ? "field-id-error" : undefined}
    {...form.register("fieldId")}
  />
  <FieldError id="field-id-error" testId="field-id-error" message={err?.message} />
</div>
```

`<FieldError>` resolves the message through `t()` itself (the message is an
i18n key from the zod schema — see [Schema](#schema)), renders nothing when
the message is empty, and bakes in the `field-error` class the e2e suite
selects on. Wire `aria-describedby` on the input to the `FieldError` `id`
so screen readers announce the message when RHF focuses the invalid field.

For complex controls (combobox, date picker), wrap with `Controller`:

```tsx
import { Controller } from "react-hook-form"

<Controller
  name="currency"
  control={form.control}
  render={({ field, fieldState }) => (
    <CurrencyCombobox
      value={field.value}
      onChange={field.onChange}
      ariaInvalid={fieldState.invalid}
    />
  )}
/>
```

### Dropdowns

There is **one blessed dropdown for form & settings fields: the shadcn
`<Select>`** (`@/components/ui/select`, Radix under the hood). Wrap it in a
`Controller` for RHF forms; pass `field.ref` to the `SelectTrigger` so
focus-on-error still lands on the control:

```tsx
<Controller
  control={form.control}
  name="locationId"
  render={({ field }) => (
    <Select value={field.value || undefined} onValueChange={field.onChange}>
      <SelectTrigger id="location" ref={field.ref} className="w-full">
        <SelectValue placeholder={t("…")} />
      </SelectTrigger>
      <SelectContent>
        {options.map((o) => (
          <SelectItem key={o.id} value={o.id}>{o.name}</SelectItem>
        ))}
      </SelectContent>
    </Select>
  )}
/>
```

Gotcha: Radix forbids an **empty-string** `SelectItem` value. For an
"auto / none" option use a sentinel (`"auto"`) and map it back to `""` in
`value` / `onValueChange` (see `SettingsPage`'s Region & formatting field).

A raw native `<select>` is banned by the ESLint rule `no-restricted-syntax`
(see `eslint.config.js`). It stays permissible only for **utility /
compact / mobile** selectors — bulk-move bars, the mobile category-tile
collapse, list sort pickers — where native semantics are intentional and
already covered by `selectOption`-based tests. Those few sites carry an
explicit `// eslint-disable-next-line no-restricted-syntax -- <reason>`.
Don't add a new bare `<select>` to a form; reach for `<Select>`.

In tests, drive a `<Select>` with `pickRadixSelect(user, triggerLabel,
{ optionLabel })` from `@/test/radix` — `user.selectOptions()` no-ops on
the Radix trigger (it's a `role="combobox"` button, not an
`HTMLSelectElement`).

### Icon pickers

There are **two intentional icon pickers** — keep them separate, they are
not duplicates:

| Component | Use for | Shape |
| --- | --- | --- |
| `components/groups/IconPicker` | Group create / settings | Popover + category tabs + grid (many curated glyphs, space-constrained form) |
| `components/locations/IconPicker` | Location / area dialogs | Inline grid (small curated palette, mirrors the mock dialogs) |

They differ in UX, data source (`features/group/icons.ts` vs. a palette
prop), and a11y trade-offs by design. Pick by surface; don't merge them.

### Reference screen

`pages/EditProfilePage.tsx` is the canonical reference for these patterns
(#1264): RHF + zod, `<FieldError>` on every field, `<Select>` for the
default-group dropdown, and `<ServerErrorBanner>` for the profile +
password server errors. Copy its shape when building a new form.

## Multi-step forms

The Add Item dialog (`features/commodities/`) is the reference for
multi-step forms. Pattern:

- One RHF instance for the whole wizard. Each step renders a slice of
  the same form.
- The schema is one big object; per-step validation runs
  `await form.trigger(["field-a", "field-b"])` before advancing.
- Persist a draft to `localStorage` keyed `commodity-draft:{slug}:create`
  on every change (see PR #1447 for the helper). Hydrate on mount.
- "Cancel" clears the draft. "Save" clears the draft on success.

## Tests

A form test wires `renderWithProviders`, types into inputs, clicks
submit, and asserts:

- The mutation was called with the expected body (MSW handler asserts).
- Validation errors render the expected i18n key (the test reads the
  resolved English string via `screen.getByText(...)`).
- Server errors render via the `Alert`.
- `axe(container)` returns no violations.

See `frontend/src/pages/auth/__tests__/LoginPage.test.tsx` for a
full-shape example. Pattern + helpers live in [testing.md](testing.md).
