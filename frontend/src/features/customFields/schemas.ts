import { z } from "zod"

import { nameKey } from "@/lib/name-key"

import {
  CUSTOM_FIELD_NAME_MAX,
  CUSTOM_FIELD_OPTION_LABEL_MAX,
  CUSTOM_FIELD_OPTIONS_MAX,
  CUSTOM_FIELD_TYPES,
} from "./api"

// The server counts runes, so the form counts code points.
function codePoints(value: string): number {
  return Array.from(value).length
}

// customFieldFormSchema backs CustomFieldFormDialog. Messages are i18n
// keys; the limits mirror go/models/custom_field.go. Options matter only
// for a select field, which needs at least one; their labels are compared
// the way the server compares names.
export const customFieldFormSchema = z
  .object({
    name: z
      .string()
      .trim()
      .min(1, "customFields:validation.nameRequired")
      .refine((v) => codePoints(v) <= CUSTOM_FIELD_NAME_MAX, "customFields:validation.nameTooLong"),
    type: z.enum(CUSTOM_FIELD_TYPES),
    options: z.array(
      z.object({
        id: z.string().optional(),
        label: z
          .string()
          .trim()
          .min(1, "customFields:validation.optionRequired")
          .refine(
            (v) => codePoints(v) <= CUSTOM_FIELD_OPTION_LABEL_MAX,
            "customFields:validation.optionTooLong"
          ),
      })
    ),
  })
  .superRefine((value, ctx) => {
    if (value.type !== "select") return
    if (value.options.length === 0) {
      ctx.addIssue({
        code: "custom",
        path: ["options"],
        message: "customFields:validation.optionsRequired",
      })
      return
    }
    if (value.options.length > CUSTOM_FIELD_OPTIONS_MAX) {
      ctx.addIssue({
        code: "custom",
        path: ["options"],
        message: "customFields:validation.optionsTooMany",
      })
    }
    const seen = new Set<string>()
    value.options.forEach((option, index) => {
      const key = nameKey(option.label)
      if (!key) return
      if (seen.has(key)) {
        ctx.addIssue({
          code: "custom",
          path: ["options", index, "label"],
          message: "customFields:validation.optionDuplicate",
        })
      }
      seen.add(key)
    })
  })

export type CustomFieldFormInput = z.input<typeof customFieldFormSchema>
export type CustomFieldFormValues = z.output<typeof customFieldFormSchema>
