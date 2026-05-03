import { z } from "zod"

import { TAG_COLORS } from "./api"

// normaliseSlug mirrors models.NormalizeTagSlug on the BE: lowercase,
// replace runs of non-alphanumerics with `-`, trim leading/trailing `-`.
// Kept client-side so the create form can show a live preview as the
// user types the label.
export function normaliseSlug(input: string): string {
  return input
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
}

const slugPattern = /^[a-z0-9]+(-[a-z0-9]+)*$/

export const tagFormSchema = z.object({
  label: z
    .string()
    .trim()
    .min(1, "tags:validation.labelRequired")
    .max(64, "tags:validation.labelTooLong"),
  slug: z
    .string()
    .trim()
    .min(1, "tags:validation.slugRequired")
    .max(64, "tags:validation.slugTooLong")
    .regex(slugPattern, "tags:validation.slugInvalid"),
  color: z.enum(TAG_COLORS as [string, ...string[]], {
    message: "tags:validation.colorRequired",
  }),
})

export type TagFormInput = z.input<typeof tagFormSchema>
export type TagFormValues = z.output<typeof tagFormSchema>
