import { z } from "zod"

import { TAG_COLORS } from "./api"
import { normaliseSlug } from "./slug"

export { normaliseSlug }

// Tag slugs and labels are capped at this many characters. The BE counts
// runes (models.MaxTagSlugRunes / MaxTagLabelRunes), so the FE counts code
// points rather than UTF-16 units: an emoji in a label is one character on
// both sides.
export const TAG_TEXT_MAX_CHARS = 64

export function codePointLength(value: string): number {
  return Array.from(value).length
}

// Same expression as the BE's tagSlugPattern. On its own it accepts
// uppercase letters and decomposed text, so the schema also requires the
// slug to be its own normal form — the BE's IsValidTagSlug does the same.
export const slugPattern = /^[\p{L}\p{N}][\p{L}\p{N}\p{M}]*(?:-[\p{L}\p{N}][\p{L}\p{N}\p{M}]*)*$/u

export function isValidSlug(value: string): boolean {
  return slugPattern.test(value) && normaliseSlug(value) === value
}

export const tagFormSchema = z.object({
  label: z
    .string()
    .trim()
    .min(1, "tags:validation.labelRequired")
    .refine((v) => codePointLength(v) <= TAG_TEXT_MAX_CHARS, "tags:validation.labelTooLong"),
  slug: z
    .string()
    .trim()
    .min(1, "tags:validation.slugRequired")
    .refine((v) => codePointLength(v) <= TAG_TEXT_MAX_CHARS, "tags:validation.slugTooLong")
    .refine(isValidSlug, "tags:validation.slugInvalid"),
  color: z.enum(TAG_COLORS, {
    message: "tags:validation.colorRequired",
  }),
})

export type TagFormInput = z.input<typeof tagFormSchema>
export type TagFormValues = z.output<typeof tagFormSchema>
