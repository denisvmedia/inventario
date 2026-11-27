import { z } from "zod"

// supplyLinkFormSchema is the shared zod schema for the supply-link
// create + edit dialog. Mirrors the BE validation: label required
// (1..200), url required (absolute http(s), 1..2048), notes optional
// (0..1000). Keeping the regex narrow so an obvious typo
// ("amazon.com" with no scheme) is caught client-side before the
// user finds out via a 422. The messages are i18n keys, which FieldError
// resolves.
export const supplyLinkFormSchema = z.object({
  label: z
    .string()
    .trim()
    .min(1, "supplies:validation.labelRequired")
    .max(200, "supplies:validation.labelTooLong"),
  url: z
    .string()
    .trim()
    .min(1, "supplies:validation.urlRequired")
    .max(2048, "supplies:validation.urlTooLong")
    .regex(/^https?:\/\/.+/i, "supplies:validation.urlScheme"),
  notes: z.string().max(1000, "supplies:validation.notesTooLong").optional().default(""),
})

export type SupplyLinkFormInput = z.input<typeof supplyLinkFormSchema>
export type SupplyLinkFormValues = z.output<typeof supplyLinkFormSchema>
