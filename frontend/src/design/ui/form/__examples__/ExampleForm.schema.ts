import { z } from 'zod'

/**
 * Minimal example schema showing the canonical layout prescribed by
 * devdocs/frontend/forms.md. Real view schemas live next to their view
 * as `<View>.schema.ts`; this file stands in for such a co-located schema
 * so the stack (zod + @vee-validate/zod + shadcn-vue `<Form>`) can be
 * exercised by unit tests without coupling the infrastructure PR to any
 * specific legacy view migration.
 *
 * When a real form is migrated, copy the shape of this file into
 * `src/views/<area>/<View>.schema.ts` and delete nothing here — the
 * example is load-bearing for the form primitive specs.
 */
export const exampleFormSchema = z.object({
  name: z
    .string({ required_error: 'Name is required' })
    .min(1, 'Name is required')
    .max(100),
  email: z
    .string({ required_error: 'Email is required' })
    .email('Must be a valid email'),
  count: z.coerce
    .number({ required_error: 'Count is required', invalid_type_error: 'Count is required' })
    .int()
    .min(1)
    .max(10_000),
})

export type ExampleFormInput = z.infer<typeof exampleFormSchema>
