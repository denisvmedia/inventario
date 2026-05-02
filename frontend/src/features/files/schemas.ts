import { z } from "zod"

import { FILE_CATEGORIES } from "./constants"

// Metadata edit form. Backed by `PUT /files/{id}` (apiserver/files.go).
// The BE auto-derives `Type` and re-derives `Category` when MIME is
// known; we still send `category` so an explicit user pick (e.g.
// reclassifying a PDF as Photos) survives.
// Error messages are i18n keys, not localised strings — the page
// translates them via `t(error.message)` so cs/ru can override the same
// validation flow without a schema fork. Keys live in
// `files:validation.*`.
export const fileMetadataSchema = z.object({
  title: z.string().trim().max(255).optional().or(z.literal("")),
  description: z.string().trim().max(1000).optional().or(z.literal("")),
  path: z.string().trim().min(1, "files:validation.pathRequired").max(255),
  category: z.enum(FILE_CATEGORIES),
  tags: z.array(z.string().trim().min(1)).default([]),
})

export type FileMetadataFormInput = z.input<typeof fileMetadataSchema>
export type FileMetadataFormValues = z.output<typeof fileMetadataSchema>
