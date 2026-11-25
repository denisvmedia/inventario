import { z } from "zod"

import { SELLER_CONTACT_MAX, SELLER_NAME_MAX, SELLER_NOTE_MAX, SELLER_WEBSITE_MAX } from "./api"

// The server counts runes, so the form counts code points.
function codePoints(value: string): number {
  return Array.from(value).length
}

// isHttpUrl mirrors validateAbsoluteHTTPURL in go/models/seller.go: an
// absolute http or https URL with a host.
export function isHttpUrl(value: string): boolean {
  try {
    const url = new URL(value)
    return (url.protocol === "http:" || url.protocol === "https:") && url.host !== ""
  } catch {
    return false
  }
}

// sellerFormSchema backs SellerFormDialog. Messages are i18n keys; the limits
// mirror go/models/seller.go.
export const sellerFormSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, "sellers:validation.nameRequired")
    .refine((v) => codePoints(v) <= SELLER_NAME_MAX, "sellers:validation.nameTooLong"),
  website: z
    .string()
    .trim()
    .refine((v) => codePoints(v) <= SELLER_WEBSITE_MAX, "sellers:validation.websiteTooLong")
    .refine((v) => v === "" || isHttpUrl(v), "sellers:validation.websiteInvalid"),
  contact: z
    .string()
    .trim()
    .refine((v) => codePoints(v) <= SELLER_CONTACT_MAX, "sellers:validation.contactTooLong"),
  note: z
    .string()
    .trim()
    .refine((v) => codePoints(v) <= SELLER_NOTE_MAX, "sellers:validation.noteTooLong"),
})

export type SellerFormInput = z.input<typeof sellerFormSchema>
export type SellerFormValues = z.output<typeof sellerFormSchema>

export const EMPTY_SELLER_FORM: SellerFormInput = { name: "", website: "", contact: "", note: "" }
