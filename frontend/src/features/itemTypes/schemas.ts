import { z } from "zod"

import { ITEM_TYPE_NAME_MAX } from "./api"

// The server counts runes, so the form counts code points.
function codePoints(value: string): number {
  return Array.from(value).length
}

// itemTypeFormSchema backs ItemTypeFormDialog. Messages are i18n keys; the
// limit mirrors go/models/item_type.go. The name may be empty only for a
// built-in type, which then goes back to its translated default name; the
// dialog checks that, since the schema does not know which type it edits.
// The icon is a key from the picker, or the key a type already had, which
// an edit keeps when the picker does not know it.
export const itemTypeFormSchema = z.object({
  name: z
    .string()
    .trim()
    .refine((v) => codePoints(v) <= ITEM_TYPE_NAME_MAX, "itemTypes:validation.nameTooLong"),
  icon: z.string(),
})

export type ItemTypeFormInput = z.input<typeof itemTypeFormSchema>
export type ItemTypeFormValues = z.output<typeof itemTypeFormSchema>
