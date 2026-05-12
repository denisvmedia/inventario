import { z } from "zod"

// Validation schemas for the location create/edit form. i18n keys live
// in the `locations` namespace under `validation`; pages translate
// `errors[name].message` at render time the same way the group forms
// do (#1413).

const NAME_REQUIRED = "locations:validation.nameRequired"
const NAME_TOO_LONG = "locations:validation.nameTooLong"
const ADDRESS_TOO_LONG = "locations:validation.addressTooLong"
const ICON_TOO_LONG = "locations:validation.iconTooLong"
const DESCRIPTION_TOO_LONG = "locations:validation.descriptionTooLong"

export const locationSchema = z.object({
  name: z.string().trim().min(1, NAME_REQUIRED).max(200, NAME_TOO_LONG),
  // Free-text "where is this" — the physical street/address slot. The
  // BE accepts an empty string as "no address".
  address: z.string().trim().max(2000, ADDRESS_TOO_LONG),
  // Short visual token (typically a single emoji) for the avatar tile
  // on the locations list. Empty string means "no icon picked" and the
  // UI falls back to the generic MapPin glyph. Capped at 16 to leave
  // room for ZWJ-joined emoji while still rejecting accidental long
  // strings.
  icon: z.string().max(16, ICON_TOO_LONG),
  // Optional one-line description rendered as the muted subtitle under
  // the location's name on the list / detail views — distinct from
  // `address`, which carries the structured street info.
  description: z.string().trim().max(200, DESCRIPTION_TOO_LONG),
})
export type LocationFormInput = z.infer<typeof locationSchema>
