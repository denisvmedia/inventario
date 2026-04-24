# Forms

This document defines how forms are built in this codebase: `vee-validate` + `zod` via shadcn-vue's `<Form>` primitive. There is no second way.

## Stack

- **`vee-validate@^4`** — form state, field state, validation orchestration, submission lifecycle.
- **`zod@^3`** — typed schemas. Single source of truth for validation **and** TypeScript types.
- **`@vee-validate/zod`** — adapter (`toTypedSchema`).
- **shadcn-vue `<Form>` primitives** — `Form`, `FormField`, `FormItem`, `FormLabel`, `FormControl`, `FormMessage`, `FormDescription`. Live in `frontend/src/design/ui/form/`.

Forbidden alternatives:

- Ad-hoc `formErrors: Ref<Record<string, string>>` objects.
- Inline `if (!name) errors.name = 'required'` chains.
- Validation in the `submit` handler instead of in a schema.
- Validation libraries other than zod (yup, joi, valibot, custom).

## Schema location

The zod schema lives next to the view that consumes it as `<View>.schema.ts`:

```
frontend/src/views/commodities/
├── CommodityCreateView.vue
├── CommodityCreateView.schema.ts
├── CommodityEditView.vue
└── CommodityEditView.schema.ts
```

When two forms share fields (Create + Edit), extract the shared bits into a base schema in the same directory:

```ts
// CommodityForm.schema.ts (shared)
import { z } from 'zod'

export const commodityBaseSchema = z.object({
  name: z.string().min(1, 'Name is required').max(100),
  shortName: z.string().max(20).optional(),
  count: z.coerce.number().int().min(1).max(10_000),
  // …
})

// CommodityCreateView.schema.ts
import { commodityBaseSchema } from './CommodityForm.schema'
export const commodityCreateSchema = commodityBaseSchema.extend({
  draft: z.boolean().default(false),
})

// CommodityEditView.schema.ts
import { commodityBaseSchema } from './CommodityForm.schema'
export const commodityEditSchema = commodityBaseSchema.extend({
  id: z.string().uuid(),
})
```

Export both the schema and the inferred type:

```ts
export type CommodityCreateInput = z.infer<typeof commodityCreateSchema>
```

## Form skeleton

```vue
<script setup lang="ts">
import { useForm } from 'vee-validate'
import { toTypedSchema } from '@vee-validate/zod'

import { Button } from '@design/ui/button'
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@design/ui/form'
import { Input } from '@design/ui/input'

import { commodityService } from '@/services/commodityService'

import { commodityCreateSchema } from './CommodityCreateView.schema'

const { handleSubmit, isSubmitting, setErrors } = useForm({
  validationSchema: toTypedSchema(commodityCreateSchema),
})

const onSubmit = handleSubmit(async (values) => {
  try {
    await commodityService.create(values)
  } catch (error) {
    if (isApiValidationError(error)) {
      setErrors(error.fieldErrors) // server → form
      return
    }
    throw error // bubble to error boundary / toast
  }
})
</script>

<template>
  <form class="space-y-6" @submit="onSubmit">
    <FormField v-slot="{ componentField }" name="name">
      <FormItem>
        <FormLabel>Name</FormLabel>
        <FormControl>
          <Input v-bind="componentField" placeholder="Enter the full name" />
        </FormControl>
        <FormMessage />
      </FormItem>
    </FormField>

    <!-- … more fields … -->

    <div class="flex justify-end gap-2">
      <Button type="button" variant="outline">Cancel</Button>
      <Button type="submit" :disabled="isSubmitting">Create</Button>
    </div>
  </form>
</template>
```

Key contracts:

- `<form @submit="onSubmit">` — never `@click` on the submit button. `handleSubmit` listens for the form-submit event so Enter-key submission works.
- Every field is `<FormField>` + `<FormItem>` + `<FormLabel>` + `<FormControl>` + `<FormMessage>`. No bare `<Input>` inside `<form>`.
- `componentField` from the slot binds `modelValue`, `onUpdate:modelValue`, `onBlur`, and validation state in one go. Use it instead of manual `v-model`.
- Submit button uses `:disabled="isSubmitting"`.

## Server validation errors

The backend returns 422s with field-keyed errors. Map them via `setErrors`:

```ts
const onSubmit = handleSubmit(async (values) => {
  try {
    await commodityService.create(values)
  } catch (error) {
    if (isApiValidationError(error)) {
      setErrors(error.fieldErrors) // { name: 'must be unique', count: 'too high' }
      return
    }
    throw error
  }
})
```

`<FormMessage>` automatically renders the message under the field. Do **not** pop a toast for a field error — the user already sees it in context.

A toast is appropriate only for transport failures (5xx, network) — and even then, prefer rethrowing to a global error boundary that handles toasts uniformly. See `useAppToast` in `design/composables/`.

## Field types reference

| Field | shadcn primitive | Notes |
|---|---|---|
| Text | `<Input>` | `type="text"` default |
| Email | `<Input type="email">` | + `z.string().email()` |
| Password | `<Input type="password">` | + autocomplete attr |
| Number | `<Input type="number">` | use `z.coerce.number()` to parse |
| Multiline | `<Textarea>` | rows defaults to 3 |
| Select | `<Select>` | for ≤ ~10 options |
| Searchable select | `<Combobox>` | for > 10 options or remote-loaded |
| Date | `<DatePicker>` | wraps Reka Calendar + Popover |
| Boolean | `<Checkbox>` | `binary` style (single boolean field) |
| Switch | `<Switch>` | for "feature on/off" semantic |
| Single-of-many | `<RadioGroup>` | always inside `<FormField>` |
| List of strings | `<InlineListEditor>` (pattern) | tags, serial numbers, urls |

## Required fields

Marked in the schema (`z.string().min(1, '…')`) — vee-validate sets `aria-required` automatically when the schema requires the field.

The visual asterisk is rendered by `<FormLabel required>`:

```vue
<FormLabel required>Name</FormLabel>
```

Do not hand-write `*` next to the label.

## Field-level help

Use `<FormDescription>` for hint text below the input:

```vue
<FormItem>
  <FormLabel>Default group</FormLabel>
  <FormControl><Select v-bind="componentField" :options="…" /></FormControl>
  <FormDescription>After login you'll land in this group.</FormDescription>
  <FormMessage />
</FormItem>
```

## Sticky footer

Long forms (`CommodityCreate`, `ExportCreate`) use the `<FormFooter sticky>` pattern (defined in `design/patterns/`):

```vue
<form @submit="onSubmit">
  <FormSection title="Basic Information">…</FormSection>
  <FormSection title="Price Information">…</FormSection>
  <!-- … -->
  <FormFooter sticky>
    <Button type="button" variant="outline" @click="cancel">Cancel</Button>
    <Button type="submit" :disabled="isSubmitting">Save</Button>
  </FormFooter>
</form>
```

The footer reads form `meta` (dirty / errors count) and surfaces `5 fields filled · 2 required missing` next to the buttons.

## Non-blocking validation

Validation runs on `blur` per field, on `change` after first blur, and on `submit` for the whole form. This is the vee-validate default; do not change it without team discussion.

## Testing forms

See [`testing.md`](./testing.md). In short:

- Vitest unit specs cover the schema (valid + invalid cases) separately from the view.
- Vitest mounts the view, fills fields by label (`getByLabelText`), submits, asserts the request payload.
- Playwright e2e covers the happy path + one error path per critical form.
