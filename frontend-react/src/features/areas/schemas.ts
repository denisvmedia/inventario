import { z } from "zod"

const NAME_REQUIRED = "locations:validation.areaNameRequired"
const NAME_TOO_LONG = "locations:validation.areaNameTooLong"
const LOCATION_REQUIRED = "locations:validation.areaLocationRequired"

export const areaSchema = z.object({
  name: z.string().trim().min(1, NAME_REQUIRED).max(200, NAME_TOO_LONG),
  location_id: z.string().min(1, LOCATION_REQUIRED),
})
export type AreaFormInput = z.infer<typeof areaSchema>
