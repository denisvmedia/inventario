import { z } from "zod"

import { isAllowedGroupIcon } from "./icons"

// Schemas for the group create/update forms. i18n keys live in the
// `groups` namespace under the `validation` sub-tree; pages translate
// errors[name].message at render time.

const ICON_MESSAGE = "groups:validation.iconUnknown"
const NAME_REQUIRED = "groups:validation.nameRequired"
const NAME_TOO_LONG = "groups:validation.nameTooLong"
const CURRENCY_INVALID = "groups:validation.currencyInvalid"

// Basic ISO-4217 sanity check: three uppercase letters. We don't ship
// the full SWIFT/ISO list — the server is authoritative — but a
// client-side guard keeps the obvious typos from a 422 round-trip.
const CURRENCY_RE = /^[A-Z]{3}$/

export const createGroupSchema = z.object({
  name: z.string().trim().min(1, NAME_REQUIRED).max(100, NAME_TOO_LONG),
  // Empty string = "no icon" per the BE contract. Anything else has to
  // be one of the curated emoji.
  icon: z.string().refine(isAllowedGroupIcon, { message: ICON_MESSAGE }),
  // main_currency is set once on create. The BE defaults to USD when
  // missing; surface a default here too so the form is never empty.
  main_currency: z
    .string()
    .trim()
    .toUpperCase()
    .refine((v) => CURRENCY_RE.test(v), { message: CURRENCY_INVALID }),
})
export type CreateGroupInput = z.infer<typeof createGroupSchema>

export const updateGroupSchema = z.object({
  name: z.string().trim().min(1, NAME_REQUIRED).max(100, NAME_TOO_LONG),
  icon: z.string().refine(isAllowedGroupIcon, { message: ICON_MESSAGE }),
})
export type UpdateGroupInput = z.infer<typeof updateGroupSchema>

// /groups/:id/settings — Danger zone delete dialog. confirm_word must
// match the group's current name; we validate that mismatch in the
// page handler (the schema doesn't know the group name) and let zod
// only enforce non-empty here.
export const deleteGroupSchema = z.object({
  confirmWord: z.string().min(1, "groups:validation.confirmWordRequired"),
  password: z.string().min(1, "auth:validation.passwordRequired"),
})
export type DeleteGroupInput = z.infer<typeof deleteGroupSchema>
