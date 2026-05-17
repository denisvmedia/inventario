import { z } from "zod"

// maintenanceFormSchema backs the MaintenanceScheduleDialog (RHF + zod)
// on the per-commodity Maintenance tab (#1368). Mirrors loans/services
// schema conventions:
//
// - i18n keys as error messages so `t(form.formState.errors.X.message)`
//   renders localised copy.
// - `interval_days` is coerced from the <input type="number"> string to
//   a positive integer; the input lives in the DOM as a string until
//   submit.
// - `next_due_at` is optional — the BE defaults it to
//   `today + interval_days` when blank.
// - Empty strings are kept as "" rather than being stripped at schema
//   level so the form re-mounts with the same shape after submission;
//   the dialog's `onSubmit` callback maps empties to undefined for the
//   API payload.
export const maintenanceFormSchema = z.object({
  title: z
    .string()
    .trim()
    .min(1, "maintenance:validation.titleRequired")
    .max(200, "maintenance:validation.titleTooLong"),
  interval_days: z.coerce
    .number()
    .int("maintenance:validation.intervalInvalid")
    .min(1, "maintenance:validation.intervalMin")
    .max(36500, "maintenance:validation.intervalMax"),
  next_due_at: z
    .string()
    .optional()
    .default("")
    .refine(
      (v) => v === "" || /^\d{4}-\d{2}-\d{2}$/.test(v),
      "maintenance:validation.nextDueAtInvalid"
    ),
  notes: z.string().max(1000, "maintenance:validation.notesTooLong").optional().default(""),
})

export type MaintenanceFormInput = z.input<typeof maintenanceFormSchema>
export type MaintenanceFormOutput = z.output<typeof maintenanceFormSchema>
