import { z } from 'zod'

/**
 * Shared zod schema for the Location create / edit forms and for the
 * inline "New Location" form rendered in `LocationListView`.
 *
 * Per devdocs/frontend/forms.md ("When two forms share fields … extract
 * the shared bits into a base schema in the same directory"). The
 * fields are identical between Create and Edit; the only Edit-specific
 * concern (the resource id) is sourced from the route, not the form,
 * so no extension is needed.
 */
export const locationFormSchema = z.object({
  name: z
    .string({ required_error: 'Name is required' })
    .min(1, 'Name is required')
    .max(100, 'Name must be at most 100 characters'),
  address: z
    .string({ required_error: 'Address is required' })
    .min(1, 'Address is required')
    .max(500, 'Address must be at most 500 characters'),
})

export type LocationFormInput = z.infer<typeof locationFormSchema>
