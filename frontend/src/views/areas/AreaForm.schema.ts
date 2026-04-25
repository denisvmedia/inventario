import { z } from 'zod'

/**
 * Shared zod schemas for the Area forms.
 *
 * `areaFormSchema` (name only) backs the inline "Add Area" forms in
 * `LocationListView` and `LocationDetailView`, where the parent
 * `location_id` is supplied by the surrounding context (the expanded
 * location row) rather than chosen by the user.
 *
 * `areaEditFormSchema` extends it with `location_id` for the
 * standalone `AreaEditView`, where the area can be re-parented to a
 * different location via a Select control.
 *
 * Per devdocs/frontend/forms.md ("When two forms share fields …
 * extract the shared bits into a base schema in the same directory").
 */
export const areaFormSchema = z.object({
  name: z
    .string({ required_error: 'Name is required' })
    .min(1, 'Name is required')
    .max(100, 'Name must be at most 100 characters'),
})

export type AreaFormInput = z.infer<typeof areaFormSchema>

export const areaEditFormSchema = areaFormSchema.extend({
  location_id: z
    .string({ required_error: 'Location is required' })
    .min(1, 'Location is required'),
})

export type AreaEditFormInput = z.infer<typeof areaEditFormSchema>
