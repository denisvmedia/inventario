import { z } from "zod"

// Validation schemas for the location create/edit form. i18n keys live
// in the `locations` namespace under `validation`; pages translate
// `errors[name].message` at render time the same way the group forms
// do (#1413).

const NAME_REQUIRED = "locations:validation.nameRequired"
const NAME_TOO_LONG = "locations:validation.nameTooLong"
const ADDRESS_TOO_LONG = "locations:validation.addressTooLong"

export const locationSchema = z.object({
  name: z.string().trim().min(1, NAME_REQUIRED).max(200, NAME_TOO_LONG),
  // Maps to the BE's `address`; the design mock calls it "description".
  // Empty string flows through — the BE accepts that as "no address".
  // We avoid `.default("")` here because zod 3 makes the input type
  // include `undefined` (default fills it in) which then disagrees
  // with react-hook-form's resolver expecting input == output.
  address: z.string().trim().max(2000, ADDRESS_TOO_LONG),
})
export type LocationFormInput = z.infer<typeof locationSchema>
