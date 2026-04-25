import { z } from 'zod'

/**
 * Schema for the inline "Add area" form rendered inside an expanded
 * location row in `LocationListView`. Co-located per the convention
 * in devdocs/frontend/forms.md (`<View>.<form>.schema.ts`).
 *
 * Identical to `LocationDetailView.areaForm.schema.ts`; both will be
 * collapsed into a shared `AreaForm.schema.ts` when the standalone
 * `AreaCreateView` / `AreaEditView` migration lands later in
 * Phase 4 — kept duplicated for now to honour the strict
 * "co-located per view" naming rule.
 */
export const locationListAreaFormSchema = z.object({
  name: z
    .string({ required_error: 'Name is required' })
    .min(1, 'Name is required')
    .max(100, 'Name must be at most 100 characters'),
})

export type LocationListAreaFormInput = z.infer<
  typeof locationListAreaFormSchema
>
