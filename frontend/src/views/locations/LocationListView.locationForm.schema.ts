import { z } from 'zod'

/**
 * Schema for the inline "Create location" form rendered inside
 * `LocationListView`. Co-located per the convention in
 * devdocs/frontend/forms.md (`<View>.<form>.schema.ts`).
 *
 * Mirrors the legacy `LocationForm` validation: name and address are
 * both required and trimmed. When the standalone `LocationCreateView`
 * / `LocationEditView` migration lands later in Phase 4, this schema
 * will be promoted to a shared `LocationForm.schema.ts`.
 */
export const locationListLocationFormSchema = z.object({
  name: z
    .string({ required_error: 'Name is required' })
    .min(1, 'Name is required')
    .max(100, 'Name must be at most 100 characters'),
  address: z
    .string({ required_error: 'Address is required' })
    .min(1, 'Address is required')
    .max(500, 'Address must be at most 500 characters'),
})

export type LocationListLocationFormInput = z.infer<
  typeof locationListLocationFormSchema
>
