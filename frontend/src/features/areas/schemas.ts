import { z } from "zod"

const NAME_REQUIRED = "locations:validation.areaNameRequired"
const NAME_TOO_LONG = "locations:validation.areaNameTooLong"
const LOCATION_REQUIRED = "locations:validation.areaLocationRequired"
const ICON_TOO_LONG = "locations:validation.iconTooLong"

export const areaSchema = z.object({
  name: z.string().trim().min(1, NAME_REQUIRED).max(200, NAME_TOO_LONG),
  location_id: z.string().min(1, LOCATION_REQUIRED),
  // Short visual token (typically a single emoji) for the avatar tile
  // on the location detail's area grid. Empty string means "no icon
  // picked" — the UI falls back to the generic Package glyph. Capped
  // at 16 to leave room for ZWJ-joined emoji.
  icon: z.string().max(16, ICON_TOO_LONG),
})
export type AreaFormInput = z.infer<typeof areaSchema>
