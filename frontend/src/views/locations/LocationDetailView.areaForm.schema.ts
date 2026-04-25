import { z } from 'zod'

/**
 * Schema for the inline "Add area" form rendered inside
 * `LocationDetailView`. Co-located per the convention in
 * devdocs/frontend/forms.md (`<View>.<form>.schema.ts`).
 *
 * The schema currently only validates the area name. When a fuller
 * area form is migrated in a later Phase 4 commit (`AreaCreateView` /
 * `AreaEditView`), this schema will be promoted to a shared
 * `frontend/src/views/areas/AreaForm.schema.ts` and extended.
 */
export const locationDetailAreaFormSchema = z.object({
  name: z
    .string({ required_error: 'Name is required' })
    .min(1, 'Name is required')
    .max(100, 'Name must be at most 100 characters'),
})

export type LocationDetailAreaFormInput = z.infer<
  typeof locationDetailAreaFormSchema
>
