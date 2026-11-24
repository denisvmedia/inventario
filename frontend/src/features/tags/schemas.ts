import { z } from "zod"

import { TAG_COLORS } from "./api"

// Tag slugs and labels are capped at this many characters. The BE counts
// runes (models.MaxTagSlugRunes / MaxTagLabelRunes), so the FE counts code
// points rather than UTF-16 units: an emoji in a label is one character on
// both sides.
export const TAG_TEXT_MAX_CHARS = 64

export function codePointLength(value: string): number {
  return Array.from(value).length
}

const LETTER_OR_NUMBER = /[\p{L}\p{N}]/u
const MARK = /\p{M}/u

// normaliseSlug mirrors models.NormalizeTagSlug on the BE, step for step:
// NFC, full Unicode lowercase, NFC again; keep letters and numbers of any
// script plus the combining marks that follow them; turn every other run
// into one `-`; drop leading and trailing `-`. The two are tested against
// the same table (go/models/tag_test.go tagSlugCases and
// __tests__/schemas.test.ts) and must change together. Kept client-side so
// the create form can show a live preview as the user types the label.
export function normaliseSlug(input: string): string {
  const lowered = input.normalize("NFC").toLowerCase().normalize("NFC")
  let out = ""
  let inRun = false
  let pendingDash = false
  for (const ch of lowered) {
    if (LETTER_OR_NUMBER.test(ch)) {
      if (pendingDash) {
        out += "-"
        pendingDash = false
      }
      out += ch
      inRun = true
    } else if (inRun && MARK.test(ch)) {
      out += ch
    } else {
      // A mark with nothing to attach to counts as a separator too.
      inRun = false
      pendingDash = out.length > 0
    }
  }
  return out
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
