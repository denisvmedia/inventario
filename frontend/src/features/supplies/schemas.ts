import { z } from "zod"

// supplyLinkFormSchema is the shared zod schema for the supply-link
// create + edit dialog. Mirrors the BE validation: label required
// (1..200), url required (absolute http(s), 1..2048), notes optional
// (0..1000). Keeping the regex narrow so an obvious typo
// ("amazon.com" with no scheme) is caught client-side before the
// user finds out via a 422.
export const supplyLinkFormSchema = z.object({
  label: z
    .string()
    .trim()
    .min(1, "Label is required")
    .max(200, "Keep the label under 200 characters"),
  url: z
    .string()
    .trim()
    .min(1, "URL is required")
    .max(2048, "Keep the URL under 2048 characters")
    .regex(/^https?:\/\/.+/i, "Use a full URL starting with http:// or https://"),
  notes: z
    .string()
    .max(1000, "Keep notes under 1000 characters")
    .optional()
    .default(""),
})

export type SupplyLinkFormInput = z.input<typeof supplyLinkFormSchema>
export type SupplyLinkFormValues = z.output<typeof supplyLinkFormSchema>
