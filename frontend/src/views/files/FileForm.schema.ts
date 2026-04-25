import { z } from 'zod'

/**
 * Shared zod schema for the File edit form.
 *
 * Backs `FileEditView` after the Phase 4 design-system migration
 * (Epic #1324 / issue #1329). Only `path` is required; `title` falls
 * back to the filename when blank, and the `linked_entity_*` triple
 * is optional when the file is standalone.
 *
 * Per devdocs/frontend/forms.md the schema lives next to the view
 * that owns it so future File Create / Edit splits can extend it
 * without churn.
 */
export const fileEditFormSchema = z.object({
  path: z
    .string({ required_error: 'Filename is required' })
    .min(1, 'Filename is required')
    .max(255, 'Filename must be at most 255 characters'),
  title: z.string().max(255, 'Title must be at most 255 characters').optional().default(''),
  description: z
    .string()
    .max(2000, 'Description must be at most 2000 characters')
    .optional()
    .default(''),
  tags: z.array(z.string().min(1)).default([]),
  linked_entity_type: z.string().optional().default(''),
  linked_entity_id: z.string().optional().default(''),
  linked_entity_meta: z.string().optional().default(''),
})

export type FileEditFormInput = z.infer<typeof fileEditFormSchema>

export const defaultFileEditFormValues = (
  overrides: Partial<FileEditFormInput> = {},
): FileEditFormInput => ({
  path: '',
  title: '',
  description: '',
  tags: [],
  linked_entity_type: '',
  linked_entity_id: '',
  linked_entity_meta: '',
  ...overrides,
})
